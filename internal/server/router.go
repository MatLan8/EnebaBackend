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
	"eneba-backend/internal/services"
)

// Deps holds everything the router needs. main.go builds it;
// new handlers are added here as the API grows.
type Deps struct {
	Config   *config.Config
	Health   *handlers.HealthHandler
	Auth     *handlers.AuthHandler
	Users    *handlers.UserHandler
	Products *handlers.ProductHandler
	Tokens   *services.TokenService // used by the auth middleware
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
			r.Post("/login", d.Auth.Login)
			r.Post("/refresh", d.Auth.Refresh)
			r.Post("/logout", d.Auth.Logout)
		})

		// Public store routes. chi matches fixed segments like /filters before
		// URL parameters, so /{slug} never swallows them.
		r.Route("/products", func(r chi.Router) {
			r.Get("/", d.Products.List)
			r.Get("/filters", d.Products.Filters)
			r.Get("/{slug}", d.Products.Get)
		})

		// Every route in this group requires a valid access token.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(d.Tokens))

			r.Get("/users/me", d.Users.Me)
		})
	})

	return r
}
