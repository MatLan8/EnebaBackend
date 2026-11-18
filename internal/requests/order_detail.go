package requests

import (
	"github.com/google/uuid"

	"eneba-backend/internal/httpx"
)

// ParseOrderIDParam reads the {id} URL parameter of GET /api/v1/orders/{id}.
// Like the cart's {offerId}, a value that isn't a UUID is a 400.
func ParseOrderIDParam(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, httpx.ValidationFailed(map[string]string{"id": "must be a valid UUID"})
	}
	if id == uuid.Nil {
		return uuid.Nil, httpx.ValidationFailed(map[string]string{"id": "must not be the nil UUID"})
	}

	return id, nil
}
