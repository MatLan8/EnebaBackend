package httpx

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"eneba-backend/internal/models"
)

type APIError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any // optional; values keep their JSON type, e.g. availableQuantity is a number
}

func (e *APIError) Error() string {
	return e.Message
}

func BadRequest(message string) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: message}
}

// details maps a field or query parameter name to its problem.
func ValidationFailed(details map[string]string) *APIError {
	typed := make(map[string]any, len(details))
	for field, problem := range details {
		typed[field] = problem
	}
	return &APIError{Status: http.StatusBadRequest, Code: "validation_failed", Message: "request validation failed", Details: typed}
}

// available lets the client lower the quantity.
func InsufficientStock(available int) *APIError {
	return &APIError{
		Status:  http.StatusConflict,
		Code:    "insufficient_stock",
		Message: fmt.Sprintf("Not enough keys in stock"),
		Details: map[string]any{"availableQuantity": available},
	}
}

func InsufficientStockForOffer(offerID string, available int) *APIError {
	err := InsufficientStock(available)
	err.Details["offerId"] = offerID
	return err
}

func CartEmpty() *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "cart_empty", Message: "the cart is empty"}
}

// offerID tells the client which cart item to remove.
func OfferUnavailable(offerID string) *APIError {
	return &APIError{
		Status:  http.StatusConflict,
		Code:    "offer_unavailable",
		Message: "An offer in the cart is no longer available",
		Details: map[string]any{"offerId": offerID},
	}
}

func PaymentProviderError() *APIError {
	return &APIError{Status: http.StatusBadGateway, Code: "payment_provider_error", Message: "the payment provider is unavailable, please try again"}
}

// status tells the client whether the order is pending or cancelled.
func OrderNotPaid(status string) *APIError {
	return &APIError{
		Status:  http.StatusConflict,
		Code:    "order_not_paid",
		Message: "The order is not paid, so its keys are not available",
		Details: map[string]any{"status": status},
	}
}

// The cart's previous Stripe session was already paid; orderID is that order, which the
// webhook is about to mark paid (or already has).
func CheckoutAlreadyCompleted(orderID string) *APIError {
	return &APIError{
		Status:  http.StatusConflict,
		Code:    "checkout_already_completed",
		Message: "This cart was already paid for",
		Details: map[string]any{"orderId": orderID},
	}
}

// A newer checkout of the same cart (e.g. a double click) replaced this one while it was starting.
func CheckoutSuperseded() *APIError {
	return &APIError{Status: http.StatusConflict, Code: "checkout_superseded", Message: "a newer checkout of this cart replaced this one"}
}

func InvalidSignature() *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "invalid_signature", Message: "invalid webhook signature"}
}

// {"error": {"code": "not_found", "message": "resource not found"}}
type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Unknown errors become a generic 500 and are logged in full, so SQL errors never leak to the client.
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

func toAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}

	switch {
	case errors.Is(err, models.ErrNotFound):
		return &APIError{Status: http.StatusNotFound, Code: "not_found", Message: "Resource not found"}
	case errors.Is(err, models.ErrEmailTaken):
		return &APIError{Status: http.StatusConflict, Code: "email_taken", Message: "Email is already registered"}
	case errors.Is(err, models.ErrInvalidCredentials):
		return &APIError{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "Invalid email or password"}
	case errors.Is(err, models.ErrInvalidToken):
		return &APIError{Status: http.StatusUnauthorized, Code: "invalid_token", Message: "Invalid or expired token"}
	case errors.Is(err, models.ErrUnauthorized):
		return &APIError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Authentication required"}
	default:
		return &APIError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "Something went wrong"}
	}
}
