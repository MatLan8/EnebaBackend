package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"eneba-backend/internal/database"
	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/requests"
	"eneba-backend/internal/responses"
	"eneba-backend/internal/services"
)

type AuthHandler struct {
	pool          *pgxpool.Pool // needed to start transactions
	users         *repository.UserRepository
	refreshTokens *repository.RefreshTokenRepository
	tokens        *services.TokenService
	secureCookies bool // true in production (HTTPS)
}

func NewAuthHandler(
	pool *pgxpool.Pool,
	users *repository.UserRepository,
	refreshTokens *repository.RefreshTokenRepository,
	tokens *services.TokenService,
	secureCookies bool,
) *AuthHandler {
	return &AuthHandler{
		pool:          pool,
		users:         users,
		refreshTokens: refreshTokens,
		tokens:        tokens,
		secureCookies: secureCookies,
	}
}

// Register handles POST /api/v1/auth/register.
// It creates the user and signs them in straight away.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req requests.RegisterRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	req.Normalize()
	if err := requests.Validate(req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	passwordHash, err := services.HashPassword(req.Password)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	var user *models.User
	var tokens *services.IssuedTokens

	// The user and their first refresh token are saved in one transaction,
	// so if either insert fails, neither is kept.
	err = database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		var err error

		user, err = h.users.WithTx(tx).Create(ctx, req.Name, req.Email, passwordHash)
		if err != nil {
			return err // models.ErrEmailTaken becomes 409 in httpx.Error
		}

		tokens, err = h.tokens.IssueTokens(ctx, h.refreshTokens.WithTx(tx), user)
		return err
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.setRefreshCookie(w, tokens.RefreshToken.Raw, tokens.RefreshToken.ExpiresAt)
	httpx.WriteJSON(w, http.StatusCreated, newAuthResponse(user, tokens))
}

// newAuthResponse builds the JSON body returned by register, login and refresh.
func newAuthResponse(user *models.User, tokens *services.IssuedTokens) responses.AuthResponse {
	return responses.AuthResponse{
		AccessToken: tokens.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   tokens.AccessTokenExpiresAt,
		User:        responses.NewUserResponse(user),
	}
}
