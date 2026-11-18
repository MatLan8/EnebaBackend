package handlers

import (
	"context"
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
	pool          *pgxpool.Pool
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

	// One transaction, so a user is never saved without their first refresh token.
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

// Unknown email and wrong password give the same error, so registered emails aren't revealed.
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

// Parallel refreshes from one browser (e.g. two tabs) send the same token; the slower ones arrive
// just after the rotation and must not look like theft.
const refreshReuseGrace = 30 * time.Second

// Refresh rotates the token: the old one is revoked and a new one is set in the cookie.
// Reusing a rotated token means it was probably stolen, so every session of the user is revoked.
// Within refreshReuseGrace of the rotation it only gets an access token, with no new cookie.
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
	concurrent := false

	err = database.WithTx(ctx, h.pool, func(tx pgx.Tx) error {
		refreshTokens := h.refreshTokens.WithTx(tx)

		stored, err := refreshTokens.GetByHashForUpdate(ctx, tokenHash)
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrInvalidToken
		}
		if err != nil {
			return err
		}

		if stored.WasRotatedWithin(refreshReuseGrace, time.Now()) {
			concurrent = true
			user, tokens, err = h.accessTokenForSuccessor(ctx, tx, stored)
			return err
		}

		if stored.ReplacedByTokenHash != nil {
			// Reuse: return nil so the transaction commits the revocation; the 401 is sent after it.
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
		// A late parallel request must not clear the cookie the first one has just set.
		if errors.Is(err, models.ErrInvalidToken) && !concurrent {
			h.clearRefreshCookie(w)
		}
		httpx.Error(w, r, err)
		return
	}

	// The browser already has, or is about to get, the successor's cookie from the first refresh.
	if !concurrent {
		h.setRefreshCookie(w, tokens.RefreshToken.Raw, tokens.RefreshToken.ExpiresAt)
	}
	httpx.WriteJSON(w, http.StatusOK, newAuthResponse(user, tokens))
}

// Only while the successor token is still active: after a logout or a further rotation the
// late request gets a plain 401, without revoking every session.
func (h *AuthHandler) accessTokenForSuccessor(ctx context.Context, tx pgx.Tx, stored *models.RefreshToken) (*models.User, *services.IssuedTokens, error) {
	successor, err := h.refreshTokens.WithTx(tx).GetByHashForUpdate(ctx, *stored.ReplacedByTokenHash)
	if errors.Is(err, models.ErrNotFound) {
		return nil, nil, models.ErrInvalidToken
	}
	if err != nil {
		return nil, nil, err
	}
	if !successor.IsActive(time.Now()) {
		return nil, nil, models.ErrInvalidToken
	}

	user, err := h.users.WithTx(tx).GetByID(ctx, stored.UserID)
	if errors.Is(err, models.ErrNotFound) {
		return nil, nil, models.ErrInvalidToken
	}
	if err != nil {
		return nil, nil, err
	}

	accessToken, expiresAt, err := h.tokens.NewAccessToken(user)
	if err != nil {
		return nil, nil, err
	}

	return user, &services.IssuedTokens{AccessToken: accessToken, AccessTokenExpiresAt: expiresAt}, nil
}

// Logout always returns 204, even without a cookie, so logging out twice is harmless.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
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

func newAuthResponse(user *models.User, tokens *services.IssuedTokens) responses.AuthResponse {
	return responses.AuthResponse{
		AccessToken: tokens.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   tokens.AccessTokenExpiresAt,
		User:        responses.NewUserResponse(user),
	}
}
