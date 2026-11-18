package responses

import "eneba-backend/internal/models"

type HomeResponse struct {
	Blocks []HomeBlockResponse `json:"blocks"`
}

// ProductTypes tells the frontend which store filter its "Show all" link should open.
type HomeBlockResponse struct {
	Title        string                    `json:"title"`
	ProductTypes []models.ProductType      `json:"productTypes"`
	Products     []ProductListItemResponse `json:"products"`
}

// A block with nothing in stock would be an empty row on the page, so it is left out.
func NewHomeResponse(blocks []HomeBlockResponse) HomeResponse {
	shown := make([]HomeBlockResponse, 0, len(blocks))
	for _, block := range blocks {
		if len(block.Products) > 0 {
			shown = append(shown, block)
		}
	}

	return HomeResponse{Blocks: shown}
}
