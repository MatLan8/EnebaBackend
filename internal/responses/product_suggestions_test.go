package responses

import (
	"encoding/json"
	"testing"
)

func TestNewProductSuggestionsResponseEmpty(t *testing.T) {
	got, err := json.Marshal(NewProductSuggestionsResponse(nil, 0))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The dropdown expects [] when nothing matches, never null.
	want := `{"items":[],"total":0}`
	if string(got) != want {
		t.Errorf("json = %s; want %s", got, want)
	}
}
