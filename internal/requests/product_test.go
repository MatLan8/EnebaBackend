package requests

import (
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/models"
)

func TestParseProductListQueryValues(t *testing.T) {
	query, _ := url.ParseQuery(
		"q=+eldn+ring+&platform=steam&platform=psn&type=dlc&region=europe&genre=rpg&genre=action" +
			"&minPrice=1000&maxPrice=5000&sort=price_desc&page=3&limit=10",
	)

	params, err := ParseProductListQuery(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Search != "eldn ring" {
		t.Errorf("search = %q; want trimmed %q", params.Search, "eldn ring")
	}
	if !slices.Equal(params.Platforms, []string{"steam", "psn"}) {
		t.Errorf("platforms = %v", params.Platforms)
	}
	if !slices.Equal(params.Types, []models.ProductType{models.ProductTypeDLC}) {
		t.Errorf("types = %v", params.Types)
	}
	if !slices.Equal(params.Regions, []models.Region{models.RegionEurope}) {
		t.Errorf("regions = %v", params.Regions)
	}
	if !slices.Equal(params.Genres, []string{"rpg", "action"}) {
		t.Errorf("genres = %v", params.Genres)
	}
	if params.MinPriceCents == nil || *params.MinPriceCents != 1000 || params.MaxPriceCents == nil || *params.MaxPriceCents != 5000 {
		t.Errorf("price range = %v, %v; want 1000, 5000", params.MinPriceCents, params.MaxPriceCents)
	}
	if params.Sort != ProductSortPriceDesc || params.Page != 3 || params.Limit != 10 {
		t.Errorf("sort, page, limit = %q, %d, %d", params.Sort, params.Page, params.Limit)
	}
	if params.Offset() != 20 {
		t.Errorf("offset = %d; want 20", params.Offset())
	}
}

func TestParseProductListQueryInvalid(t *testing.T) {
	tests := []struct {
		name  string
		query string
		field string // the parameter that must be reported
	}{
		{"unknown type", "type=movie", "type"},
		{"unknown region", "region=mars", "region"},
		{"negative min price", "minPrice=-1", "minPrice"},
		{"max price not a number", "maxPrice=cheap", "maxPrice"},
		{"min above max", "minPrice=5000&maxPrice=1000", "maxPrice"},
		{"unknown sort", "sort=cheapest", "sort"},
		{"removed relevance sort", "sort=relevance", "sort"},
		{"page zero", "page=0", "page"},
		{"limit zero", "limit=0", "limit"},
		{"limit too big", "limit=61", "limit"},
		{"search too long", "q=" + strings.Repeat("a", 101), "q"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, _ := url.ParseQuery(tt.query)

			_, err := ParseProductListQuery(query)

			var apiErr *httpx.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v; want *httpx.APIError", err)
			}
			if apiErr.Code != "validation_failed" {
				t.Errorf("code = %q; want validation_failed", apiErr.Code)
			}
			if _, ok := apiErr.Details[tt.field]; !ok {
				t.Errorf("details = %v; want an entry for %q", apiErr.Details, tt.field)
			}
		})
	}
}
