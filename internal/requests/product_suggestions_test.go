package requests

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"eneba-backend/internal/httpx"
)

func TestParseSuggestionQueryMissingQ(t *testing.T) {
	_, err := ParseSuggestionQuery(url.Values{})

	var apiErr *httpx.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v; want *httpx.APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest || apiErr.Code != "validation_failed" {
		t.Errorf("status, code = %d, %q; want 400, validation_failed", apiErr.Status, apiErr.Code)
	}
	if _, ok := apiErr.Details["q"]; !ok {
		t.Errorf("details = %v; want a problem for q", apiErr.Details)
	}
}

func TestParseSuggestionQuery(t *testing.T) {
	tests := []struct {
		name       string
		rawQuery   string
		wantSearch string
		wantShort  bool
	}{
		{name: "trimmed", rawQuery: "q=+eld+", wantSearch: "eld", wantShort: false},
		{name: "two letters", rawQuery: "q=ab", wantSearch: "ab", wantShort: true},
		{name: "spaces around two letters", rawQuery: "q=++ab++", wantSearch: "ab", wantShort: true},
		{name: "empty", rawQuery: "q=", wantSearch: "", wantShort: true},
		{name: "three runes, six bytes", rawQuery: "q=%C4%85%C4%8D%C4%99", wantSearch: "ąčę", wantShort: false},
		{name: "special characters", rawQuery: "q=%25%25%25", wantSearch: "%%%", wantShort: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := url.ParseQuery(tt.rawQuery)
			if err != nil {
				t.Fatalf("parse query: %v", err)
			}

			params, err := ParseSuggestionQuery(query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if params.Search != tt.wantSearch {
				t.Errorf("search = %q; want %q", params.Search, tt.wantSearch)
			}
			if params.IsTooShort() != tt.wantShort {
				t.Errorf("IsTooShort() = %v; want %v", params.IsTooShort(), tt.wantShort)
			}
		})
	}
}
