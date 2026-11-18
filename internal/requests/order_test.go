package requests

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"eneba-backend/internal/httpx"
)

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
