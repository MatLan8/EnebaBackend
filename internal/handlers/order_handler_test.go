package handlers

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"eneba-backend/internal/models"
	"eneba-backend/internal/repository"
	"eneba-backend/internal/services"
)

func newTestKeyCipher(t *testing.T) *services.KeyCipher {
	t.Helper()

	keyCipher, err := services.NewKeyCipher(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}

	return keyCipher
}

func TestNewOrderKeysResponseDecryptsInOrder(t *testing.T) {
	keyCipher := newTestKeyCipher(t)
	order := models.Order{ID: uuid.New(), PublicID: "o-abc1234"} // publicId must not be in the response
	logo := "https://example.com/logo.png"
	rows := []repository.OrderKeyRow{
		{KeyID: uuid.New(), CodeEncrypted: keyCipher.Encrypt("AAAA-1111"), Title: "Elden Ring", Platform: models.PlatformSteam, SellerName: "PixelVault", SellerLogoURL: &logo, SellerRating: 9.5},
		{KeyID: uuid.New(), CodeEncrypted: keyCipher.Encrypt("BBBB-2222"), Title: "Elden Ring", SellerName: "NeonKey"},
	}

	got, err := newOrderKeysResponse(order, rows, keyCipher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.OrderID != order.ID || len(got.Keys) != 2 {
		t.Fatalf("response = %+v", got)
	}
	if got.Keys[0].Code != "AAAA-1111" || got.Keys[1].Code != "BBBB-2222" {
		t.Errorf("codes are wrong or out of order")
	}
	first := got.Keys[0]
	if first.ID != rows[0].KeyID || first.Product.Title != "Elden Ring" || first.Product.Platform != models.PlatformSteam {
		t.Errorf("first key = %+v", first)
	}
	if first.Seller.Name != "PixelVault" || first.Seller.LogoURL != &logo || first.Seller.Rating != 9.5 {
		t.Errorf("first seller = %+v", first.Seller)
	}
}

func TestNewOrderKeysResponseNoKeysIsEmptyJSONArray(t *testing.T) {
	got, err := newOrderKeysResponse(models.Order{ID: uuid.New()}, nil, newTestKeyCipher(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body, _ := json.Marshal(got)
	if !strings.Contains(string(body), `"keys":[]`) {
		t.Errorf("JSON = %s; want \"keys\":[]", body)
	}
	if strings.Contains(string(body), "publicId") {
		t.Errorf("JSON = %s; publicId must not be returned", body)
	}
}

func TestNewOrderKeysResponseDecryptFailureNamesKeyNotCode(t *testing.T) {
	keyCipher := newTestKeyCipher(t)
	brokenID := uuid.New()
	rows := []repository.OrderKeyRow{
		{KeyID: uuid.New(), CodeEncrypted: keyCipher.Encrypt("GOOD-CODE")},
		{KeyID: brokenID, CodeEncrypted: []byte("not encrypted with this secret")},
	}

	_, err := newOrderKeysResponse(models.Order{ID: uuid.New()}, rows, keyCipher)
	if err == nil {
		t.Fatal("want an error for the broken key")
	}
	if !strings.Contains(err.Error(), brokenID.String()) {
		t.Errorf("error %q should name the broken key %s", err, brokenID)
	}
	if strings.Contains(err.Error(), "GOOD-CODE") {
		t.Errorf("error %q must not contain a key code", err)
	}
}
