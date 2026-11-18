package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

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

// Get handles GET /api/v1/orders/{id} and returns one of the signed-in user's
// orders with its items. An order that doesn't exist and another user's order
// both give the same 404, so the response never reveals that an ID exists.
func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		httpx.Error(w, r, models.ErrUnauthorized)
		return
	}

	orderID, err := requests.ParseOrderIDParam(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	order, err := h.orders.GetByUser(ctx, userID, orderID)
	if err != nil {
		httpx.Error(w, r, err) // models.ErrNotFound becomes a 404
		return
	}

	httpx.WriteJSON(w, http.StatusOK, order)
}
