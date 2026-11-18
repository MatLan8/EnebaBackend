package responses

import (
	"github.com/google/uuid"

	"eneba-backend/internal/models"
)

// Offers are sorted cheapest first; the frontend shows the first two as recommended.
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
	Currency       string             `json:"currency"`       // of basePriceCents; base prices are in models.DefaultCurrency
	Genres         []GenreResponse    `json:"genres"`
	Offers         []OfferResponse    `json:"offers"`
}

type GenreResponse struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type OfferResponse struct {
	ID     uuid.UUID      `json:"id"`
	Price  PriceResponse  `json:"price"`
	Seller SellerResponse `json:"seller"`
}

type SellerResponse struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	LogoURL *string   `json:"logoUrl"`
	Rating  float64   `json:"rating"` // 0 to 10
}
