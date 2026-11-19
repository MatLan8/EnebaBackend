package responses

import (
	"github.com/google/uuid"

	"eneba-backend/internal/models"
)

// Keys are in the same order as the order's items.
type OrderKeysResponse struct {
	OrderID uuid.UUID          `json:"orderId"`
	Keys    []OrderKeyResponse `json:"keys"`
}

type OrderKeyResponse struct {
	ID      uuid.UUID               `json:"id"`   // the digital key's ID
	Code    string                  `json:"code"` // the decrypted key, e.g. "ABCD-EFGH-IJKL"
	Product OrderKeyProductResponse `json:"product"`
	Seller  OrderKeySellerResponse  `json:"seller"`
}

type OrderKeyProductResponse struct {
	Title    string             `json:"title"`
	ImageURL string             `json:"imageUrl"`
	Type     models.ProductType `json:"type"`
	Platform PlatformResponse   `json:"platform"`
	Region   models.Region      `json:"region"`
}

// SellerResponse without the ID, which the keys page doesn't need.
type OrderKeySellerResponse struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logoUrl"`
	Rating  float64 `json:"rating"` // 0 to 10
}
