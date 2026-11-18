package handlers

import (
	"fmt"
	"net/http"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/responses"
)

// homeBlockProductLimit is how many product cards each home page block shows.
const homeBlockProductLimit = 12

// homeBlock defines one block of the home page: its title and the product
// types it shows.
type homeBlock struct {
	title string
	types []models.ProductType
}

// homeBlocks are the home page blocks, in the order they are shown.
// Adding a block is one more line here.
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

// Get handles GET /api/v1/home and returns the home page blocks: the best
// selling products of each kind that can be bought right now. A block without
// any such products is left out. It is public, no sign-in needed.
func (h *HomeHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The blocks are loaded one after another. Each is a small, indexed query,
	// so running them in parallel wouldn't be noticeably faster.
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
