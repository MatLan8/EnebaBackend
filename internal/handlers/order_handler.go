package handlers

import (
	"net/http"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/middleware"
	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/requests"
	"eneba-backend/internal/responses"
)

type OrderHandler struct {
	orders *repository.OrderRepository
}

func NewOrderHandler(orders *repository.OrderRepository) *OrderHandler {
	return &OrderHandler{orders: orders}
}

// List handles GET /api/v1/orders and returns one page of the signed-in user's
// orders, newest first, in every status (pending, paid and cancelled).
// ?publicId= finds orders by part of their public ID, e.g. "k3x" finds "o-k3x9q2a".
// It runs behind middleware.Authenticate, so only the user's own orders are listed.
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		httpx.Error(w, r, models.ErrUnauthorized)
		return
	}

	params, err := requests.ParseOrderListQuery(r.URL.Query())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, total, err := h.orders.ListByUser(ctx, userID, params)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, responses.NewOrderListResponse(items, total, params))
}
