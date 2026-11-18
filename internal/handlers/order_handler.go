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
	keyCipher *services.KeyCipher // decrypts the codes for GET /orders/{id}/keys
}

func NewOrderHandler(orders *repository.OrderRepository, keyCipher *services.KeyCipher) *OrderHandler {
	return &OrderHandler{orders: orders, keyCipher: keyCipher}
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

// Keys handles GET /api/v1/orders/{id}/keys and returns the plaintext codes of
// one of the signed-in user's paid orders. Another user's order or an unknown ID
// is a 404. A pending or cancelled order is a 409 order_not_paid, so the
// frontend can say the payment isn't complete yet.
// The page may preselect a key with ?key=; the API ignores it and returns all keys.
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
		httpx.Error(w, r, err) // models.ErrNotFound becomes a 404
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

// newOrderKeysResponse decrypts every key of the order and builds the response.
// If one key can't be decrypted (changed data or a different secret) it returns
// an error that names the key's ID, so it can be found in the logs.
func newOrderKeysResponse(order models.Order, keyRows []repository.OrderKeyRow, keyCipher *services.KeyCipher) (responses.OrderKeysResponse, error) {
	// make(..., 0, n) so an order without keys has [] in the JSON, not null.
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
