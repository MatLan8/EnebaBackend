package handlers

import (
	"net/http"
	"time"
)

const (
	refreshCookieName = "refresh_token"
	// The browser only sends the cookie to /api/v1/auth/*, not to every API call.
	refreshCookiePath = "/api/v1/auth"
)

// setRefreshCookie stores the raw refresh token in an HttpOnly cookie,
// so JavaScript (and any XSS) can never read it.
func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: h.sameSite(),
	})
}

// clearRefreshCookie tells the browser to delete the cookie (used by logout).
func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: h.sameSite(),
	})
}

func (h *AuthHandler) sameSite() http.SameSite {
	return cookieSameSite(h.secureCookies)
}

// cookieSameSite is shared by the refresh token and cart cookies. In production
// the frontend and API are on different domains, so a cookie must be
// SameSite=None (which browsers only accept with Secure). Locally both run on
// localhost, where Lax works without HTTPS.
func cookieSameSite(secure bool) http.SameSite {
	if secure {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}
