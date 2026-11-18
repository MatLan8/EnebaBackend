package responses

import (
	"github.com/google/uuid"

	"eneba-backend/internal/models"
)

// OrderKeysResponse is returned by GET /api/v1/orders/{id}/keys: the
// plaintext codes of a paid order's keys, in the same order as the order's items.
type OrderKeysResponse struct {
	OrderID uuid.UUID          `json:"orderId"`
	Keys    []OrderKeyResponse `json:"keys"`
}

// OrderKeyResponse is one bought key with the product and seller it came from.
type OrderKeyResponse struct {
	ID      uuid.UUID               `json:"id"`   // the digital key's ID
	Code    string                  `json:"code"` // the decrypted key, e.g. "ABCD-EFGH-IJKL"
	Product OrderKeyProductResponse `json:"product"`
	Seller  OrderKeySellerResponse  `json:"seller"`
}

// OrderKeyProductResponse is the product a key activates. The fields have the
// same names and types as on the product page.
type OrderKeyProductResponse struct {
	Title    string             `json:"title"`
	ImageURL string             `json:"imageUrl"`
	Type     models.ProductType `json:"type"`
	Platform models.Platform    `json:"platform"`
	Region   models.Region      `json:"region"`
}

// OrderKeySellerResponse is the seller a key was bought from. It is
// SellerResponse without the ID, which the keys page doesn't need.
type OrderKeySellerResponse struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logoUrl"`
	Rating  float64 `json:"rating"` // 0 to 10
}
