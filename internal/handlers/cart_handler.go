package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"eneba-backend/internal/database"
	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/requests"
	"eneba-backend/internal/responses"
)

// Cart routes are public: the cart is found by the cart_token cookie, not the signed-in user.
// Every change except emptying the cart answers with the whole updated cart.
type CartHandler struct {
	pool          *pgxpool.Pool
	carts         *repository.CartRepository
	secureCookies bool // true in production (HTTPS)
}

func NewCartHandler(pool *pgxpool.Pool, carts *repository.CartRepository, secureCookies bool) *CartHandler {
	return &CartHandler{pool: pool, carts: carts, secureCookies: secureCookies}
}

// Get never creates a cart or sets a cookie; without a cart it returns an empty one.
func (h *CartHandler) Get(w http.ResponseWriter, r *http.Request) {
	cartID, ok := cartIDFromCookie(r)
	if !ok {
		httpx.WriteJSON(w, http.StatusOK, responses.NewCartResponse(nil, ""))
		return
	}

	h.writeCart(w, r, cartID)
}

// AddItem adds one key per call, like one click on "Add to cart"; PATCH sets other amounts.
// The first add creates the cart and sets the cart_token cookie.
func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req requests.AddCartItemRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	offerID, err := req.Validate()
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	cartID, hasCookie := cartIDFromCookie(r)

	err = database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		carts := h.carts.WithTx(tx)

		// Locking the offer serializes concurrent adds, so the checks below see the latest quantity.
		if err := carts.LockActiveOffer(ctx, offerID); err != nil {
			return err
		}

		// A new cart is rolled back with everything else if the add fails.
		exists := false
		if hasCookie {
			var err error
			if exists, err = carts.LockCart(ctx, cartID); err != nil {
				return err
			}
		}
		if !exists {
			var err error
			if cartID, err = carts.CreateCart(ctx); err != nil {
				return err
			}
		}

		current, err := carts.GetQuantityForUpdate(ctx, cartID, offerID)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			return err
		}

		quantity := current + 1
		if quantity > models.MaxCartItemQuantity {
			return httpx.ValidationFailed(map[string]string{
				"quantity": fmt.Sprintf(
					"a cart can hold at most %d keys of one offer, it already has %d",
					models.MaxCartItemQuantity, current,
				),
			})
		}

		if err := ensureInStock(ctx, carts, offerID, quantity); err != nil {
			return err
		}

		if err := carts.AddItem(ctx, cartID, offerID); err != nil {
			return err
		}

		return carts.Touch(ctx, cartID)
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	// Only now, after the commit, does the browser learn the (maybe new) cart ID.
	setCartCookie(w, cartID, h.secureCookies)
	h.writeCart(w, r, cartID)
}

func (h *CartHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	offerID, err := requests.ParseOfferIDParam(chi.URLParam(r, "offerId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var req requests.UpdateCartItemRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	quantity, err := req.Validate()
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	cartID, ok := cartIDFromCookie(r)
	if !ok {
		httpx.Error(w, r, models.ErrNotFound)
		return
	}

	err = database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		carts := h.carts.WithTx(tx)

		if err := lockExistingCart(ctx, carts, cartID); err != nil {
			return err
		}

		if _, err := carts.GetQuantityForUpdate(ctx, cartID, offerID); err != nil {
			return err
		}

		if err := ensureInStock(ctx, carts, offerID, quantity); err != nil {
			return err
		}

		if err := carts.SetQuantity(ctx, cartID, offerID, quantity); err != nil {
			return err
		}

		return carts.Touch(ctx, cartID)
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	setCartCookie(w, cartID, h.secureCookies)
	h.writeCart(w, r, cartID)
}

func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	offerID, err := requests.ParseOfferIDParam(chi.URLParam(r, "offerId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	cartID, ok := cartIDFromCookie(r)
	if !ok {
		httpx.Error(w, r, models.ErrNotFound)
		return
	}

	err = database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		carts := h.carts.WithTx(tx)

		if err := lockExistingCart(ctx, carts, cartID); err != nil {
			return err
		}

		if err := carts.RemoveItem(ctx, cartID, offerID); err != nil {
			return err
		}

		return carts.Touch(ctx, cartID)
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	setCartCookie(w, cartID, h.secureCookies)
	h.writeCart(w, r, cartID)
}

// Clear keeps the cart row and the cookie, so the next add reuses the same cart.
func (h *CartHandler) Clear(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cartID, ok := cartIDFromCookie(r)
	if !ok {
		httpx.NoContent(w)
		return
	}

	exists := false
	err := database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		carts := h.carts.WithTx(tx)

		var err error
		if exists, err = carts.LockCart(ctx, cartID); err != nil || !exists {
			return err
		}

		if err := carts.Clear(ctx, cartID); err != nil {
			return err
		}

		return carts.Touch(ctx, cartID)
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if exists {
		setCartCookie(w, cartID, h.secureCookies)
	}
	httpx.NoContent(w)
}

func (h *CartHandler) writeCart(w http.ResponseWriter, r *http.Request, cartID uuid.UUID) {
	cart, err := h.carts.GetCart(r.Context(), cartID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, cart)
}

func lockExistingCart(ctx context.Context, carts *repository.CartRepository, cartID uuid.UUID) error {
	exists, err := carts.LockCart(ctx, cartID)
	if err != nil {
		return err
	}
	if !exists {
		return models.ErrNotFound
	}

	return nil
}

// Checkout checks the stock again, since keys can sell out while they sit in the cart.
func ensureInStock(ctx context.Context, carts *repository.CartRepository, offerID uuid.UUID, quantity int) error {
	available, err := carts.CountAvailableKeys(ctx, offerID)
	if err != nil {
		return err
	}
	if quantity > available {
		return httpx.InsufficientStock(available)
	}

	return nil
}
