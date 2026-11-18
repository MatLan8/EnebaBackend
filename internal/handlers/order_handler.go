package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/middleware"
	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/requests"
	"eneba-backend/internal/responses"
	"eneba-backend/internal/services"
)

type OrderHandler struct {
	orders    *repository.OrderRepository
	keyCipher *services.KeyCipher
}

func NewOrderHandler(orders *repository.OrderRepository, keyCipher *services.KeyCipher) *OrderHandler {
	return &OrderHandler{orders: orders, keyCipher: keyCipher}
}

// Every status is listed (pending, paid and cancelled), like on Eneba.
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

// Another user's order gives the same 404 as a missing one, so order IDs are never revealed.
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
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, order)
}

// Keys returns plaintext codes, for paid orders only. An unpaid order is a 409 order_not_paid, so
// the frontend can say the payment isn't complete. The frontend's ?key= preselection is ignored.
func (h *OrderHandler) Keys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The response contains secrets, so browsers and proxies must not store it.
	w.Header().Set("Cache-Control", "no-store")

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

	order, err := h.orders.GetByIDForUser(ctx, userID, orderID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if order.Status != models.OrderStatusPaid {
		httpx.Error(w, r, httpx.OrderNotPaid(string(order.Status)))
		return
	}

	keyRows, err := h.orders.ListKeys(ctx, order.ID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	response, err := newOrderKeysResponse(order, keyRows, h.keyCipher)
	if err != nil {
		// httpx.Error logs the error (it names the key ID, never the code) and answers 500.
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func newOrderKeysResponse(order models.Order, keyRows []repository.OrderKeyRow, keyCipher *services.KeyCipher) (responses.OrderKeysResponse, error) {
	// make(..., 0, n) so no keys is [] in the JSON, not null.
	keys := make([]responses.OrderKeyResponse, 0, len(keyRows))

	for _, row := range keyRows {
		code, err := keyCipher.Decrypt(row.CodeEncrypted)
		if err != nil {
			return responses.OrderKeysResponse{}, fmt.Errorf("key %s of order %s: %w", row.KeyID, order.ID, err)
		}

		keys = append(keys, responses.OrderKeyResponse{
			ID:   row.KeyID,
			Code: code,
			Product: responses.OrderKeyProductResponse{
				Title:    row.Title,
				ImageURL: row.ImageURL,
				Type:     row.Type,
				Platform: row.Platform,
				Region:   row.Region,
			},
			Seller: responses.OrderKeySellerResponse{
				Name:    row.SellerName,
				LogoURL: row.SellerLogoURL,
				Rating:  row.SellerRating,
			},
		})
	}

	return responses.OrderKeysResponse{
		OrderID: order.ID,
		Keys:    keys,
	}, nil
}
