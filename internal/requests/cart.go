package requests

import (
	"fmt"

	"github.com/google/uuid"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
)

// OfferID is a pointer so a missing field can be told apart from an empty one.
type AddCartItemRequest struct {
	OfferID *string `json:"offerId"`
}

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

// Quantity is the new amount, not a change, so a retried request can't add a key twice.
type UpdateCartItemRequest struct {
	Quantity *int `json:"quantity"`
}

// 0 is not allowed: removing an item is DELETE /api/v1/cart/items/{offerId}.
func (r UpdateCartItemRequest) Validate() (int, error) {
	if problem := checkQuantity(r.Quantity); problem != "" {
		return 0, httpx.ValidationFailed(map[string]string{"quantity": problem})
	}

	return *r.Quantity, nil
}

func ParseOfferIDParam(raw string) (uuid.UUID, error) {
	id, problem := parseOfferID(raw)
	if problem != "" {
		return uuid.Nil, httpx.ValidationFailed(map[string]string{"offerId": problem})
	}

	return id, nil
}

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

func checkQuantity(quantity *int) string {
	if quantity == nil {
		return "is required"
	}
	if *quantity < 1 || *quantity > models.MaxCartItemQuantity {
		return fmt.Sprintf("must be between 1 and %d", models.MaxCartItemQuantity)
	}

	return ""
}
