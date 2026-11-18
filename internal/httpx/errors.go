package httpx

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5/middleware"

	"eneba-backend/internal/models"
)

// APIError is an error that already knows its HTTP status and the
// message the client should see.
type APIError struct {
	Status  int
	Code    string
	Message string
	Details map[string]string // optional, e.g. field -> problem for validation errors
}

func (e *APIError) Error() string {
	return e.Message
}

// BadRequest builds a 400 error with the given message.
func BadRequest(message string) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: message}
}

// ValidationFailed builds a 400 error listing the problem with each field.
func ValidationFailed(details map[string]string) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "validation_failed", Message: "request validation failed", Details: details}
}

// InsufficientStock builds a 409 error for asking for more keys than an offer has left.
// available is how many keys can still be bought, so the client can lower the quantity.
func InsufficientStock(available int) *APIError {
	return &APIError{
		Status:  http.StatusConflict,
		Code:    "insufficient_stock",
		Message: fmt.Sprintf("not enough keys in stock, %d available", available),
		Details: map[string]string{"availableQuantity": strconv.Itoa(available)},
	}
}

// InsufficientStockForOffer is InsufficientStock for one offer of a cart, used
// at checkout, where the cart can hold several offers.
func InsufficientStockForOffer(offerID string, available int) *APIError {
	err := InsufficientStock(available)
	err.Details["offerId"] = offerID
	return err
}

// CartEmpty builds a 400 error for a checkout without a cart or without items.
func CartEmpty() *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "cart_empty", Message: "the cart is empty"}
}

// OfferUnavailable builds a 409 error for a cart offer that can no longer be
// bought (it was deactivated). offerID tells the client which item to remove.
func OfferUnavailable(offerID string) *APIError {
	return &APIError{
		Status:  http.StatusConflict,
		Code:    "offer_unavailable",
		Message: "an offer in the cart is no longer available",
		Details: map[string]string{"offerId": offerID},
	}
}

// PaymentProviderError builds a 502 error for when Stripe can't be reached or refuses a request.
func PaymentProviderError() *APIError {
	return &APIError{Status: http.StatusBadGateway, Code: "payment_provider_error", Message: "the payment provider is unavailable, please try again"}
}

// OrderNotPaid builds a 409 error for asking for the keys of an order that
// isn't paid (yet). status tells the client whether it is pending or cancelled.
func OrderNotPaid(status string) *APIError {
	return &APIError{
		Status:  http.StatusConflict,
		Code:    "order_not_paid",
		Message: "the order is not paid, so its keys are not available",
		Details: map[string]string{"status": status},
	}
}

// InvalidSignature builds a 400 error for a webhook request without a valid signature.
func InvalidSignature() *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "invalid_signature", Message: "invalid webhook signature"}
}

// errorResponse is the JSON shape of every error the API returns:
// {"error": {"code": "not_found", "message": "resource not found"}}
type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Error writes err as a JSON error response with the right status code.
// Unknown errors become a generic 500 and are logged with full detail,
// so internal messages (SQL errors etc.) never leak to the client.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := toAPIError(err)

	if apiErr.Status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
			"request_id", middleware.GetReqID(r.Context()),
		)
	}

	WriteJSON(w, apiErr.Status, errorResponse{
		Error: errorDetail{Code: apiErr.Code, Message: apiErr.Message, Details: apiErr.Details},
	})
}

// toAPIError maps domain errors to HTTP errors.
// errors.Is also matches wrapped errors, e.g. fmt.Errorf("get user: %w", models.ErrNotFound).
func toAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}

	switch {
	case errors.Is(err, models.ErrNotFound):
		return &APIError{Status: http.StatusNotFound, Code: "not_found", Message: "resource not found"}
	case errors.Is(err, models.ErrEmailTaken):
		return &APIError{Status: http.StatusConflict, Code: "email_taken", Message: "email is already registered"}
	case errors.Is(err, models.ErrInvalidCredentials):
		return &APIError{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "invalid email or password"}
	case errors.Is(err, models.ErrInvalidToken):
		return &APIError{Status: http.StatusUnauthorized, Code: "invalid_token", Message: "invalid or expired token"}
	case errors.Is(err, models.ErrUnauthorized):
		return &APIError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "authentication required"}
	default:
		return &APIError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "something went wrong"}
	}
}
