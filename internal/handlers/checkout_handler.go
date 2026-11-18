package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"eneba-backend/internal/database"
	"eneba-backend/internal/httpx"
	"eneba-backend/internal/middleware"
	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/responses"
	"eneba-backend/internal/services"
)

const (
	// checkoutSessionTTL is how long the Stripe payment page stays open. Stripe's
	// minimum is 30 minutes; the extra minute covers the time the request takes.
	// The keys are reserved a bit longer (see repository.keyReservationMinutes).
	checkoutSessionTTL = 31 * time.Minute
	// publicIDAttempts is how many random order numbers are tried before giving up.
	publicIDAttempts = 5
)

// CheckoutHandler turns the browser's cart into an order and a Stripe payment page.
type CheckoutHandler struct {
	pool        *pgxpool.Pool // needed to start transactions
	carts       *repository.CartRepository
	orders      *repository.OrderRepository
	users       *repository.UserRepository
	stripe      *services.StripeService
	frontendURL string // where Stripe sends the buyer back to
}

func NewCheckoutHandler(
	pool *pgxpool.Pool,
	carts *repository.CartRepository,
	orders *repository.OrderRepository,
	users *repository.UserRepository,
	stripe *services.StripeService,
	frontendURL string,
) *CheckoutHandler {
	return &CheckoutHandler{
		pool:        pool,
		carts:       carts,
		orders:      orders,
		users:       users,
		stripe:      stripe,
		frontendURL: frontendURL,
	}
}

// Create handles POST /api/v1/checkout. It needs a signed-in user and the
// cart_token cookie, and has no body.
//
//  1. In one transaction: reserve a key for every item in the cart and create a
//     pending order with one order item per key, priced at the current offer price.
//  2. Create a Stripe Checkout session for the order and answer with its URL.
//
// The cart is not emptied here. It is emptied by the Stripe webhook once the
// payment succeeds, so a buyer who cancels on Stripe still has their cart.
// Keys are released when Stripe reports the session as expired.
func (h *CheckoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		httpx.Error(w, r, models.ErrUnauthorized)
		return
	}

	cartID, ok := cartIDFromCookie(r)
	if !ok {
		httpx.Error(w, r, httpx.CartEmpty())
		return
	}

	user, err := h.users.GetByID(ctx, userID)
	if errors.Is(err, models.ErrNotFound) {
		// The token is valid but the user was deleted since it was issued.
		httpx.Error(w, r, models.ErrInvalidToken)
		return
	}
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var orderID uuid.UUID
	var lines []repository.CheckoutLine

	err = database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		carts := h.carts.WithTx(tx)
		orders := h.orders.WithTx(tx)

		// Locking the cart makes a second checkout of the same cart (e.g. a
		// double click) wait until this one is done.
		if err := lockExistingCart(ctx, carts, cartID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return httpx.CartEmpty()
			}
			return err
		}

		var err error
		lines, err = orders.ListCheckoutLines(ctx, cartID)
		if err != nil {
			return err
		}

		totalCents, currency, err := summarizeCheckout(lines)
		if err != nil {
			return err
		}

		items, err := reserveKeys(ctx, orders, lines)
		if err != nil {
			return err
		}

		orderID, err = createPendingOrder(ctx, orders, repository.NewOrder{
			UserID:     userID,
			CartID:     cartID,
			TotalCents: totalCents,
			Currency:   currency,
		})
		if err != nil {
			return err
		}

		return orders.CreateOrderItems(ctx, orderID, items)
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	// The Stripe call happens after the commit: a slow network call should not
	// keep the cart and the keys locked.
	session, err := h.stripe.CreateCheckoutSession(ctx, services.CheckoutSessionParams{
		OrderID:       orderID,
		CustomerEmail: user.Email,
		Currency:      lines[0].Currency,
		LineItems:     checkoutLineItems(lines),
		SuccessURL:    fmt.Sprintf("%s/orders/%s?paid=1", h.frontendURL, orderID),
		CancelURL:     h.frontendURL + "/cart",
		ExpiresAt:     time.Now().Add(checkoutSessionTTL),
	})
	if err != nil {
		slog.ErrorContext(ctx, "stripe checkout session failed", "order_id", orderID, "error", err)

		// Undo the order so its keys can be bought again. context.WithoutCancel:
		// this must run even if the client has already gone away.
		if cancelErr := h.cancelOrder(context.WithoutCancel(ctx), orderID); cancelErr != nil {
			slog.ErrorContext(ctx, "cancel order after stripe failure", "order_id", orderID, "error", cancelErr)
		}

		httpx.Error(w, r, httpx.PaymentProviderError())
		return
	}

	// If saving the session ID fails, the webhook still finds the order through
	// the order_id in the session's metadata, so the buyer can go on and pay.
	if err := h.orders.SetStripeSession(ctx, orderID, session.ID); err != nil {
		slog.ErrorContext(ctx, "save stripe session id", "order_id", orderID, "error", err)
	}

	slog.InfoContext(ctx, "checkout started", "order_id", orderID, "stripe_session_id", session.ID)
	httpx.WriteJSON(w, http.StatusCreated, responses.CheckoutResponse{
		OrderID:     orderID,
		CheckoutURL: session.URL,
	})
}

