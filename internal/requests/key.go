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

type KeyListParams struct {
	Search string // product title or order public ID, any case; "" means no search
	Page   int
	Limit  int
}

func (p KeyListParams) Offset() int {
	return (p.Page - 1) * p.Limit
}

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

// Every problem is returned at once, keyed by query parameter name.
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
