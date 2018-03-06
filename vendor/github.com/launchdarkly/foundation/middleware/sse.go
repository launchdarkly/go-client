package middleware

import (
	"mime"
	"net/http"
)

const esMediaType = "text/event-stream"

func isTextEventStream(s string) bool {
	mt, _, err := mime.ParseMediaType(s)

	if err == nil && mt == esMediaType {
		return true
	}

	return false
}

var EventStreamOnly = func(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For all requests, set the Content-Type header to text/event-stream
		w.Header().Set("Content-Type", esMediaType)
		h.ServeHTTP(w, r)
	})
}
