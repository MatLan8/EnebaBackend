package requests

import (
	"fmt"

	"github.com/google/uuid"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
)

// AddCartItemRequest is the body of POST /api/v1/cart/items. Each call adds
// exactly one key of the offer, the same as one click on "Add to cart".
// OfferID is a pointer so a missing field can be told apart from an empty one.
type AddCartItemRequest struct {
	OfferID *string `json:"offerId"`
}

// Validate checks the body and returns the parsed offer ID.
// On failure it returns a 400 *httpx.APIError.
func (r AddCartItemRequest) Validate() (uuid.UUID, error) {
	if r.OfferID == nil {
		return uuid.Nil, httpx.ValidationFailed(map[string]string{"offerId": "is required"})
	}

	offerID, problem := parseOfferID(*r.OfferID)
	if problem != "" {
		return uuid.Nil, httpx.ValidationFailed(map[string]string{"offerId": problem})
	}

	return offerID, nil
}

// UpdateCartItemRequest is the body of PATCH /api/v1/cart/items/{offerId}.
// Quantity is the new amount, not a change: "+" in the cart sends the current
// quantity plus one, so a retried request can't add a key twice.
type UpdateCartItemRequest struct {
	Quantity *int `json:"quantity"`
}

// Validate checks the body and returns the new quantity.
// 0 is not allowed: removing an item is DELETE /api/v1/cart/items/{offerId}.
func (r UpdateCartItemRequest) Validate() (int, error) {
	if problem := checkQuantity(r.Quantity); problem != "" {
		return 0, httpx.ValidationFailed(map[string]string{"quantity": problem})
	}

	return *r.Quantity, nil
}

// ParseOfferIDParam reads the {offerId} URL parameter of the cart item routes.
func ParseOfferIDParam(raw string) (uuid.UUID, error) {
	id, problem := parseOfferID(raw)
	if problem != "" {
		return uuid.Nil, httpx.ValidationFailed(map[string]string{"offerId": problem})
	}

	return id, nil
}

// parseOfferID returns the parsed ID, or a message describing what is wrong with it.
func parseOfferID(raw string) (uuid.UUID, string) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, "must be a valid UUID"
	}
	if id == uuid.Nil {
		return uuid.Nil, "must not be the nil UUID"
	}

	return id, ""
}

// checkQuantity returns a message if quantity is missing or outside 1 to
// models.MaxCartItemQuantity, or "" if it is fine.
func checkQuantity(quantity *int) string {
	if quantity == nil {
		return "is required"
	}
	if *quantity < 1 || *quantity > models.MaxCartItemQuantity {
		return fmt.Sprintf("must be between 1 and %d", models.MaxCartItemQuantity)
	}

	return ""
}
