package responses

type FilterOption struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Value is the slug used in the URL, Label the name to display.
type GenreFilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// In cents, for the price slider.
type PriceRange struct {
	MinCents int `json:"minCents"`
	MaxCents int `json:"maxCents"`
}

type ProductFiltersResponse struct {
	Types     []FilterOption      `json:"types"`
	Platforms []FilterOption      `json:"platforms"`
	Regions   []FilterOption      `json:"regions"`
	Genres    []GenreFilterOption `json:"genres"`
	Price     PriceRange          `json:"price"`
}

// Always starts at 0 and ends at the most expensive buyable offer, rounded up to a whole euro.
func NewPriceRange(maxPriceCents int) PriceRange {
	return PriceRange{
		MinCents: 0,
		MaxCents: roundUpToWholeEuro(maxPriceCents),
	}
}

// Adding 99 before the integer division pushes any partial euro up to the next one.
func roundUpToWholeEuro(cents int) int {
	return (cents + 99) / 100 * 100
}
