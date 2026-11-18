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

	// A full public ID such as "o-k3x9q2a" is 9 characters.
	maxOrderPublicIDSearchLength = 32
)

type OrderListParams struct {
	PublicID string // part of the public ID, any case; "" means no filter
	Page     int
	Limit    int
}

func (p OrderListParams) Offset() int {
	return (p.Page - 1) * p.Limit
}

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

// Every problem is returned at once, keyed by query parameter name.
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

// Anything that isn't a whole number gives 0, which is never a valid page or limit, so Validate reports it.
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