// cancelOrder cancels an order whose Stripe session couldn't be created.
func (h *CheckoutHandler) cancelOrder(ctx context.Context, orderID uuid.UUID) error {
	return database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		return h.orders.WithTx(tx).Cancel(ctx, orderID)
	})
}

// summarizeCheckout checks the cart can be bought and returns its total and
// currency. An empty cart is a 400 cart_empty, an inactive offer a 409
// offer_unavailable. Prices are the offers' live prices.
func summarizeCheckout(lines []repository.CheckoutLine) (int, string, error) {
	if len(lines) == 0 {
		return 0, "", httpx.CartEmpty()
	}

	totalCents := 0
	currency := lines[0].Currency
	for _, line := range lines {
		if !line.IsActive {
			return 0, "", httpx.OfferUnavailable(line.OfferID.String())
		}
		if line.Currency != currency {
			// Every offer is in EUR today, and one Stripe session has one currency.
			return 0, "", fmt.Errorf("checkout: cart mixes currencies %s and %s", currency, line.Currency)
		}
		totalCents += line.PriceCents * line.Quantity
	}

	return totalCents, currency, nil
}

// reserveKeys takes quantity available keys for every line and holds them for
// this checkout. It returns one order item per key. If an offer has fewer keys
// left than the cart wants, it returns a 409 insufficient_stock for that offer.
func reserveKeys(ctx context.Context, orders *repository.OrderRepository, lines []repository.CheckoutLine) ([]repository.NewOrderItem, error) {
	var items []repository.NewOrderItem

	for _, line := range lines {
		keyIDs, err := orders.LockAvailableKeys(ctx, line.OfferID, line.Quantity)
		if err != nil {
			return nil, err
		}
		if len(keyIDs) < line.Quantity {
			return nil, httpx.InsufficientStockForOffer(line.OfferID.String(), len(keyIDs))
		}

		if err := orders.ReserveKeys(ctx, keyIDs); err != nil {
			return nil, err
		}

		for _, keyID := range keyIDs {
			items = append(items, repository.NewOrderItem{
				OfferID:        line.OfferID,
				DigitalKeyID:   keyID,
				UnitPriceCents: line.PriceCents,
			})
		}
	}

	return items, nil
}

// createPendingOrder inserts the order with a new random public ID, trying
// again with another ID if one is already taken.
func createPendingOrder(ctx context.Context, orders *repository.OrderRepository, order repository.NewOrder) (uuid.UUID, error) {
	for range publicIDAttempts {
		publicID, err := services.NewOrderPublicID()
		if err != nil {
			return uuid.Nil, err
		}
		order.PublicID = publicID

		orderID, created, err := orders.CreateOrder(ctx, order)
		if err != nil {
			return uuid.Nil, err
		}
		if created {
			return orderID, nil
		}
	}

	return uuid.Nil, fmt.Errorf("create order: no free public id after %d attempts", publicIDAttempts)
}

// checkoutLineItems turns the cart lines into rows on the Stripe page: one row
// per offer, with the number of keys as the quantity. A cart has each offer once
// already, but lines for the same offer are merged anyway, so Stripe never shows
// the same offer twice.
func checkoutLineItems(lines []repository.CheckoutLine) []services.CheckoutLineItem {
	items := make([]services.CheckoutLineItem, 0, len(lines))
	indexByOffer := make(map[uuid.UUID]int, len(lines))

	for _, line := range lines {
		if i, ok := indexByOffer[line.OfferID]; ok {
			items[i].Quantity += line.Quantity
			continue
		}

		indexByOffer[line.OfferID] = len(items)
		items = append(items, services.CheckoutLineItem{
			Name:            line.Title,
			ImageURL:        line.ImageURL,
			UnitAmountCents: line.PriceCents,
			Quantity:        line.Quantity,
		})
	}

	return items
}
