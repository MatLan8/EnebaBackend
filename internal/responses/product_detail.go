package responses

import (
	"github.com/google/uuid"

	"eneba-backend/internal/models"
)

// ProductDetailResponse is returned by GET /api/v1/products/{slug}.
// Offers are sorted cheapest first, so the frontend shows the first two as
// recommended and the rest below them.
type ProductDetailResponse struct {
	ID             uuid.UUID          `json:"id"`
	Slug           string             `json:"slug"`
	Title          string             `json:"title"`
	Description    string             `json:"description"`
	ImageURL       string             `json:"imageUrl"`
	Type           models.ProductType `json:"type"`
	Platform       models.Platform    `json:"platform"`
	Region         models.Region      `json:"region"`
	Developer      *string            `json:"developer"`
	Publisher      *string            `json:"publisher"`
	ReleaseDate    string             `json:"releaseDate"`    // "YYYY-MM-DD"
	BasePriceCents int                `json:"basePriceCents"` // official price, offers' discounts are measured against it
	Genres         []GenreResponse    `json:"genres"`
	Offers         []OfferResponse    `json:"offers"`
}

// GenreResponse is one genre tag on the product page.
type GenreResponse struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// OfferResponse is one seller's offer on the product page. ID is what the
// cart will send when the offer is bought.
type OfferResponse struct {
	ID     uuid.UUID      `json:"id"`
	Price  PriceResponse  `json:"price"`
	Seller SellerResponse `json:"seller"`
}

// SellerResponse is the seller shown next to an offer.
type SellerResponse struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	LogoURL *string   `json:"logoUrl"`
	Rating  float64   `json:"rating"` // 0 to 10
}
