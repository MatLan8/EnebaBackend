package responses

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestDistinctSellerNames(t *testing.T) {
	items := []OrderDetailItemResponse{
		{SellerName: "PixelVault"},
		{SellerName: "KeyForge"},
		{SellerName: "PixelVault"},
		{SellerName: "GameHub"},
		{SellerName: "KeyForge"},
	}

	got := DistinctSellerNames(items)

	want := []string{"PixelVault", "KeyForge", "GameHub"}
	if !slices.Equal(got, want) {
		t.Errorf("DistinctSellerNames = %v; want %v", got, want)
	}
}

func TestDistinctSellerNamesNoItemsIsEmptyJSONArray(t *testing.T) {
	got := DistinctSellerNames(nil)

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "[]" {
		t.Errorf("JSON = %s; want []", body)
	}
}
