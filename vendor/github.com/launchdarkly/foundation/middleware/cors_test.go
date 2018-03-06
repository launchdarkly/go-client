package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const ok = "ok\n"

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte(ok))
})

var siteMap map[string][]string = make(map[string][]string)

func TestCorsHeadersWithoutOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/", nil)

	handler := CorsHeadersMiddleware(&siteMap, "/", okHandler)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestCorsHeadersWithOrigin(t *testing.T) {
	origin := "https://otherdomain.com"
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("Origin", origin)

	handler := CorsHeadersMiddleware(&siteMap, "/", okHandler)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, origin, rr.Header().Get("Access-Control-Allow-Origin"))
}
