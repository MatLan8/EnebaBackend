package requests

import (
	"net/url"
	"strings"
	"unicode/utf8"

	"eneba-backend/internal/httpx"
)

// MinSuggestionQueryLength is how many characters the search box needs before
// suggestions are looked up. Shorter text matches too many products to be useful.
const MinSuggestionQueryLength = 3

// ProductSuggestionParams is read from the query string of
// GET /api/v1/products/suggestions.
type ProductSuggestionParams struct {
	Search string // trimmed, matched against the title the same way as the store's search
}

// IsTooShort reports whether Search has fewer than MinSuggestionQueryLength
// characters. It counts runes, not bytes, so "ąčę" is 3 characters, not 6.
func (p ProductSuggestionParams) IsTooShort() bool {
	return utf8.RuneCountInString(p.Search) < MinSuggestionQueryLength
}

// ParseSuggestionQuery reads the query string of GET /api/v1/products/suggestions.
// q must be present: a request without it is a bug in the client, so it is a 400.
// q may be empty or short though (the user is still typing); the handler then
// returns no suggestions instead of an error.
func ParseSuggestionQuery(query url.Values) (ProductSuggestionParams, error) {
	if !query.Has("q") {
		return ProductSuggestionParams{}, httpx.ValidationFailed(map[string]string{
			"q": "is required",
		})
	}

	return ProductSuggestionParams{
		Search: strings.TrimSpace(query.Get("q")),
	}, nil
}
