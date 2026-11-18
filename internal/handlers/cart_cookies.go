package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	// cartCookieName holds the ID of the browser's cart. The cart belongs to the
	// browser, not to a user, so it works before signing in, like on Eneba.
	cartCookieName = "cart_token"
	// cartCookieMaxAge is renewed on every change, so a cart only expires
	// after 180 days without being touched.
	cartCookieMaxAge = 180 * 24 * time.Hour
)

// cartIDFromCookie returns the cart ID from the cart_token cookie. ok is false
// if the cookie is missing or isn't a valid UUID; the caller treats that the
// same as a cart that doesn't exist.
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

// setCartCookie stores the cart ID in an HttpOnly cookie for the whole site,
// so the frontend's JavaScript never sees it. secure and SameSite follow the
// refresh token cookie: Secure + None in production, Lax on localhost.
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
