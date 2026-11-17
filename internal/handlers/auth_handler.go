package handlers

import (
	"errors"
	"net/http"
	"time"

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

// Login handles POST /api/v1/auth/login.
// Unknown email and wrong password return the same invalid_credentials error,
// so the response does not reveal which emails are registered.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req requests.LoginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	req.Normalize()
	if err := requests.Validate(req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	user, err := h.users.GetByEmail(ctx, req.Email)
	if errors.Is(err, models.ErrNotFound) {
		httpx.Error(w, r, models.ErrInvalidCredentials)
		return
	}
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if !services.CheckPassword(user.PasswordHash, req.Password) {
		httpx.Error(w, r, models.ErrInvalidCredentials)
		return
	}

	tokens, err := h.tokens.IssueTokens(ctx, h.refreshTokens, user)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.setRefreshCookie(w, tokens.RefreshToken.Raw, tokens.RefreshToken.ExpiresAt)
	httpx.WriteJSON(w, http.StatusOK, newAuthResponse(user, tokens))
}

// Refresh handles POST /api/v1/auth/refresh.
// It needs only the refresh_token cookie. The token is rotated: the old one is
// revoked, a new one is set in the cookie, and a new access token is returned.
// If an already rotated token is used again, it was most likely stolen, so
// every session of that user is revoked.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		httpx.Error(w, r, models.ErrInvalidToken)
		return
	}
	tokenHash := services.HashRefreshToken(cookie.Value)

	var user *models.User
	var tokens *services.IssuedTokens
	reused := false

	err = database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		refreshTokens := h.refreshTokens.WithTx(tx)

		stored, err := refreshTokens.GetByHashForUpdate(ctx, tokenHash)
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrInvalidToken
		}
		if err != nil {
			return err
		}

		if stored.ReplacedByTokenHash != nil {
			// Reuse detected. Return nil (not an error) so the transaction
			// commits the revocation; the 401 is sent after it.
			reused = true
			return refreshTokens.RevokeAllForUser(ctx, stored.UserID)
		}
		if !stored.IsActive(time.Now()) {
			return models.ErrInvalidToken // logged out or expired
		}

		user, err = h.users.WithTx(tx).GetByID(ctx, stored.UserID)
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrInvalidToken
		}
		if err != nil {
			return err
		}

		tokens, err = h.tokens.IssueTokens(ctx, refreshTokens, user)
		if err != nil {
			return err
		}

		return refreshTokens.MarkReplaced(ctx, stored.ID, tokens.RefreshToken.Hash)
	})
	if err == nil && reused {
		err = models.ErrInvalidToken
	}
	if err != nil {
		if errors.Is(err, models.ErrInvalidToken) {
			h.clearRefreshCookie(w) // the cookie is useless now, so remove it
		}
		httpx.Error(w, r, err)
		return
	}

	h.setRefreshCookie(w, tokens.RefreshToken.Raw, tokens.RefreshToken.ExpiresAt)
	httpx.WriteJSON(w, http.StatusOK, newAuthResponse(user, tokens))
}

// Logout handles POST /api/v1/auth/logout.
// It only needs the refresh_token cookie: the token is revoked and the cookie cleared.
// It always returns 204, even if there was no cookie, so logging out twice is harmless.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		// No cookie means there is nothing to revoke.
		h.clearRefreshCookie(w)
		httpx.NoContent(w)
		return
	}

	tokenHash := services.HashRefreshToken(cookie.Value)
	if err := h.refreshTokens.Revoke(ctx, tokenHash); err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.clearRefreshCookie(w)
	httpx.NoContent(w)
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
