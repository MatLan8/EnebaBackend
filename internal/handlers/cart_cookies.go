package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	// The cart belongs to the browser, not a user, so it works before signing in (like Eneba).
	cartCookieName = "cart_token"
	// Renewed on every change, so a cart only expires after 180 days untouched.
	cartCookieMaxAge = 180 * 24 * time.Hour
)

// A missing or invalid cookie is treated like a cart that doesn't exist.
func cartIDFromCookie(r *http.Request) (uuid.UUID, bool) {
	cookie, err := r.Cookie(cartCookieName)
	if err != nil {
		return uuid.Nil, false
	}

	cartID, err := uuid.Parse(cookie.Value)
	if err != nil || cartID == uuid.Nil {
		return uuid.Nil, false
	}

	return cartID, true
}

// HttpOnly and site-wide; Secure and SameSite follow the refresh token cookie.
func setCartCookie(w http.ResponseWriter, cartID uuid.UUID, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cartCookieName,
		Value:    cartID.String(),
		Path:     "/",
		Expires:  time.Now().Add(cartCookieMaxAge),
		MaxAge:   int(cartCookieMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: cookieSameSite(secure),
	})
}
