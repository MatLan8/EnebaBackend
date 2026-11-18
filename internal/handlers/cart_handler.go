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

// CartHandler serves the browser's cart. The routes are public: the cart is
// found by the cart_token cookie, not by the signed-in user, so a token in the
// Authorization header is ignored. Every change except emptying the cart
// answers with the whole updated cart.
type CartHandler struct {
	pool          *pgxpool.Pool // needed to start transactions
	carts         *repository.CartRepository
	secureCookies bool // true in production (HTTPS)
}

func NewCartHandler(pool *pgxpool.Pool, carts *repository.CartRepository, secureCookies bool) *CartHandler {
	return &CartHandler{pool: pool, carts: carts, secureCookies: secureCookies}
}

// Get handles GET /api/v1/cart. A browser without a cart (no cookie, a broken
// cookie or a cart that no longer exists) gets an empty cart. Nothing is
// created and no cookie is set; the first add does that.
func (h *CartHandler) Get(w http.ResponseWriter, r *http.Request) {
	cartID, ok := cartIDFromCookie(r)
	if !ok {
		httpx.WriteJSON(w, http.StatusOK, responses.NewCartResponse(nil, ""))
		return
	}

	// An unknown cart ID simply has no items, so it also returns an empty cart.
	h.writeCart(w, r, cartID)
}

// AddItem handles POST /api/v1/cart/items with {"offerId": "..."}. Each call adds
// one key, so clicking "Add to cart" twice gives 2. To set any other amount use
// UpdateItem. If the browser has no cart yet, one is created and its ID is set
// in the cart_token cookie. An unknown or inactive offer is a 404, an 11th key
// is a 400, and more keys than are unsold is a 409 insufficient_stock.
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

		// Locking the offer makes concurrent adds of it wait for each other,
		// so the checks below always see the latest quantity.
		if err := carts.LockActiveOffer(ctx, offerID); err != nil {
			return err // models.ErrNotFound becomes 404
		}

		// Use the cookie's cart if it still exists, otherwise start a new one.
		// The new cart is rolled back with everything else if the add fails.
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
		// Not in the cart yet means current is 0.

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

// UpdateItem handles PATCH /api/v1/cart/items/{offerId} and sets the item's
// quantity to the value in the body. No cart, or an offer that isn't in the
// cart, is a 404, and more than the unsold keys is a 409 insufficient_stock.
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
			return err // models.ErrNotFound becomes 404
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

// RemoveItem handles DELETE /api/v1/cart/items/{offerId}.
// No cart, or an offer that isn't in the cart, is a 404.
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
			return err // models.ErrNotFound becomes 404
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

// Clear handles DELETE /api/v1/cart and removes every item with a 204. The cart
// row and the cookie are kept, so the next add reuses the same cart.
// Without a cart there is nothing to empty, which is also a 204.
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

// writeCart responds with the cart's current contents.
func (h *CartHandler) writeCart(w http.ResponseWriter, r *http.Request, cartID uuid.UUID) {
	cart, err := h.carts.GetCart(r.Context(), cartID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, cart)
}

// lockExistingCart locks the cart's row, or returns models.ErrNotFound if the
// cookie points to a cart that doesn't exist.
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

// ensureInStock returns a 409 insufficient_stock error if the offer has fewer
// unsold keys than quantity. Checkout checks the stock again, since keys can
// sell out while they sit in the cart.
func ensureInStock(ctx context.Context, carts *repository.CartRepository, offerID uuid.UUID, quantity int) error {
	available, err := carts.CountUnsoldKeys(ctx, offerID)
	if err != nil {
		return err
	}
	if quantity > available {
		return httpx.InsufficientStock(available)
	}

	return nil
}
