package middleware

import (
	"io"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/justinas/alice"
)

func LoggingHandler(out io.Writer) alice.Constructor {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.UserAgent() != "HAProxy/HealthCheck" && r.UserAgent() != "LD/HealthCheck" {
				handlers.LoggingHandler(out, h).ServeHTTP(w, r)
			} else {
				h.ServeHTTP(w, r)
			}
		})
	}
}

func CombinedLoggingHandler(out io.Writer) alice.Constructor {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.UserAgent() != "HAProxy/HealthCheck" && r.UserAgent() != "LD/HealthCheck" {
				handlers.CombinedLoggingHandler(out, h).ServeHTTP(w, r)
			} else {
				h.ServeHTTP(w, r)
			}
		})
	}
}
