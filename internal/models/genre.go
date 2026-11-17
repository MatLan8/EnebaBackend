package models

import "github.com/google/uuid"

// Genre mirrors a row in the genres table.
// Products and genres are linked through the product_genres join table,
// which has no model of its own: it only holds the two IDs.
type Genre struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}
