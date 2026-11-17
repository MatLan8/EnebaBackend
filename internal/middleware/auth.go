package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
	"eneba-backend/internal/services"
)

// userIDKey is the key the authenticated user's ID is stored under in the request context.
// Using our own unexported type means no other package can read or overwrite it by accident.
type userIDKey struct{}

// Authenticate protects routes with an access token. It expects the header
// "Authorization: Bearer <token>", verifies the token and puts the user ID
// into the request context. Requests without a valid token get a 401.
func Authenticate(tokens *services.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				httpx.Error(w, r, models.ErrUnauthorized)
				return
			}

			// Split "Bearer eyJ..." into the scheme and the token.
			scheme, token, found := strings.Cut(header, " ")
			if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
				httpx.Error(w, r, models.ErrUnauthorized)
				return
			}

			claims, err := tokens.ParseAccessToken(token)
			if err != nil {
				httpx.Error(w, r, err) // models.ErrInvalidToken, 401
				return
			}

			userID, err := claims.UserID()
			if err != nil {
				httpx.Error(w, r, err)
				return
			}

			// Requests are immutable, so WithContext returns a copy carrying the user ID.
			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext returns the ID of the user that Authenticate let through.
// ok is false if the route isn't behind Authenticate.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return userID, ok
}
