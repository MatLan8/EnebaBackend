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
	Config        *config.Config
	Health        *handlers.HealthHandler
	Auth          *handlers.AuthHandler
	Users         *handlers.UserHandler
	Products      *handlers.ProductHandler
	Cart          *handlers.CartHandler
	Checkout      *handlers.CheckoutHandler
	Orders        *handlers.OrderHandler
	Keys          *handlers.KeyHandler
	Home          *handlers.HomeHandler
	StripeWebhook *handlers.StripeWebhookHandler
	Tokens        *services.TokenService // used by the auth middleware
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

		// The home page blocks. Public like the store.
		r.Get("/home", d.Home.Get)

		// Public store routes. chi matches fixed segments like /filters and /suggestions before
		// URL parameters, so /{slug} never swallows them.
		r.Route("/products", func(r chi.Router) {
			r.Get("/", d.Products.List)
			r.Get("/filters", d.Products.Filters)
			r.Get("/suggestions", d.Products.Suggestions)
			r.Get("/{slug}", d.Products.Get)
		})

		// The browser's cart, public like the store. It is found by the cart_token
		// cookie, not by the signed-in user. Items are addressed by offer id, since an
		// offer is in a cart at most once.
		r.Route("/cart", func(r chi.Router) {
			r.Get("/", d.Cart.Get)
			r.Delete("/", d.Cart.Clear)
			r.Post("/items", d.Cart.AddItem)
			r.Patch("/items/{offerId}", d.Cart.UpdateItem)
			r.Delete("/items/{offerId}", d.Cart.RemoveItem)
		})

		// Stripe calls this, not the frontend. It is public; every request is checked
		// with its Stripe-Signature header instead, and the handler reads the raw body itself.
		r.Post("/webhooks/stripe", d.StripeWebhook.Handle)

		// Every route in this group requires a valid access token.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(d.Tokens))

			r.Get("/users/me", d.Users.Me)

			// Turns the cart from the cart_token cookie into an order and a Stripe payment page.
			r.Post("/checkout", d.Checkout.Create)

			// The signed-in user's own orders, newest first.
			r.Get("/orders", d.Orders.List)
			r.Get("/orders/{id}", d.Orders.Get)
			// The signed-in user's key library: keys from paid orders, without their codes.
			r.Get("/keys", d.Keys.List)

			r.Get("/orders/{id}/keys", d.Orders.Keys) // paid orders only; the response holds plaintext codes
		})
	})

	return r
}
