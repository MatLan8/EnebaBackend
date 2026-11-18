package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"eneba-backend/internal/models"
	"eneba-backend/internal/requests"
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

func TestSeedFile_Users(t *testing.T) {
	raw, err := os.ReadFile("../../seed/data/seed.json")
	if err != nil {
		t.Fatalf("read seed file: %v", err)
	}

	// Make sure no user in the file carries a password. The shared
	// dev password lives in cmd/seed, not in the JSON.
	var payload struct {
		Users []map[string]json.RawMessage `json:"users"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode seed file: %v", err)
	}
	if len(payload.Users) != 20 {
		t.Errorf("got %d users, want 20", len(payload.Users))
	}
	for i, u := range payload.Users {
		if _, ok := u["password"]; ok {
			t.Errorf("user %d: seed file must not contain a password", i)
		}
		if _, ok := u["name"]; !ok {
			t.Errorf("user %d: missing name", i)
		}
		if _, ok := u["email"]; !ok {
			t.Errorf("user %d: missing email", i)
		}
	}

	data, err := loadSeedFile("../../seed/data/seed.json")
	if err != nil {
		t.Fatalf("load seed file: %v", err)
	}
	if len(data.Users) != 20 {
		t.Fatalf("got %d users, want 20", len(data.Users))
	}

	emails := make(map[string]string, len(data.Users))
	for i, u := range data.Users {
		if u.Email != strings.ToLower(u.Email) {
			t.Errorf("user %d %q: email %q is not lowercase", i, u.Name, u.Email)
		}
		if !strings.HasSuffix(u.Email, "@example.com") {
			t.Errorf("user %d %q: email %q should use the example.com domain", i, u.Name, u.Email)
		}
		if name, ok := emails[u.Email]; ok {
			t.Errorf("duplicate email %q (%q and %q)", u.Email, name, u.Name)
		}
		emails[u.Email] = u.Name
	}
}

func TestValidateUsers(t *testing.T) {
	valid := seedUser{Name: "Emma Johnson", Email: "emma.johnson@example.com"}
	if err := validateUsers([]seedUser{valid}); err != nil {
		t.Fatalf("valid user: %v", err)
	}
	if err := validateUsers(nil); err != nil {
		t.Fatalf("empty list: %v", err)
	}

	tests := []struct {
		name    string
		users   []seedUser
		wantSub string
	}{
		{
			name:    "empty name",
			users:   []seedUser{{Name: "", Email: "a@example.com"}},
			wantSub: "name is required",
		},
		{
			name:    "short name",
			users:   []seedUser{{Name: "A", Email: "a@example.com"}},
			wantSub: "name must be between",
		},
		{
			name:    "missing email",
			users:   []seedUser{{Name: "Emma Johnson", Email: ""}},
			wantSub: "email is required",
		},
		{
			name:    "invalid email",
			users:   []seedUser{{Name: "Emma Johnson", Email: "not-an-email"}},
			wantSub: "invalid email",
		},
		{
			name:    "email without domain dot",
			users:   []seedUser{{Name: "Emma Johnson", Email: "emma@localhost"}},
			wantSub: "invalid email",
		},
		{
			name:    "uppercase email",
			users:   []seedUser{{Name: "Emma Johnson", Email: "Emma.Johnson@example.com"}},
			wantSub: "must be lowercase",
		},
		{
			name: "duplicate email",
			users: []seedUser{
				valid,
				{Name: "Emma J", Email: "emma.johnson@example.com"},
			},
			wantSub: "duplicate email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUsers(tt.users)
			if err == nil {
				t.Fatalf("expected an error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err, tt.wantSub)
			}
		})
	}
}

func TestLoadSeedFile_RejectsInvalidUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.json")
	content := `{
		"users": [{"name": "Emma Johnson", "email": "Emma.Johnson@example.com"}],
		"genres": [],
		"sellers": [],
		"products": [],
		"offers": []
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadSeedFile(path)
	if err == nil {
		t.Fatal("expected uppercase email to be rejected")
	}
	if !strings.Contains(err.Error(), "must be lowercase") {
		t.Errorf("error %q does not mention lowercase", err)
	}
}

func TestLoadSeedFile_TrimsUserFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.json")
	content := `{
		"users": [{"name": "  Emma Johnson  ", "email": "  emma.johnson@example.com  "}],
		"genres": [],
		"sellers": [],
		"products": [],
		"offers": []
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	data, err := loadSeedFile(path)
	if err != nil {
		t.Fatalf("load seed file: %v", err)
	}
	if data.Users[0].Name != "Emma Johnson" {
		t.Errorf("name = %q, want trimmed", data.Users[0].Name)
	}
	if data.Users[0].Email != "emma.johnson@example.com" {
		t.Errorf("email = %q, want trimmed", data.Users[0].Email)
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
