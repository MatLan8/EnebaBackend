package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// The types below match the layout of seed/data/seed.json.
// IDs in the file are the integer IDs from the scraped dump. They are only
// used to link rows inside the file; the database gets new UUIDs.

type seedFile struct {
	Users    []seedUser    `json:"users"`
	Genres   []seedGenre   `json:"genres"`
	Sellers  []seedSeller  `json:"sellers"`
	Products []seedProduct `json:"products"`
	Offers   []seedOffer   `json:"offers"`
}

type seedGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type seedSeller struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	LogoURL *string `json:"logoUrl"`
	Rating  float64 `json:"rating"` // 0 to 5 in the dump, doubled to our 0 to 10 scale
}

type seedProduct struct {
	ID             int     `json:"id"`
	Title          string  `json:"title"`
	Slug           string  `json:"slug"`
	Description    string  `json:"description"`
	ImageURL       string  `json:"imageUrl"`
	Type           string  `json:"type"`
	Platform       string  `json:"platform"`
	Region         string  `json:"region"`
	Developer      *string `json:"developer"`
	Publisher      *string `json:"publisher"`
	ReleaseDate    string  `json:"releaseDate"`    // "2006-01-02", required
	BasePriceCents int     `json:"basePriceCents"` // official store price or face value, required
	GenreIDs       []int   `json:"genreIds"`
}

type seedOffer struct {
	ProductID  int    `json:"productId"`
	SellerID   int    `json:"sellerId"`
	PriceCents int    `json:"priceCents"`
	Currency   string `json:"currency"`
	IsActive   bool   `json:"isActive"`
	KeyCount   int    `json:"keyCount"` // how many fake digital keys to generate
}

// loadSeedFile reads and decodes the seed JSON file.
func loadSeedFile(path string) (*seedFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read seed file: %w", err)
	}

	var data seedFile
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode seed file: %w", err)
	}

	// Check users, required product fields and offer prices before anything
	// touches the database, so bad data fails the run before -reset wipes the
	// catalog.
	normalizeUsers(data.Users)
	if err := validateUsers(data.Users); err != nil {
		return nil, err
	}

	basePrices := make(map[int]int, len(data.Products))
	for _, p := range data.Products {
		if _, err := parseDate(p.ReleaseDate); err != nil {
			return nil, fmt.Errorf("product %q: %w", p.Slug, err)
		}
		if p.BasePriceCents <= 0 {
			return nil, fmt.Errorf("product %q: basePriceCents must be positive, got %d", p.Slug, p.BasePriceCents)
		}
		basePrices[p.ID] = p.BasePriceCents
	}

	// Discounts are measured against the base price, so an offer may match it
	// but never cost more.
	for i, o := range data.Offers {
		basePrice, ok := basePrices[o.ProductID]
		if ok && o.PriceCents > basePrice {
			return nil, fmt.Errorf(
				"offer %d (product %d): priceCents %d is above the base price %d",
				i, o.ProductID, o.PriceCents, basePrice,
			)
		}
	}

	return &data, nil
}
