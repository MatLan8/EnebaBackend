package handlers

import (
	"fmt"
	"net/http"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/responses"
)

const homeBlockProductLimit = 12

type homeBlock struct {
	title string
	types []models.ProductType
}

var homeBlocks = []homeBlock{
	{title: "Most popular games", types: []models.ProductType{models.ProductTypeGame}},
	{title: "Most popular subscriptions", types: []models.ProductType{models.ProductTypeSubscription}},
	{title: "Most popular add-ons", types: []models.ProductType{models.ProductTypeGiftCard, models.ProductTypeTopUp}},
}

type HomeHandler struct {
	products *repository.ProductRepository
}

func NewHomeHandler(products *repository.ProductRepository) *HomeHandler {
	return &HomeHandler{products: products}
}

// Blocks without any buyable products are left out.
func (h *HomeHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Sequential on purpose: each block is a small indexed query, so parallel wouldn't be noticeably faster.
	blocks := make([]responses.HomeBlockResponse, 0, len(homeBlocks))
	for _, block := range homeBlocks {
		products, err := h.products.ListPopular(ctx, block.types, homeBlockProductLimit)
		if err != nil {
			httpx.Error(w, r, fmt.Errorf("home block %q: %w", block.title, err))
			return
		}

		blocks = append(blocks, responses.HomeBlockResponse{
			Title:        block.title,
			ProductTypes: block.types,
			Products:     products,
		})
	}

	httpx.WriteJSON(w, http.StatusOK, responses.NewHomeResponse(blocks))
}
