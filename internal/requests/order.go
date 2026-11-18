package requests

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"eneba-backend/internal/httpx"
)

const (
	defaultOrderPageLimit = 10
	maxOrderPageLimit     = 60

	// maxOrderPublicIDSearchLength stops very long search strings. A full public
	// ID such as "o-k3x9q2a" is 9 characters.
	maxOrderPublicIDSearchLength = 32
)

// OrderListParams is what the signed-in user's order list is filtered and paged
// by, read from the query string of GET /api/v1/orders.
type OrderListParams struct {
	PublicID string // part of an order's public ID, any case; "" means no filter
	Page     int    // starts at 1
	Limit    int    // orders per page
}

// Offset is the number of rows to skip to reach Page.
func (p OrderListParams) Offset() int {
	return (p.Page - 1) * p.Limit
}

// ParseOrderListQuery reads the query string of GET /api/v1/orders.
// Every parameter is optional: ?publicId=k3x&page=2&limit=10.
func ParseOrderListQuery(query url.Values) (OrderListParams, error) {
	params := OrderListParams{
		PublicID: strings.TrimSpace(query.Get("publicId")),
		Page:     intOrDefault(query.Get("page"), 1),
		Limit:    intOrDefault(query.Get("limit"), defaultOrderPageLimit),
	}

	if err := params.Validate(); err != nil {
		return OrderListParams{}, err
	}

	return params, nil
}

// Validate checks the params. Every problem found is returned at once as a 400
// validation error, keyed by query parameter name.
func (p OrderListParams) Validate() error {
	details := map[string]string{}

	if p.Page < 1 {
		details["page"] = "must be a whole number, 1 or more"
	}
	if p.Limit < 1 || p.Limit > maxOrderPageLimit {
		details["limit"] = fmt.Sprintf("must be a whole number from 1 to %d", maxOrderPageLimit)
	}
	if utf8.RuneCountInString(p.PublicID) > maxOrderPublicIDSearchLength {
		details["publicId"] = fmt.Sprintf("must be at most %d characters", maxOrderPublicIDSearchLength)
	}

	if len(details) > 0 {
		return httpx.ValidationFailed(details)
	}

	return nil
}

// intOrDefault reads a whole number from the query string. An empty value gives
// fallback. Anything that isn't a whole number gives 0, which is never a valid
// page or limit, so Validate reports it.
func intOrDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}

	return n
}
