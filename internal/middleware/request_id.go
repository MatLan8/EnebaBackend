package middleware

import (
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// chi's RequestID only stores the ID in the context; this also sends it back, so a user's
// error report can be matched to the server logs. Must run after chimw.RequestID.
func RequestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := chimw.GetReqID(r.Context()); id != "" {
			w.Header().Set("X-Request-Id", id)
		}
		next.ServeHTTP(w, r)
	})
}
