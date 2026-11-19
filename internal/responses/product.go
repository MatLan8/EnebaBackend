package responses

import (
	"github.com/google/uuid"

	"eneba-backend/internal/models"
	"eneba-backend/internal/requests"
)

// DiscountPercent compares the offer with the product's base price (sent once as basePriceCents);
// null when the offer isn't cheaper.
type PriceResponse struct {
	Cents           int    `json:"cents"`
	DiscountPercent *int   `json:"discountPercent"`
	Currency        string `json:"currency"`
}

// Sent wherever a product is shown, except the store filters.
type PlatformResponse struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	IconURL string `json:"iconUrl"`
}

type ProductListItemResponse struct {
	ID             uuid.UUID          `json:"id"`
	Slug           string             `json:"slug"`
	Title          string             `json:"title"`
	ImageURL       string             `json:"imageUrl"`
	Type           models.ProductType `json:"type"`
	Platform       PlatformResponse   `json:"platform"`
	Region         models.Region      `json:"region"`
	BasePriceCents int                `json:"basePriceCents"` // official price, shown crossed out
	Currency       string             `json:"currency"`       // of basePriceCents; base prices are in models.DefaultCurrency
	Price          PriceResponse      `json:"price"`
	// The offer that price comes from, so the store can add it to the cart without opening the product.
	CheapestOfferID uuid.UUID `json:"cheapestOfferId"`
}

type ProductListResponse struct {
	Items      []ProductListItemResponse `json:"items"`
	Page       int                       `json:"page"`
	Limit      int                       `json:"limit"`
	Total      int                       `json:"total"`
	TotalPages int                       `json:"totalPages"`
}

// An offer at (or above) the base price has no discount to show, so discountPercent stays null.
func NewPriceResponse(priceCents, basePriceCents int, currency string) PriceResponse {
	price := PriceResponse{
		Cents:    priceCents,
		Currency: currency,
	}

	if basePriceCents > priceCents {
		percent := discountPercent(priceCents, basePriceCents)
		price.DiscountPercent = &percent
	}

	return price
}

func NewProductListResponse(items []ProductListItemResponse, total int, params requests.ProductListParams) ProductListResponse {
	return ProductListResponse{
		Items:      items,
		Page:       params.Page,
		Limit:      params.Limit,
		Total:      total,
		TotalPages: totalPages(total, params.Limit),
	}
}

// Rounded to the nearest percent: adding basePriceCents/2 before the integer division rounds
// instead of truncating. basePriceCents must be positive.
func discountPercent(priceCents, basePriceCents int) int {
	return ((basePriceCents-priceCents)*100 + basePriceCents/2) / basePriceCents
}

// Adding limit-1 rounds the division up.
func totalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
