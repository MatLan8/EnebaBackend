package responses

import (
	"time"

	"github.com/google/uuid"

	"eneba-backend/internal/requests"
)

// KeyListItemResponse is one key in the signed-in user's key library. It has
// no code: the codes are only sent by GET /api/v1/orders/{id}/keys.
type KeyListItemResponse struct {
	ID              uuid.UUID `json:"id"` // the digital key's ID
	OrderID         uuid.UUID `json:"orderId"`
	PublicID        string    `json:"publicId"`    // the order's public ID
	PurchasedAt     time.Time `json:"purchasedAt"` // when the order was paid
	SellerName      string    `json:"sellerName"`
	ProductTitle    string    `json:"productTitle"`
	ProductImageURL string    `json:"productImageUrl"`
	PriceCents      int       `json:"priceCents"` // the price paid for this key
}

// KeyListResponse is returned by GET /api/v1/keys. The paging fields are the
// same as in OrderListResponse.
type KeyListResponse struct {
	Items      []KeyListItemResponse `json:"items"`
	Page       int                   `json:"page"`
	Limit      int                   `json:"limit"`
	Total      int                   `json:"total"`
	TotalPages int                   `json:"totalPages"`
}

// NewKeyListResponse wraps one page of keys with the paging info the client
// needs to show page numbers.
func NewKeyListResponse(items []KeyListItemResponse, total int, params requests.KeyListParams) KeyListResponse {
	return KeyListResponse{
		Items:      items,
		Page:       params.Page,
		Limit:      params.Limit,
		Total:      total,
		TotalPages: totalPages(total, params.Limit),
	}
}
