package responses

import (
	"github.com/google/uuid"

	"eneba-backend/internal/models"
	"eneba-backend/internal/requests"
)

// PriceResponse is the price of an offer, e.g. the "From" price on a product
// card, which comes from the product's cheapest active offer. OriginalCents is
// the product's official base price, shown crossed out, and DiscountPercent is
// how much cheaper the offer is than that. Both are null when the offer isn't
// cheaper than the base price.
type PriceResponse struct {
	Cents           int    `json:"cents"`
	OriginalCents   *int   `json:"originalCents"`
	DiscountPercent *int   `json:"discountPercent"`
	Currency        string `json:"currency"`
}

// ProductListItemResponse is one product card in the store grid.
type ProductListItemResponse struct {
	ID       uuid.UUID          `json:"id"`
	Slug     string             `json:"slug"`
	Title    string             `json:"title"`
	ImageURL string             `json:"imageUrl"`
	Type     models.ProductType `json:"type"`
	Platform models.Platform    `json:"platform"`
	Region   models.Region      `json:"region"`
	Price    PriceResponse      `json:"price"`
}

// ProductListResponse is returned by GET /api/v1/products.
type ProductListResponse struct {
	Items      []ProductListItemResponse `json:"items"`
	Page       int                       `json:"page"`
	Limit      int                       `json:"limit"`
	Total      int                       `json:"total"`
	TotalPages int                       `json:"totalPages"`
}

// NewPriceResponse builds the price of an offer and compares it with the
// product's official base price, e.g. an offer of 1650 for a product with a
// base price of 2999 -> originalCents 2999, discountPercent 45.
// Sellers can also ask more than the base price; then there is no discount to
// show, so originalCents and discountPercent stay null.
func NewPriceResponse(priceCents, basePriceCents int, currency string) PriceResponse {
	price := PriceResponse{
		Cents:    priceCents,
		Currency: currency,
	}

	if basePriceCents > priceCents {
		percent := discountPercent(priceCents, basePriceCents)
		price.OriginalCents = &basePriceCents
		price.DiscountPercent = &percent
	}

	return price
}

// NewProductListResponse wraps one page of product cards with the paging info
// the client needs to show page numbers.
func NewProductListResponse(items []ProductListItemResponse, total int, params requests.ProductListParams) ProductListResponse {
	return ProductListResponse{
		Items:      items,
		Page:       params.Page,
		Limit:      params.Limit,
		Total:      total,
		TotalPages: totalPages(total, params.Limit),
	}
}

// discountPercent is how much cheaper the price is than the base price, rounded
// to the nearest whole percent, e.g. 3299 from 5999 -> 45. basePriceCents must be
// positive. Adding basePriceCents/2 before the integer division rounds to the
// nearest value instead of always rounding down.
func discountPercent(priceCents, basePriceCents int) int {
	return ((basePriceCents-priceCents)*100 + basePriceCents/2) / basePriceCents
}

// totalPages is how many pages of size limit are needed for total items,
// e.g. 52 items with 24 per page -> 3. Adding limit-1 rounds the division up.
func totalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
