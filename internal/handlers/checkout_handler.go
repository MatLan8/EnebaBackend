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
	// Stripe's minimum is 30 minutes; the extra minute covers the request itself.
	// Keys are reserved a bit longer (see repository.keyReservationMinutes).
	checkoutSessionTTL = 31 * time.Minute
	publicIDAttempts   = 5
)

type CheckoutHandler struct {
	pool        *pgxpool.Pool
	carts       *repository.CartRepository
	orders      *repository.OrderRepository
	users       *repository.UserRepository
	stripe      *services.StripeService
	frontendURL string
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

// Create reserves the cart's keys and creates a pending order in one transaction, then opens a
// Stripe Checkout session. A cart has at most one pending order: a previous one is cancelled first.
// The cart is only emptied by the webhook after payment; an expired session releases the keys.
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

	expired, err := h.expirePreviousSessions(ctx, cartID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var orderID uuid.UUID
	var lines []repository.CheckoutLine
	var superseded []string

	err = database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		carts := h.carts.WithTx(tx)
		orders := h.orders.WithTx(tx)

		// Locking the cart makes a second checkout of it (e.g. a double click) wait for this one.
		if err := lockExistingCart(ctx, carts, cartID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return httpx.CartEmpty()
			}
			return err
		}

		var err error
		if superseded, err = cancelPendingOrders(ctx, orders, cartID); err != nil {
			return err
		}

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

	// A session created after expirePreviousSessions looked (e.g. by a double click) is expired now.
	for _, sessionID := range superseded {
		if !expired[sessionID] {
			h.expireSupersededSession(ctx, sessionID)
		}
	}

	// Stripe is called after the commit, so a slow network call doesn't keep the cart and keys locked.
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

		// Release the keys; WithoutCancel so this runs even if the client has gone away.
		if cancelErr := h.cancelOrder(context.WithoutCancel(ctx), orderID); cancelErr != nil {
			slog.ErrorContext(ctx, "cancel order after stripe failure", "order_id", orderID, "error", cancelErr)
		}

		httpx.Error(w, r, httpx.PaymentProviderError())
		return
	}

	// Not fatal: the webhook also finds the order by the order_id in the session metadata.
	saved, err := h.orders.SetStripeSession(ctx, orderID, session.ID)
	if err != nil {
		slog.ErrorContext(ctx, "save stripe session id", "order_id", orderID, "error", err)
	} else if !saved {
		// A newer checkout of the cart cancelled this order while its session was being created.
		h.expireSupersededSession(ctx, session.ID)
		httpx.Error(w, r, httpx.CheckoutSuperseded())
		return
	}

	slog.InfoContext(ctx, "checkout started", "order_id", orderID, "stripe_session_id", session.ID)
	httpx.WriteJSON(w, http.StatusCreated, responses.CheckoutResponse{
		OrderID:     orderID,
		CheckoutURL: session.URL,
	})
}

// Runs before the transaction, so no locks are held during the Stripe calls. Once expired, a
// previous session can't be paid any more, so cancelling its order can't lose a payment.
// Returns the expired session IDs.
func (h *CheckoutHandler) expirePreviousSessions(ctx context.Context, cartID uuid.UUID) (map[string]bool, error) {
	pending, err := h.orders.ListPendingForCart(ctx, cartID)
	if err != nil {
		return nil, err
	}

	expired := make(map[string]bool, len(pending))
	for _, order := range pending {
		if order.StripeSessionID == nil {
			continue
		}

		err := h.stripe.ExpireCheckoutSession(ctx, *order.StripeSessionID)
		if errors.Is(err, services.ErrCheckoutSessionCompleted) {
			return nil, httpx.CheckoutAlreadyCompleted(order.ID.String())
		}
		if err != nil {
			slog.ErrorContext(ctx, "expire previous stripe session", "order_id", order.ID, "error", err)
			return nil, httpx.PaymentProviderError()
		}

		expired[*order.StripeSessionID] = true
	}

	return expired, nil
}

// Returns the Stripe session IDs of the cancelled orders.
func cancelPendingOrders(ctx context.Context, orders *repository.OrderRepository, cartID uuid.UUID) ([]string, error) {
	pending, err := orders.ListPendingForCart(ctx, cartID)
	if err != nil {
		return nil, err
	}

	var sessionIDs []string
	for _, order := range pending {
		if err := orders.Cancel(ctx, order.ID); err != nil {
			return nil, err
		}
		if order.StripeSessionID != nil {
			sessionIDs = append(sessionIDs, *order.StripeSessionID)
		}
	}

	return sessionIDs, nil
}

// Best effort: the order is already cancelled, and a session Stripe fails to expire just runs out.
func (h *CheckoutHandler) expireSupersededSession(ctx context.Context, sessionID string) {
	err := h.stripe.ExpireCheckoutSession(context.WithoutCancel(ctx), sessionID)
	if errors.Is(err, services.ErrCheckoutSessionCompleted) {
		slog.ErrorContext(ctx, "superseded stripe session was paid, refund manually", "stripe_session_id", sessionID)
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "expire superseded stripe session", "stripe_session_id", sessionID, "error", err)
	}
}

func (h *CheckoutHandler) cancelOrder(ctx context.Context, orderID uuid.UUID) error {
	return database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		return h.orders.WithTx(tx).Cancel(ctx, orderID)
	})
}

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

// One Stripe row per offer with the key count as quantity; lines of the same offer are merged.
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
