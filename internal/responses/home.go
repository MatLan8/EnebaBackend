package responses

import "eneba-backend/internal/models"

// HomeResponse is returned by GET /api/v1/home: the product blocks of the home
// page, in the order they are shown.
type HomeResponse struct {
	Blocks []HomeBlockResponse `json:"blocks"`
}

// HomeBlockResponse is one row of product cards on the home page, e.g. "Most
// popular games". ProductTypes tells the frontend which store filter its
// "Show all" link should open.
type HomeBlockResponse struct {
	Title        string                    `json:"title"`
	ProductTypes []models.ProductType      `json:"productTypes"`
	Products     []ProductListItemResponse `json:"products"`
}

// NewHomeResponse keeps the blocks that have products, in their order. A block
// with nothing in stock would be an empty row on the page, so it is left out.
// Blocks is always a slice, so no blocks at all gives [] in the JSON.
func NewHomeResponse(blocks []HomeBlockResponse) HomeResponse {
	shown := make([]HomeBlockResponse, 0, len(blocks))
	for _, block := range blocks {
		if len(block.Products) > 0 {
			shown = append(shown, block)
		}
	}

	return HomeResponse{Blocks: shown}
}
