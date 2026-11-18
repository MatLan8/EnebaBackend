package requests

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"eneba-backend/internal/httpx"
)

func TestParseOrderIDParamInvalid(t *testing.T) {
	for _, raw := range []string{"", "o-k3x9q2a", "123", uuid.Nil.String()} {
		_, err := ParseOrderIDParam(raw)

		var apiErr *httpx.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "validation_failed" {
			t.Errorf("ParseOrderIDParam(%q) err = %v; want validation_failed", raw, err)
			continue
		}
		if _, ok := apiErr.Details["id"]; !ok {
			t.Errorf("ParseOrderIDParam(%q) details = %v; want an entry for id", raw, apiErr.Details)
		}
	}
}
