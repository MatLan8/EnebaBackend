package main

import (
	"slices"
	"testing"

	"eneba-backend/internal/models"
)

// TestSeedFile_MatchesSchema checks the real seed file, so a typo in the data
// fails here instead of halfway through a seed run.
func TestSeedFile_MatchesSchema(t *testing.T) {
	data, err := loadSeedFile("../../seed/data/seed.json")
	if err != nil {
		t.Fatalf("load seed file: %v", err)
	}

	genres := make(map[int]bool)
	for _, g := range data.Genres {
		genres[g.ID] = true
	}
	sellers := make(map[int]bool)
	for _, s := range data.Sellers {
		sellers[s.ID] = true
		if s.Rating < 0 || s.Rating > 5 {
			t.Errorf("seller %q: rating %v is outside 0-5", s.Name, s.Rating)
		}
	}

	products := make(map[int]bool)
	for _, p := range data.Products {
		products[p.ID] = true
		if !slices.Contains(models.ProductTypes, models.ProductType(p.Type)) {
			t.Errorf("product %q: unknown type %q", p.Slug, p.Type)
		}
		if !slices.Contains(models.Platforms, models.Platform(p.Platform)) {
			t.Errorf("product %q: unknown platform %q", p.Slug, p.Platform)
		}
		if !slices.Contains(models.Regions, models.Region(p.Region)) {
			t.Errorf("product %q: unknown region %q", p.Slug, p.Region)
		}
		if _, err := parseDate(p.ReleaseDate); err != nil {
			t.Errorf("product %q: %v", p.Slug, err)
		}
		for _, id := range p.GenreIDs {
			if !genres[id] {
				t.Errorf("product %q: unknown genre id %d", p.Slug, id)
			}
		}
	}

	for i, o := range data.Offers {
		if !products[o.ProductID] || !sellers[o.SellerID] {
			t.Errorf("offer %d: unknown product %d or seller %d", i, o.ProductID, o.SellerID)
		}
		if o.PriceCents <= 0 {
			t.Errorf("offer %d: price must be positive", i)
		}
		if o.OriginalPriceCents != nil && *o.OriginalPriceCents <= o.PriceCents {
			t.Errorf("offer %d: original price must be higher than the price", i)
		}
		if o.KeyCount < 1 {
			t.Errorf("offer %d: needs at least one key", i)
		}
	}
}
