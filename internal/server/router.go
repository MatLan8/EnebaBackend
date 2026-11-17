package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"eneba-backend/internal/config"
	"eneba-backend/internal/handlers"
	"eneba-backend/internal/httpx"
	"eneba-backend/internal/middleware"
	"eneba-backend/internal/models"
)

// Deps holds everything the router needs. main.go builds it;
// new handlers are added here as the API grows.
type Deps struct {
	Config *config.Config
	Health *handlers.HealthHandler
	Auth   *handlers.AuthHandler
}

// NewRouter registers middleware and all routes.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// Order matters: each middleware wraps the ones after it.
	r.Use(chimw.RequestID)                       // give every request an ID
	r.Use(middleware.RequestLogger)              // log it (sees the 500 written by Recoverer)
	r.Use(chimw.Recoverer)                       // turn panics into 500 instead of crashing
	r.Use(middleware.CORS(d.Config.CORSOrigins)) // answer browser preflight requests

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, models.ErrNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, &httpx.APIError{
			Status:  http.StatusMethodNotAllowed,
			Code:    "method_not_allowed",
			Message: "method not allowed",
		})
	})

	r.Get("/health", d.Health.Health)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", d.Auth.Register)
		})
	})

	return r
}
