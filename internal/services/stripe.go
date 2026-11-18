package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"
)

type StripeService struct {
	client        *stripe.Client
	webhookSecret string // whsec_..., signs every webhook request
}

func NewStripeService(secretKey, webhookSecret string) *StripeService {
	return &StripeService{
		client:        stripe.NewClient(secretKey),
		webhookSecret: webhookSecret,
	}
}

type CheckoutLineItem struct {
	Name            string
	ImageURL        string // shown only if it is an https URL
	UnitAmountCents int
	Quantity        int
}

type CheckoutSessionParams struct {
	OrderID       uuid.UUID
	CustomerEmail string
	Currency      string // "EUR"; Stripe wants it in lower case
	LineItems     []CheckoutLineItem
	SuccessURL    string
	CancelURL     string
	ExpiresAt     time.Time // Stripe accepts 30 minutes to 24 hours from now
}

type CheckoutSession struct {
	ID  string // cs_test_...
	URL string // https://checkout.stripe.com/..., where the buyer pays
}

// The order ID is the idempotency key, so a retried request (e.g. after a timeout) returns
// the same session instead of creating a second one.
func (s *StripeService) CreateCheckoutSession(ctx context.Context, p CheckoutSessionParams) (CheckoutSession, error) {
	currency := strings.ToLower(p.Currency)

	lineItems := make([]*stripe.CheckoutSessionCreateLineItemParams, 0, len(p.LineItems))
	for _, item := range p.LineItems {
		productData := &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
			Name: stripe.String(item.Name),
		}
		if isHTTPSURL(item.ImageURL) {
			productData.Images = []*string{stripe.String(item.ImageURL)}
		}

		lineItems = append(lineItems, &stripe.CheckoutSessionCreateLineItemParams{
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:    stripe.String(currency),
				UnitAmount:  stripe.Int64(int64(item.UnitAmountCents)),
				ProductData: productData,
			},
			Quantity: stripe.Int64(int64(item.Quantity)),
		})
	}

	orderID := p.OrderID.String()
	params := &stripe.CheckoutSessionCreateParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems:         lineItems,
		CustomerEmail:     stripe.String(p.CustomerEmail),
		ClientReferenceID: stripe.String(orderID),
		Metadata:          map[string]string{"order_id": orderID},
		ExpiresAt:         stripe.Int64(p.ExpiresAt.Unix()),
		SuccessURL:        stripe.String(p.SuccessURL),
		CancelURL:         stripe.String(p.CancelURL),
	}
	params.SetIdempotencyKey("checkout-" + orderID)

	session, err := s.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("create stripe checkout session: %w", err)
	}

	return CheckoutSession{ID: session.ID, URL: session.URL}, nil
}

// The session was paid, so it can no longer be expired.
var ErrCheckoutSessionCompleted = errors.New("stripe checkout session is already complete")

// An already expired session counts as success, so this is safe to call twice.
func (s *StripeService) ExpireCheckoutSession(ctx context.Context, sessionID string) error {
	_, err := s.client.V1CheckoutSessions.Expire(ctx, sessionID, nil)
	if err == nil {
		return nil
	}

	// Stripe only expires open sessions; the session's status tells why it refused.
	session, getErr := s.client.V1CheckoutSessions.Retrieve(ctx, sessionID, nil)
	if getErr != nil {
		return fmt.Errorf("expire stripe checkout session: %w", err)
	}

	switch session.Status {
	case stripe.CheckoutSessionStatusExpired:
		return nil
	case stripe.CheckoutSessionStatusComplete:
		return ErrCheckoutSessionCompleted
	default:
		return fmt.Errorf("expire stripe checkout session: %w", err)
	}
}

// Object is the raw JSON of the object the event is about, e.g. the Checkout session.
type StripeEvent struct {
	ID     string
	Type   string
	Object json.RawMessage
}

// Without a secret no request can be trusted, so all of them are rejected.
var ErrWebhookNotConfigured = errors.New("stripe webhook secret is not configured")

// Signatures older than 5 minutes fail. API version mismatches are ignored: only a few stable
// session fields are read, and `stripe listen` sends events in the account's API version.
func (s *StripeService) ConstructEvent(payload []byte, signatureHeader string) (StripeEvent, error) {
	if s.webhookSecret == "" {
		return StripeEvent{}, ErrWebhookNotConfigured
	}

	event, err := webhook.ConstructEventWithOptions(payload, signatureHeader, s.webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		return StripeEvent{}, err
	}

	stripeEvent := StripeEvent{ID: event.ID, Type: string(event.Type)}
	if event.Data != nil {
		stripeEvent.Object = event.Data.Raw
	}

	return stripeEvent, nil
}

// isHTTPSURL reports whether raw is an absolute https URL. Stripe only shows
// product images from https URLs.
func isHTTPSURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != ""
}
