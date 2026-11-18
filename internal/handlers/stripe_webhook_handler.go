package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"eneba-backend/internal/database"
	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/requests"
	"eneba-backend/internal/services"
)

// Checkout session events are a few KB.
const maxWebhookBodyBytes = 64 << 10

// Payment is confirmed by this webhook, not the success redirect: the buyer's browser can close
// before the redirect, and anyone can open the success URL. Webhooks are signed and retried by Stripe.
type StripeWebhookHandler struct {
	pool   *pgxpool.Pool
	orders *repository.OrderRepository
	carts  *repository.CartRepository
	stripe *services.StripeService
}

func NewStripeWebhookHandler(
	pool *pgxpool.Pool,
	orders *repository.OrderRepository,
	carts *repository.CartRepository,
	stripe *services.StripeService,
) *StripeWebhookHandler {
	return &StripeWebhookHandler{pool: pool, orders: orders, carts: carts, stripe: stripe}
}

type webhookAction int

const (
	webhookIgnore   webhookAction = iota // an event we don't handle, or a session that isn't paid yet
	webhookMarkPaid                      // the buyer paid: sell the keys
	webhookCancel                        // the payment page expired: release the keys
)

// payment_status can be "unpaid" on checkout.session.completed for delayed methods
// (e.g. bank transfers); cards are always "paid" right away.
func webhookActionFor(eventType, paymentStatus string) webhookAction {
	switch eventType {
	case "checkout.session.completed":
		if paymentStatus == "paid" {
			return webhookMarkPaid
		}
		return webhookIgnore
	case "checkout.session.expired":
		return webhookCancel
	default:
		return webhookIgnore
	}
}

type paidOutcome int

const (
	paidSellKeys        paidOutcome = iota // pending: sell the keys and empty the cart
	paidAlreadyDone                        // paid: a repeated delivery of the same event, nothing to do
	paidAfterCancelling                    // cancelled: the money came in after the keys were released
)

func paidOutcomeFor(status models.OrderStatus) paidOutcome {
	switch status {
	case models.OrderStatusPaid:
		return paidAlreadyDone
	case models.OrderStatusCancelled:
		return paidAfterCancelling
	default:
		return paidSellKeys
	}
}

// Public, authenticated by the Stripe-Signature header. Database errors answer 500 so Stripe
// retries; every event handled or ignored gets a 200.
func (h *StripeWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The signature covers the exact bytes Stripe sent, so the raw body is read without decoding.
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes))
	if err != nil {
		slog.WarnContext(ctx, "stripe webhook rejected: body too large or unreadable", "error", err)
		httpx.Error(w, r, httpx.BadRequest("request body is too large or unreadable"))
		return
	}

	event, err := h.stripe.ConstructEvent(payload, r.Header.Get("Stripe-Signature"))
	if err != nil {
		// Usually a wrong STRIPE_WEBHOOK_SECRET (signature) or a malformed event.
		slog.WarnContext(ctx, "stripe webhook rejected: signature or event invalid", "error", err)
		httpx.Error(w, r, httpx.InvalidSignature())
		return
	}

	session, err := requests.ParseStripeCheckoutSession(event.Object)
	if err != nil {
		slog.InfoContext(ctx, "stripe webhook ignored", "event_id", event.ID, "type", event.Type)
		w.WriteHeader(http.StatusOK)
		return
	}

	// The metadata order_id is logged separately; sessions not created by our checkout
	// (e.g. `stripe trigger`) don't have one.
	log := slog.With("event_id", event.ID, "type", event.Type, "stripe_session_id", session.ID)
	if metadataOrderID, ok := session.OrderID(); ok {
		log = log.With("metadata_order_id", metadataOrderID)
	}

	switch webhookActionFor(event.Type, session.PaymentStatus) {
	case webhookMarkPaid:
		err = h.markPaid(ctx, log, session)
	case webhookCancel:
		err = h.cancel(ctx, log, session)
	default:
		log.InfoContext(ctx, "stripe webhook ignored", "payment_status", session.PaymentStatus)
	}

	if errors.Is(err, models.ErrNotFound) {
		// e.g. a session made by `stripe trigger`, or an order deleted since.
		// Answering 200 stops Stripe from retrying an event we can never handle.
		log.WarnContext(ctx, "stripe webhook: no order for this session")
		err = nil
	}
	if err != nil {
		log.ErrorContext(ctx, "stripe webhook failed", "error", err)
		httpx.Error(w, r, err) // 500, Stripe retries
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Locking the order makes the webhook idempotent: a duplicate delivery waits, then sees "paid"
// and does nothing, so keys and sold counts change only once.
func (h *StripeWebhookHandler) markPaid(ctx context.Context, log *slog.Logger, session requests.StripeCheckoutSession) error {
	orderID, _ := session.OrderID()

	return database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		orders := h.orders.WithTx(tx)

		order, err := orders.GetForUpdate(ctx, session.ID, orderID)
		if err != nil {
			return err
		}

		switch paidOutcomeFor(order.Status) {
		case paidAlreadyDone:
			log.InfoContext(ctx, "stripe webhook: order already paid", "order_id", order.ID)
			return nil
		case paidAfterCancelling:
			// Its keys were released, so we no longer know which ones it had. Stripe doesn't allow paying
			// an expired session, so this shouldn't happen; if it does, refund by hand.
			log.ErrorContext(ctx, "stripe webhook: payment for a cancelled order, refund manually", "order_id", order.ID)
			return nil
		}

		if err := orders.MarkPaid(ctx, order.ID, session.ID); err != nil {
			return err
		}
		if err := orders.MarkKeysSold(ctx, order.ID); err != nil {
			return err
		}
		if err := orders.IncrementSoldCounts(ctx, order.ID); err != nil {
			return err
		}

		// The cart is emptied only now that the payment went through.
		if order.CartID != nil {
			carts := h.carts.WithTx(tx)
			if err := carts.Clear(ctx, *order.CartID); err != nil {
				return err
			}
			if err := carts.Touch(ctx, *order.CartID); err != nil {
				return err
			}
		}

		log.InfoContext(ctx, "stripe webhook: order paid", "order_id", order.ID)
		return nil
	})
}

// Paid or already cancelled orders are left alone.
func (h *StripeWebhookHandler) cancel(ctx context.Context, log *slog.Logger, session requests.StripeCheckoutSession) error {
	orderID, _ := session.OrderID()

	return database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		orders := h.orders.WithTx(tx)

		order, err := orders.GetForUpdate(ctx, session.ID, orderID)
		if err != nil {
			return err
		}
		if order.Status != models.OrderStatusPending {
			log.InfoContext(ctx, "stripe webhook: order not pending, nothing to cancel", "order_id", order.ID, "status", order.Status)
			return nil
		}

		if err := orders.Cancel(ctx, order.ID); err != nil {
			return err
		}

		log.InfoContext(ctx, "stripe webhook: order cancelled, keys released", "order_id", order.ID)
		return nil
	})
}
