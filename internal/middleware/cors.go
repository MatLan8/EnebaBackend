package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS allows the React frontend (on another domain) to call the API.
// AllowCredentials is required so the browser sends the refresh token cookie;
// with credentials the allowed origins must be listed explicitly, never "*".
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
