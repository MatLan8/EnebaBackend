package responses

import (
	"time"

	"github.com/google/uuid"

	"eneba-backend/internal/models"
)

// OrderDetailItemResponse is one bought key of an order. There is no quantity:
// an order has one item per key, so two keys of the same offer are two items.
type OrderDetailItemResponse struct {
	ID             uuid.UUID `json:"id"` // the order item's ID
	Title          string    `json:"title"`
	ImageURL       string    `json:"imageUrl"`
	UnitPriceCents int       `json:"unitPriceCents"` // the price paid, fixed at checkout
	SellerName     string    `json:"sellerName"`
}

// OrderDetailResponse is returned by GET /api/v1/orders/{id}.
// A cancelled order has no items (cancelling deletes them to free the keys), so
// items and sellerNames are both [].
type OrderDetailResponse struct {
	ID          uuid.UUID                 `json:"id"`
	PublicID    string                    `json:"publicId"`
	CreatedAt   time.Time                 `json:"createdAt"`
	PaidAt      *time.Time                `json:"paidAt"` // null until the order is paid
	Status      models.OrderStatus        `json:"status"`
	SellerNames []string                  `json:"sellerNames"`
	TotalCents  int                       `json:"totalCents"`
	Currency    string                    `json:"currency"`
	Items       []OrderDetailItemResponse `json:"items"`
}

// DistinctSellerNames lists each seller of items once, in the order they first
// appear, e.g. items from A, B, A -> [A, B]. It always returns a slice, never
// nil, so no items gives [] in the JSON.
func DistinctSellerNames(items []OrderDetailItemResponse) []string {
	names := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))

	for _, item := range items {
		if seen[item.SellerName] {
			continue
		}
		seen[item.SellerName] = true
		names = append(names, item.SellerName)
	}

	return names
}
