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

// An unexported key type, so no other package can read or overwrite the user ID by accident.
type userIDKey struct{}

func Authenticate(tokens *services.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				httpx.Error(w, r, models.ErrUnauthorized)
				return
			}

			scheme, token, found := strings.Cut(header, " ")
			if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
				httpx.Error(w, r, models.ErrUnauthorized)
				return
			}

			claims, err := tokens.ParseAccessToken(token)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}

			userID, err := claims.UserID()
			if err != nil {
				httpx.Error(w, r, err)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ok is false if the route isn't behind Authenticate.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return userID, ok
}
