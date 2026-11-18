package responses

import (
	"github.com/google/uuid"

	"eneba-backend/internal/models"
)

// CartResponse is returned by GET /api/v1/cart and by every cart change except
// DELETE /api/v1/cart, so the frontend never has to fetch the cart again.
// Prices are read live from the offers; they are only locked in at checkout.
type CartResponse struct {
	Items      []CartItemResponse `json:"items"`
	ItemCount  int                `json:"itemCount"`  // sum of quantities, for the cart badge
	TotalCents int                `json:"totalCents"` // sum of price x quantity
	Currency   string             `json:"currency"`
}

// CartItemResponse is one offer in the cart. The line price is
// PriceCents x Quantity, worked out by the frontend.
type CartItemResponse struct {
	OfferID           uuid.UUID           `json:"offerId"`
	Quantity          int                 `json:"quantity"`
	AvailableQuantity int                 `json:"availableQuantity"` // keys not sold or reserved, caps the "+" button
	PriceCents        int                 `json:"priceCents"`        // one key, the offer's current price
	Product           CartProductResponse `json:"product"`
	Seller            CartSellerResponse  `json:"seller"`
}

type CartProductResponse struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	ImageURL string `json:"imageUrl"`
	IsDLC    bool   `json:"isDlc"` // shows the DLC badge
}

type CartSellerResponse struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logoUrl"`
}

// NewCartResponse adds up the items into a cart. A nil items becomes an empty
// slice, so the JSON has [] and not null. An empty currency (an empty cart)
// becomes models.DefaultCurrency.
func NewCartResponse(items []CartItemResponse, currency string) CartResponse {
	if items == nil {
		items = []CartItemResponse{}
	}
	if currency == "" {
		currency = models.DefaultCurrency
	}

	cart := CartResponse{
		Items:    items,
		Currency: currency,
	}
	for _, item := range items {
		cart.ItemCount += item.Quantity
		cart.TotalCents += item.PriceCents * item.Quantity
	}

	return cart
}
