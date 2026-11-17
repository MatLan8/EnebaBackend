package models

// FilterCount is one value of a store filter, e.g. platform "steam",
// and how many products have it.
type FilterCount struct {
	Value string
	Count int
}

// GenreFilterCount is a genre and how many products are in it.
type GenreFilterCount struct {
	Slug  string
	Name  string
	Count int
}

// ProductFilters is everything the store's filter sidebar needs.
// It has no json tags because handlers return responses.ProductFiltersResponse instead.
type ProductFilters struct {
	Types         []FilterCount
	Platforms     []FilterCount
	Regions       []FilterCount
	Genres        []GenreFilterCount
	MaxPriceCents int // the most expensive active offer, 0 if there are none
}
