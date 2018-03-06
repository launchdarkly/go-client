package middleware

import (
	"net/http"
)

var NoCache = func(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't cache this content. Really.
		w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate, no-store")
		h.ServeHTTP(w, r)
	})
}
