package responses

import (
	"github.com/google/uuid"

	"eneba-backend/internal/models"
	"eneba-backend/internal/requests"
)

// PriceResponse is the "From" price on a product card, taken from the
// product's cheapest active offer. OriginalCents and DiscountPercent are
// null when that offer isn't discounted.
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

// NewPriceResponse builds the "From" price of a product card from its cheapest
// active offer and works out the discount.
func NewPriceResponse(priceCents int, originalCents *int, currency string) PriceResponse {
	return PriceResponse{
		Cents:           priceCents,
		OriginalCents:   originalCents,
		DiscountPercent: discountPercent(priceCents, originalCents),
		Currency:        currency,
	}
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

// discountPercent is how much cheaper the price is than the original, rounded
// to the nearest whole percent, e.g. 3299 from 5999 -> 45. It returns nil when
// there is no original price. Adding original/2 before the integer division
// rounds to the nearest value instead of always rounding down.
func discountPercent(priceCents int, originalCents *int) *int {
	if originalCents == nil || *originalCents <= 0 {
		return nil
	}

	original := *originalCents
	percent := ((original-priceCents)*100 + original/2) / original
	return &percent
}

// totalPages is how many pages of size limit are needed for total items,
// e.g. 52 items with 24 per page -> 3. Adding limit-1 rounds the division up.
func totalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
