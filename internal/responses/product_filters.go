package responses

type FilterOption struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Value is the slug used in the URL, Label the name to display.
type LabeledFilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type ProductFiltersResponse struct {
	Types     []FilterOption        `json:"types"`
	Platforms []LabeledFilterOption `json:"platforms"` // no icon here, the filter list shows names only
	Regions   []FilterOption        `json:"regions"`
	Genres    []LabeledFilterOption `json:"genres"`
}
