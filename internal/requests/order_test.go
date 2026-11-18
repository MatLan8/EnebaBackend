package requests

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"eneba-backend/internal/httpx"
)

func TestParseOrderListQueryDefaults(t *testing.T) {
	params, err := ParseOrderListQuery(url.Values{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.Page != 1 || params.Limit != 10 || params.PublicID != "" {
		t.Errorf("params = %+v; want page 1, limit 10, no publicId", params)
	}
}

func TestParseOrderListQueryValues(t *testing.T) {
	query, _ := url.ParseQuery("publicId=+K3X+&page=3&limit=5")

	params, err := ParseOrderListQuery(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params.PublicID != "K3X" {
		t.Errorf("publicId = %q; want trimmed %q", params.PublicID, "K3X")
	}
	if params.Page != 3 || params.Limit != 5 {
		t.Errorf("page, limit = %d, %d; want 3, 5", params.Page, params.Limit)
	}
	if params.Offset() != 10 {
		t.Errorf("offset = %d; want 10", params.Offset())
	}
}

func TestParseOrderListQueryInvalid(t *testing.T) {
	tests := []struct {
		name  string
		query string
		field string // the parameter that must be reported
	}{
		{"page zero", "page=0", "page"},
		{"negative page", "page=-2", "page"},
		{"page not a number", "page=two", "page"},
		{"limit zero", "limit=0", "limit"},
		{"limit too big", "limit=61", "limit"},
		{"limit not a number", "limit=all", "limit"},
		{"publicId too long", "publicId=" + strings.Repeat("a", 33), "publicId"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, _ := url.ParseQuery(tt.query)

			_, err := ParseOrderListQuery(query)

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

func TestOrderListParamsValidateReportsEveryProblem(t *testing.T) {
	params := OrderListParams{PublicID: strings.Repeat("x", 40), Page: 0, Limit: 100}

	var apiErr *httpx.APIError
	if !errors.As(params.Validate(), &apiErr) {
		t.Fatal("want a validation error")
	}
	for _, field := range []string{"page", "limit", "publicId"} {
		if _, ok := apiErr.Details[field]; !ok {
			t.Errorf("details = %v; want an entry for %q", apiErr.Details, field)
		}
	}
}

func TestOrderListParamsValidateAcceptsLimits(t *testing.T) {
	for _, params := range []OrderListParams{
		{Page: 1, Limit: 1},
		{Page: 1, Limit: 60, PublicID: strings.Repeat("a", 32)},
	} {
		if err := params.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v; want nil", params, err)
		}
	}
}
