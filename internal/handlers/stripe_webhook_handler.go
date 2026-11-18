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

// maxWebhookBodyBytes caps the webhook body. Checkout session events are a few KB.
const maxWebhookBodyBytes = 64 << 10

// StripeWebhookHandler receives events from Stripe.
//
// Why a webhook and not the success redirect: the redirect comes from the
// buyer's browser, which can close before it arrives, and anyone can open the
// success URL without paying. The webhook comes from Stripe itself, is signed
// with STRIPE_WEBHOOK_SECRET, and Stripe retries it until it gets a 2xx.
type StripeWebhookHandler struct {
	pool   *pgxpool.Pool // needed to start transactions
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

// webhookAction is what an event asks us to do with its order.
type webhookAction int

const (
	webhookIgnore   webhookAction = iota // an event we don't handle, or a session that isn't paid yet
	webhookMarkPaid                      // the buyer paid: sell the keys
	webhookCancel                        // the payment page expired: release the keys
)

// webhookActionFor decides what to do with an event. It is a pure function, so
// the rules can be tested without Stripe or a database.
// payment_status can be "unpaid" on checkout.session.completed for delayed
// payment methods (e.g. bank transfers); cards are always "paid" right away.
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

// paidOutcome is what a payment does to an order, depending on its status.
type paidOutcome int

const (
	paidSellKeys        paidOutcome = iota // pending: sell the keys and empty the cart
	paidAlreadyDone                        // paid: a repeated delivery of the same event, nothing to do
	paidAfterCancelling                    // cancelled: the money came in after the keys were released
)

// paidOutcomeFor decides what a payment does to an order in the given status.
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

// Handle handles POST /api/v1/webhooks/stripe. It is public: instead of a
// token, every request must carry a valid Stripe-Signature header.
// It answers 200 for every event it has dealt with or chosen to ignore, and 500
// when the database fails, so that Stripe sends the event again later.
func (h *StripeWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The signature covers the exact bytes Stripe sent, so the raw body is read
	// as-is (no JSON decoding first).
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
		// Not a Checkout session event (or not one we can read): nothing to do.
		slog.InfoContext(ctx, "stripe webhook ignored", "event_id", event.ID, "type", event.Type)
		w.WriteHeader(http.StatusOK)
		return
	}

	// markPaid and cancel log the order_id of the order they found. The order ID
	// from the session's metadata is logged separately; sessions not created by
	// our checkout (e.g. from `stripe trigger`) don't have one.
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

// markPaid sells the order's keys once Stripe reports the payment.
// The order row is locked first, so if Stripe delivers the event twice at the
// same time, the second delivery waits, then sees "paid" and does nothing.
// That makes the webhook idempotent: keys and sold counts change only once.
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
			// The order was cancelled (its session expired, or creating the session
			// looked like it failed) and its keys were released, so we no longer know
			// which keys it had. Stripe doesn't let an expired session be paid, so
			// this should not happen; if it does, someone has to refund by hand.
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

// cancel releases the keys of an order whose payment page expired unpaid.
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
