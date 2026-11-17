package handlers

import (
	"errors"
	"net/http"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/middleware"
	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/responses"
)

type UserHandler struct {
	users *repository.UserRepository
}

func NewUserHandler(users *repository.UserRepository) *UserHandler {
	return &UserHandler{users: users}
}

// Me handles GET /api/v1/users/me and returns the signed-in user.
// It runs behind middleware.Authenticate, which has already checked the access token.
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		httpx.Error(w, r, models.ErrUnauthorized)
		return
	}

	user, err := h.users.GetByID(ctx, userID)
	if errors.Is(err, models.ErrNotFound) {
		// The token is valid but the user was deleted since it was issued.
		httpx.Error(w, r, models.ErrInvalidToken)
		return
	}
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, responses.NewUserResponse(user))
}
