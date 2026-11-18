package responses

// FilterOption is one value of a filter with the number of products that have it.
type FilterOption struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// GenreFilterOption is a genre filter value. Value is the slug used in the URL,
// Label is the name to display.
type GenreFilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// PriceRange is the range for the price slider, in cents.
type PriceRange struct {
	MinCents int `json:"minCents"`
	MaxCents int `json:"maxCents"`
}

// ProductFiltersResponse is returned by GET /api/v1/products/filters.
type ProductFiltersResponse struct {
	Types     []FilterOption      `json:"types"`
	Platforms []FilterOption      `json:"platforms"`
	Regions   []FilterOption      `json:"regions"`
	Genres    []GenreFilterOption `json:"genres"`
	Price     PriceRange          `json:"price"`
}

// NewPriceRange builds the price slider range. It always starts at 0 and ends
// at the most expensive buyable offer, rounded up to a whole euro.
func NewPriceRange(maxPriceCents int) PriceRange {
	return PriceRange{
		MinCents: 0,
		MaxCents: roundUpToWholeEuro(maxPriceCents),
	}
}

// roundUpToWholeEuro rounds cents up to the next multiple of 100, e.g. 8999 -> 9000.
// Exact multiples stay the same. Integer division drops the remainder,
// so adding 99 first pushes any partial euro up to the next one.
func roundUpToWholeEuro(cents int) int {
	return (cents + 99) / 100 * 100
}
