package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// AllowCredentials is needed for the refresh token cookie, and with credentials the origins
// must be listed explicitly, never "*".
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300, // browsers may cache the preflight response for 5 minutes
	})
}
