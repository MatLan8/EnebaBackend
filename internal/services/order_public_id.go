package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	orderPublicIDPrefix   = "o-"
	orderPublicIDAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	orderPublicIDLength   = 7 // 36^7 is about 78 billion, collisions are rare and retried
)

// NewOrderPublicID returns a short random order number such as "o-k3x9q2a",
// shown to the buyer instead of the UUID. crypto/rand makes it unguessable, so
// one order number gives no hint about the next one.
func NewOrderPublicID() (string, error) {
	max := big.NewInt(int64(len(orderPublicIDAlphabet)))

	id := make([]byte, orderPublicIDLength)
	for i := range id {
		// rand.Int picks evenly from 0..35, unlike byte % 36, which would favour some characters.
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate order public id: %w", err)
		}
		id[i] = orderPublicIDAlphabet[n.Int64()]
	}

	return orderPublicIDPrefix + string(id), nil
}
