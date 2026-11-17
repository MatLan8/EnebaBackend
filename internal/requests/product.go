package requests

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
)

const (
	defaultProductPageLimit = 24
	maxProductPageLimit     = 60
)

// ProductSort is how the store's product list is ordered.
type ProductSort string

const (
	ProductSortPopular   ProductSort = "popular"    // most sold first, the default
	ProductSortPriceAsc  ProductSort = "price_asc"  // cheapest first
	ProductSortPriceDesc ProductSort = "price_desc" // most expensive first
	ProductSortNewest    ProductSort = "newest"     // latest release date first
	ProductSortOldest    ProductSort = "oldest"     // earliest release date first
	ProductSortNameAsc   ProductSort = "name_asc"   // title A to Z
	ProductSortNameDesc  ProductSort = "name_desc"  // title Z to A
)

// ProductSorts lists every valid ProductSort.
var ProductSorts = []ProductSort{
	ProductSortPopular,
	ProductSortPriceAsc,
	ProductSortPriceDesc,
	ProductSortNewest,
	ProductSortOldest,
	ProductSortNameAsc,
	ProductSortNameDesc,
}

// ProductListParams is what the store's product list is filtered, sorted and
// paged by, read from the query string of GET /api/v1/products.
// Empty slices and nil pointers mean "no filter".
type ProductListParams struct {
	Search        string // matched against the title, typo-tolerant
	Types         []models.ProductType
	Platforms     []models.Platform
	Regions       []models.Region
	Genres        []string // genre slugs
	MinPriceCents *int     // compared with the cheapest active offer
	MaxPriceCents *int
	Sort          ProductSort
	Page          int // starts at 1
	Limit         int // items per page
}

// Offset is the number of rows to skip to reach Page.
func (p ProductListParams) Offset() int {
	return (p.Page - 1) * p.Limit
}

// ParseProductListQuery reads the query string of GET /api/v1/products.
// Every parameter is optional. type, platform, region and genre can be
// repeated, e.g. ?platform=steam&platform=psn. Every problem found is
// returned at once as a 400 validation error, keyed by parameter name.
func ParseProductListQuery(query url.Values) (ProductListParams, error) {
	details := map[string]string{}

	params := ProductListParams{
		Search: strings.TrimSpace(query.Get("q")),
		Genres: query["genre"], // unknown slugs simply match nothing
		Page:   1,
		Limit:  defaultProductPageLimit,
	}

	var ok bool
	if params.Types, ok = parseEnumValues(query["type"], models.ProductTypes); !ok {
		details["type"] = mustBeOneOf(models.ProductTypes)
	}
	if params.Platforms, ok = parseEnumValues(query["platform"], models.Platforms); !ok {
		details["platform"] = mustBeOneOf(models.Platforms)
	}
	if params.Regions, ok = parseEnumValues(query["region"], models.Regions); !ok {
		details["region"] = mustBeOneOf(models.Regions)
	}

	if raw := query.Get("minPrice"); raw != "" {
		cents, err := strconv.Atoi(raw)
		if err != nil || cents < 0 {
			details["minPrice"] = "must be a whole number of cents, 0 or more"
		} else {
			params.MinPriceCents = &cents
		}
	}
	if raw := query.Get("maxPrice"); raw != "" {
		cents, err := strconv.Atoi(raw)
		if err != nil || cents < 0 {
			details["maxPrice"] = "must be a whole number of cents, 0 or more"
		} else {
			params.MaxPriceCents = &cents
		}
	}
	if params.MinPriceCents != nil && params.MaxPriceCents != nil && *params.MinPriceCents > *params.MaxPriceCents {
		details["maxPrice"] = "must be greater than or equal to minPrice"
	}

	if raw := query.Get("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			details["page"] = "must be a whole number, 1 or more"
		} else {
			params.Page = page
		}
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maxProductPageLimit {
			details["limit"] = fmt.Sprintf("must be a whole number from 1 to %d", maxProductPageLimit)
		} else {
			params.Limit = limit
		}
	}

	params.Sort = ProductSortPopular
	if raw := query.Get("sort"); raw != "" {
		sort := ProductSort(raw)
		if !slices.Contains(ProductSorts, sort) {
			details["sort"] = mustBeOneOf(ProductSorts)
		} else {
			params.Sort = sort
		}
	}

	if len(details) > 0 {
		return ProductListParams{}, httpx.ValidationFailed(details)
	}
	return params, nil
}

// parseEnumValues converts raw strings into one of the "enum" types, e.g. Platform.
// ok is false if any value isn't in allowed.
// [T ~string] makes it generic: T can be any type whose underlying type is string,
// so the same function works for ProductType, Platform and Region.
func parseEnumValues[T ~string](raw []string, allowed []T) (values []T, ok bool) {
	for _, s := range raw {
		value := T(s)
		if !slices.Contains(allowed, value) {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
}

// mustBeOneOf builds a message like "must be one of: steam, psn".
func mustBeOneOf[T ~string](allowed []T) string {
	names := make([]string, len(allowed))
	for i, a := range allowed {
		names[i] = string(a)
	}
	return "must be one of: " + strings.Join(names, ", ")
}
