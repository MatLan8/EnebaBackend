package main

import (
	"slices"
	"testing"
	"time"

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
	basePrices := make(map[int]int)
	for _, p := range data.Products {
		products[p.ID] = true
		basePrices[p.ID] = p.BasePriceCents
		if !slices.Contains(models.ProductTypes, models.ProductType(p.Type)) {
			t.Errorf("product %q: unknown type %q", p.Slug, p.Type)
		}
		if !slices.Contains(models.Platforms, models.Platform(p.Platform)) {
			t.Errorf("product %q: unknown platform %q", p.Slug, p.Platform)
		}
		if !slices.Contains(models.Regions, models.Region(p.Region)) {
			t.Errorf("product %q: unknown region %q", p.Slug, p.Region)
		}
		// release_date is NOT NULL, so every product needs a valid date.
		if _, err := parseDate(p.ReleaseDate); err != nil {
			t.Errorf("product %q: %v", p.Slug, err)
		}
		// base_price_cents is NOT NULL and must be positive, discounts are measured against it.
		if p.BasePriceCents <= 0 {
			t.Errorf("product %q: base price must be positive", p.Slug)
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
		// Discounts are measured against the base price, so an offer may match it but never cost more.
		if o.PriceCents > basePrices[o.ProductID] {
			t.Errorf("offer %d: price %d is above the base price %d", i, o.PriceCents, basePrices[o.ProductID])
		}
		if o.KeyCount < 1 {
			t.Errorf("offer %d: needs at least one key", i)
		}
	}
}

func TestParseDate(t *testing.T) {
	got, err := parseDate("2013-11-29")
	if err != nil {
		t.Fatalf("parseDate: %v", err)
	}
	if want := time.Date(2013, 11, 29, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("parseDate = %v, want %v", got, want)
	}

	for _, value := range []string{"", "29/11/2013", "2013-13-01"} {
		if _, err := parseDate(value); err == nil {
			t.Errorf("parseDate(%q): expected an error", value)
		}
	}
}
