package responses

// ProductSuggestionsResponse is returned by GET /api/v1/products/suggestions,
// the dropdown under the search box. Items holds at most 6 products; Total
// counts every match, for the "show all N results" link to the store page.
type ProductSuggestionsResponse struct {
	Items []ProductSuggestionResponse `json:"items"`
	Total int                         `json:"total"`
}

// ProductSuggestionResponse is one row of the search dropdown.
type ProductSuggestionResponse struct {
	Slug           string        `json:"slug"`
	Title          string        `json:"title"`
	ImageURL       string        `json:"imageUrl"`
	IsDLC          bool          `json:"isDlc"`          // shows the DLC badge
	BasePriceCents int           `json:"basePriceCents"` // official price, shown crossed out
	Price          PriceResponse `json:"price"`          // the cheapest buyable offer
}

// NewProductSuggestionsResponse wraps the suggestions with the total number of
// matches. A nil items becomes an empty slice, so the JSON has [] and not null.
func NewProductSuggestionsResponse(items []ProductSuggestionResponse, total int) ProductSuggestionsResponse {
	if items == nil {
		items = []ProductSuggestionResponse{}
	}

	return ProductSuggestionsResponse{
		Items: items,
		Total: total,
	}
}
