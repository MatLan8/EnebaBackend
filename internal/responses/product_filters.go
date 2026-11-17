package responses

import "eneba-backend/internal/models"

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

func NewProductFiltersResponse(f *models.ProductFilters) ProductFiltersResponse {
	// make(..., 0, n) instead of a nil slice, so an empty list is [] in the JSON, not null.
	genres := make([]GenreFilterOption, 0, len(f.Genres))
	for _, g := range f.Genres {
		genres = append(genres, GenreFilterOption{Value: g.Slug, Label: g.Name, Count: g.Count})
	}

	return ProductFiltersResponse{
		Types:     newFilterOptions(f.Types),
		Platforms: newFilterOptions(f.Platforms),
		Regions:   newFilterOptions(f.Regions),
		Genres:    genres,
		Price: PriceRange{
			MinCents: 0, // the slider always starts at 0
			MaxCents: roundUpToWholeEuro(f.MaxPriceCents),
		},
	}
}

func newFilterOptions(counts []models.FilterCount) []FilterOption {
	options := make([]FilterOption, 0, len(counts))
	for _, c := range counts {
		options = append(options, FilterOption{Value: c.Value, Count: c.Count})
	}

	return options
}

// roundUpToWholeEuro rounds cents up to the next multiple of 100, e.g. 8999 -> 9000.
// Exact multiples stay the same. Integer division drops the remainder,
// so adding 99 first pushes any partial euro up to the next one.
func roundUpToWholeEuro(cents int) int {
	return (cents + 99) / 100 * 100
}
