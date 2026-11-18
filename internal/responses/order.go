package responses

import (
	"time"

	"github.com/google/uuid"

	"eneba-backend/internal/models"
	"eneba-backend/internal/requests"
)

// OrderListItemResponse is one row of the signed-in user's order history.
// ProductTitle is the title of the order's first item. It is null for a
// cancelled order, because cancelling deletes the order's items to free its keys.
type OrderListItemResponse struct {
	ID           uuid.UUID          `json:"id"`
	PublicID     string             `json:"publicId"`
	CreatedAt    time.Time          `json:"createdAt"`
	Status       models.OrderStatus `json:"status"`
	ProductTitle *string            `json:"productTitle"`
	TotalCents   int                `json:"totalCents"`
	Currency     string             `json:"currency"`
}

// OrderListResponse is returned by GET /api/v1/orders. The paging fields are
// the same as in ProductListResponse.
type OrderListResponse struct {
	Items      []OrderListItemResponse `json:"items"`
	Page       int                     `json:"page"`
	Limit      int                     `json:"limit"`
	Total      int                     `json:"total"`
	TotalPages int                     `json:"totalPages"`
}

// NewOrderListResponse wraps one page of orders with the paging info the
// client needs to show page numbers.
func NewOrderListResponse(items []OrderListItemResponse, total int, params requests.OrderListParams) OrderListResponse {
	return OrderListResponse{
		Items:      items,
		Page:       params.Page,
		Limit:      params.Limit,
		Total:      total,
		TotalPages: totalPages(total, params.Limit),
	}
}
