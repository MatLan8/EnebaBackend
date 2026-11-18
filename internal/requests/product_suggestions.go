package requests

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"eneba-backend/internal/httpx"
)

// Shorter text matches too many products to be useful.
const MinSuggestionQueryLength = 3

type ProductSuggestionParams struct {
	Search string // trimmed
}

// Counts runes, not bytes, so "ąčę" is 3 characters, not 6.
func (p ProductSuggestionParams) IsTooShort() bool {
	return utf8.RuneCountInString(p.Search) < MinSuggestionQueryLength
}

// A missing q is a client bug (400), but an empty or short q is fine while the user types:
// the handler then returns no suggestions.
func ParseSuggestionQuery(query url.Values) (ProductSuggestionParams, error) {
	if !query.Has("q") {
		return ProductSuggestionParams{}, httpx.ValidationFailed(map[string]string{
			"q": "is required",
		})
	}

	params := ProductSuggestionParams{
		Search: strings.TrimSpace(query.Get("q")),
	}
	if utf8.RuneCountInString(params.Search) > maxProductSearchLength {
		return ProductSuggestionParams{}, httpx.ValidationFailed(map[string]string{
			"q": fmt.Sprintf("must be at most %d characters", maxProductSearchLength),
		})
	}

	return params, nil
}
