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

type KeyHandler struct {
	keys *repository.KeyRepository
}

func NewKeyHandler(keys *repository.KeyRepository) *KeyHandler {
	return &KeyHandler{keys: keys}
}

// List handles GET /api/v1/keys and returns one page of the signed-in user's
// key library: every key from their paid orders, newest purchase first.
// ?q= searches product titles and order public IDs. The codes are not included;
// the frontend gets them from GET /api/v1/orders/{id}/keys.
func (h *KeyHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		httpx.Error(w, r, models.ErrUnauthorized)
		return
	}

	params, err := requests.ParseKeyListQuery(r.URL.Query())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, total, err := h.keys.List(ctx, userID, params)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, responses.NewKeyListResponse(items, total, params))
}
