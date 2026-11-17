package handlers

import (
	"net/http"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/repository"
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

	httpx.WriteJSON(w, http.StatusOK, responses.NewProductFiltersResponse(filters))
}
