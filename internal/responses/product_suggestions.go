package responses

// Items holds at most 6 products; Total counts every match, for the "show all N results" link.
type ProductSuggestionsResponse struct {
	Items []ProductSuggestionResponse `json:"items"`
	Total int                         `json:"total"`
}

type ProductSuggestionResponse struct {
	Slug           string        `json:"slug"`
	Title          string        `json:"title"`
	ImageURL       string        `json:"imageUrl"`
	IsDLC          bool          `json:"isDlc"`          // shows the DLC badge
	BasePriceCents int           `json:"basePriceCents"` // official price, shown crossed out
	Price          PriceResponse `json:"price"`          // the cheapest buyable offer
}

func NewProductSuggestionsResponse(items []ProductSuggestionResponse, total int) ProductSuggestionsResponse {
	if items == nil {
		items = []ProductSuggestionResponse{}
	}

	return ProductSuggestionsResponse{
		Items: items,
		Total: total,
	}
}
