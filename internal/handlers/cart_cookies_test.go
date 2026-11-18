package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestCartIDFromCookie(t *testing.T) {
	cartID := uuid.New()

	tests := []struct {
		name   string
		cookie *http.Cookie
		wantID uuid.UUID
		wantOK bool
	}{
		{name: "valid", cookie: &http.Cookie{Name: cartCookieName, Value: cartID.String()}, wantID: cartID, wantOK: true},
		{name: "missing", cookie: nil},
		{name: "garbage", cookie: &http.Cookie{Name: cartCookieName, Value: "not-a-uuid"}},
		{name: "nil uuid", cookie: &http.Cookie{Name: cartCookieName, Value: uuid.Nil.String()}},
		{name: "other cookie", cookie: &http.Cookie{Name: "refresh_token", Value: cartID.String()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
			if tt.cookie != nil {
				r.AddCookie(tt.cookie)
			}

			gotID, gotOK := cartIDFromCookie(r)
			if gotID != tt.wantID || gotOK != tt.wantOK {
				t.Errorf("cartIDFromCookie = %s, %v; want %s, %v", gotID, gotOK, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestSetCartCookie(t *testing.T) {
	cartID := uuid.New()

	tests := []struct {
		secure       bool
		wantSameSite http.SameSite
	}{
		{secure: false, wantSameSite: http.SameSiteLaxMode},
		{secure: true, wantSameSite: http.SameSiteNoneMode},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		setCartCookie(w, cartID, tt.secure)

		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("got %d cookies; want 1", len(cookies))
		}
		c := cookies[0]

		if c.Name != cartCookieName || c.Value != cartID.String() || c.Path != "/" {
			t.Errorf("name, value, path = %q, %q, %q; want %q, %q, /", c.Name, c.Value, c.Path, cartCookieName, cartID)
		}
		if !c.HttpOnly || c.Secure != tt.secure || c.SameSite != tt.wantSameSite {
			t.Errorf("httpOnly, secure, sameSite = %v, %v, %v; want true, %v, %v", c.HttpOnly, c.Secure, c.SameSite, tt.secure, tt.wantSameSite)
		}
		if c.MaxAge != 180*24*60*60 {
			t.Errorf("maxAge = %d; want 180 days", c.MaxAge)
		}
	}
}
