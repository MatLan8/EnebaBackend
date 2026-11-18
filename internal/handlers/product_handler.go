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

func (h *ProductHandler) Filters(w http.ResponseWriter, r *http.Request) {
	filters, err := h.products.GetFilters(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, filters)
}

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

func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	product, err := h.products.GetBySlug(r.Context(), slug)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, product)
}

// A q shorter than 3 characters returns no suggestions without querying the database.
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
