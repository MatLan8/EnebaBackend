package responses

import (
	"time"

	"github.com/google/uuid"

	"eneba-backend/internal/models"
	"eneba-backend/internal/requests"
)

// ProductTitle is the first item's title; null for a cancelled order, because cancelling deletes
// the items to free their keys.
type OrderListItemResponse struct {
	ID           uuid.UUID          `json:"id"`
	PublicID     string             `json:"publicId"`
	CreatedAt    time.Time          `json:"createdAt"`
	Status       models.OrderStatus `json:"status"`
	ProductTitle *string            `json:"productTitle"`
	TotalCents   int                `json:"totalCents"`
	Currency     string             `json:"currency"`
}

type OrderListResponse struct {
	Items      []OrderListItemResponse `json:"items"`
	Page       int                     `json:"page"`
	Limit      int                     `json:"limit"`
	Total      int                     `json:"total"`
	TotalPages int                     `json:"totalPages"`
}

func NewOrderListResponse(items []OrderListItemResponse, total int, params requests.OrderListParams) OrderListResponse {
	return OrderListResponse{
		Items:      items,
		Page:       params.Page,
		Limit:      params.Limit,
		Total:      total,
		TotalPages: totalPages(total, params.Limit),
	}
}
