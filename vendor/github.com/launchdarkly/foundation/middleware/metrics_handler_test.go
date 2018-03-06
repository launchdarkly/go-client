package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The unspoken assertion here is that we're handling a null mux route without panicking
func TestMetricsNameForUnknownRoute(t *testing.T) {
	h := MetricsRecordingResponseWriter{nil, 202, 99}
	req, _ := http.NewRequest("POST", "http://fake.com/path/morepath?query=param", nil)
	name := h.metricsName(req)
	assert.Equal(t, "POST.unknown_route.202.", name)
}

// The unspoken assertion here is that we're handling a null mux route without panicking
func TestMetricsNameForStaticAsset(t *testing.T) {
	h := MetricsRecordingResponseWriter{nil, 200, 99}
	req, _ := http.NewRequest("GET", "http://fake.com/s/unicorn.jpg?query=param", nil)
	name := h.metricsName(req)
	assert.Equal(t, "GET.s_unicorn-jpg.200.", name)
}
