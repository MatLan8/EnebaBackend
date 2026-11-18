package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/requests"
	"eneba-backend/internal/responses"
)

type ProductHandler struct {
	products *repository.ProductRepository
}

func NewProductHandler(products *repository.ProductRepository) *ProductHandler {
	return &ProductHandler{products: products}
}

// Filters handles GET /api/v1/products/filters and returns the values the
// store can be filtered by, with product counts. It is public, no sign-in needed.
func (h *ProductHandler) Filters(w http.ResponseWriter, r *http.Request) {
	filters, err := h.products.GetFilters(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, filters)
}

// List handles GET /api/v1/products and returns one page of the store's
// products. Query parameters filter, search, sort and page the list; see
// requests.ParseProductListQuery. It is public, no sign-in needed.
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	params, err := requests.ParseProductListQuery(r.URL.Query())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items, total, err := h.products.List(r.Context(), params)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, responses.NewProductListResponse(items, total, params))
}

// Get handles GET /api/v1/products/{slug} and returns the product page: the
// product, its genres and the offers that can be bought, cheapest first.
// An unknown slug is a 404. It is public, no sign-in needed.
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	product, err := h.products.GetBySlug(r.Context(), slug)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, product)
}

// Suggestions handles GET /api/v1/products/suggestions?q=... and returns up to
// 6 products for the search box dropdown, plus how many products match in total.
// The frontend calls it while the user types, so a q shorter than 3 characters
// returns no suggestions straight away, without querying the database.
// It is public, no sign-in needed.
func (h *ProductHandler) Suggestions(w http.ResponseWriter, r *http.Request) {
	params, err := requests.ParseSuggestionQuery(r.URL.Query())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if params.IsTooShort() {
		httpx.WriteJSON(w, http.StatusOK, responses.NewProductSuggestionsResponse(nil, 0))
		return
	}

	suggestions, err := h.products.Suggest(r.Context(), params.Search)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, suggestions)
}
