package requests

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"eneba-backend/internal/httpx"
)

func TestParseKeyListQueryDefaults(t *testing.T) {
	params, err := ParseKeyListQuery(url.Values{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Page != 1 || params.Limit != 10 || params.Search != "" {
		t.Errorf("params = %+v; want page 1, limit 10, no search", params)
	}
}

func TestParseKeyListQueryValues(t *testing.T) {
	query, _ := url.ParseQuery("q=+Elden+Ring+&page=2&limit=5")

	params, err := ParseKeyListQuery(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Search != "Elden Ring" {
		t.Errorf("search = %q; want trimmed %q", params.Search, "Elden Ring")
	}
	if params.Page != 2 || params.Limit != 5 || params.Offset() != 5 {
		t.Errorf("page, limit, offset = %d, %d, %d; want 2, 5, 5", params.Page, params.Limit, params.Offset())
	}
}

func TestParseKeyListQueryInvalid(t *testing.T) {
	tests := []struct {
		name  string
		query string
		field string // the parameter that must be reported
	}{
		{"page zero", "page=0", "page"},
		{"page not a number", "page=first", "page"},
		{"limit zero", "limit=0", "limit"},
		{"limit too big", "limit=61", "limit"},
		{"limit not a number", "limit=many", "limit"},
		{"q too long", "q=" + strings.Repeat("a", 101), "q"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, _ := url.ParseQuery(tt.query)

			_, err := ParseKeyListQuery(query)

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

func TestKeyListParamsValidate(t *testing.T) {
	ok := []KeyListParams{
		{Page: 1, Limit: 1},
		{Page: 3, Limit: 60, Search: strings.Repeat("ž", 100)}, // counted in characters, not bytes
	}
	for _, params := range ok {
		if err := params.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v; want nil", params, err)
		}
	}

	var apiErr *httpx.APIError
	bad := KeyListParams{Page: 0, Limit: 0, Search: strings.Repeat("a", 101)}
	if !errors.As(bad.Validate(), &apiErr) || len(apiErr.Details) != 3 {
		t.Errorf("Validate(%+v) should report page, limit and q", bad)
	}
}
