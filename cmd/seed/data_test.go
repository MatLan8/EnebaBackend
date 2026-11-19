package main

import (
	"slices"
	"testing"

	"eneba-backend/internal/models"
	"eneba-backend/internal/requests"
)

// Must match the slugs inserted by migrations/000011_platforms.up.sql.
var platformSlugs = []string{
	"steam", "epic_games", "ea_app", "battlenet", "psn", "ubisoft", "xbox_live", "nintendo_eshop",
	"microsoft", "roblox", "google_play", "windows", "eset", "spotify", "discord", "riot_games",
}

// Checks the real seed file, so a data typo fails here instead of halfway through a seed run.
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
		if !slices.Contains(platformSlugs, p.Platform) {
			t.Errorf("product %q: unknown platform %q", p.Slug, p.Platform)
		}
		if !slices.Contains(models.Regions, models.Region(p.Region)) {
			t.Errorf("product %q: unknown region %q", p.Slug, p.Region)
		}
		if _, err := parseDate(p.ReleaseDate); err != nil {
			t.Errorf("product %q: %v", p.Slug, err)
		}
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
		if o.PriceCents > basePrices[o.ProductID] {
			t.Errorf("offer %d: price %d is above the base price %d", i, o.PriceCents, basePrices[o.ProductID])
		}
		if o.KeyCount < 1 {
			t.Errorf("offer %d: needs at least one key", i)
		}
	}
}

func TestDevPassword_PassesRegisterRules(t *testing.T) {
	req := requests.RegisterRequest{
		Name:     "Seed User",
		Email:    "seed@example.com",
		Password: devPassword,
	}
	if err := requests.Validate(&req); err != nil {
		t.Fatalf("devPassword %q should pass register rules: %v", devPassword, err)
	}
}
