package requests

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"eneba-backend/internal/httpx"
)

const (
	defaultKeyPageLimit = 10
	maxKeyPageLimit     = 60
	maxKeySearchLength  = 100
)

// KeyListParams is what the signed-in user's key library is searched and paged
// by, read from the query string of GET /api/v1/keys.
type KeyListParams struct {
	Search string // part of a product title or an order's public ID, any case; "" means no search
	Page   int    // starts at 1
	Limit  int    // keys per page
}

// Offset is the number of rows to skip to reach Page.
func (p KeyListParams) Offset() int {
	return (p.Page - 1) * p.Limit
}

// ParseKeyListQuery reads the query string of GET /api/v1/keys.
// Every parameter is optional: ?q=elden&page=2&limit=10.
func ParseKeyListQuery(query url.Values) (KeyListParams, error) {
	params := KeyListParams{
		Search: strings.TrimSpace(query.Get("q")),
		Page:   intOrDefault(query.Get("page"), 1),
		Limit:  intOrDefault(query.Get("limit"), defaultKeyPageLimit),
	}

	if err := params.Validate(); err != nil {
		return KeyListParams{}, err
	}

	return params, nil
}

// Validate checks the params. Every problem found is returned at once as a 400
// validation error, keyed by query parameter name.
func (p KeyListParams) Validate() error {
	details := map[string]string{}

	if p.Page < 1 {
		details["page"] = "must be a whole number, 1 or more"
	}
	if p.Limit < 1 || p.Limit > maxKeyPageLimit {
		details["limit"] = fmt.Sprintf("must be a whole number from 1 to %d", maxKeyPageLimit)
	}
	if utf8.RuneCountInString(p.Search) > maxKeySearchLength {
		details["q"] = fmt.Sprintf("must be at most %d characters", maxKeySearchLength)
	}

	if len(details) > 0 {
		return httpx.ValidationFailed(details)
	}

	return nil
}
