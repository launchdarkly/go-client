package middleware

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alecthomas/units"
	"github.com/launchdarkly/foundation/config"
	"github.com/launchdarkly/foundation/fmetrics"
	metrics "github.com/launchdarkly/go-metrics"
	"github.com/stretchr/testify/assert"
)

func TestMaxRequestSize(t *testing.T) {
	type args struct {
		maxBytes       units.Base2Bytes
		lengthRequired LengthRequiredBool
		reqLength      int
		chunkedRequest bool
		method         string
	}
	tests := []struct {
		name            string
		args            args
		expectedHandler mockHandler
		expectedStatus  int
	}{
		{
			name: "compliant request, length required",
			args: args{
				maxBytes:       100,
				lengthRequired: LengthRequired,
				reqLength:      90,
				chunkedRequest: false,
				method:         http.MethodPost,
			},
			expectedHandler: mockHandler{
				lengthHeader: "90",
				bytesRead:    90,
				handled:      true,
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "compliant request, length not required",
			args: args{
				maxBytes:       100,
				lengthRequired: LengthNotRequired,
				reqLength:      90,
				chunkedRequest: true,
				method:         http.MethodPost,
			},
			expectedHandler: mockHandler{
				lengthHeader:     "",
				bytesRead:        90,
				transferEncoding: []string{"chunked"},
				handled:          true,
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "compliant request, length required, but GET",
			args: args{
				maxBytes:       100,
				lengthRequired: LengthRequired,
				reqLength:      0,
				chunkedRequest: true,
				method:         http.MethodGet,
			},
			expectedHandler: mockHandler{
				lengthHeader: "",
				bytesRead:    0,
				handled:      true,
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "non-compliant too-long request, length required",
			args: args{
				maxBytes:       100,
				lengthRequired: LengthRequired,
				reqLength:      110,
				chunkedRequest: false,
				method:         http.MethodPost,
			},
			expectedHandler: mockHandler{},
			expectedStatus:  http.StatusRequestEntityTooLarge,
		},
		{
			name: "non-compliant chunked request, length required",
			args: args{
				maxBytes:       100,
				lengthRequired: LengthRequired,
				reqLength:      90,
				chunkedRequest: true,
				method:         http.MethodPost,
			},
			expectedHandler: mockHandler{},
			expectedStatus:  http.StatusLengthRequired,
		},
		{
			name: "non-compliant too-long request, length not required",
			args: args{
				maxBytes:       100,
				lengthRequired: LengthNotRequired,
				reqLength:      110,
				chunkedRequest: true,
				method:         http.MethodPost,
			},
			expectedHandler: mockHandler{
				lengthHeader:     "",
				bytesRead:        100,
				handled:          true,
				transferEncoding: []string{"chunked"},
			},
			// if the handler did anything with the data, this would probably be 400, because it would be truncated.
			// In an indeal world, we'd return StatusRequestEntityTooLarge, but that is proving tricky
			expectedStatus: http.StatusNoContent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mockHandler{}
			wrapped := MaxRequestSize(tt.args.maxBytes, tt.args.lengthRequired)(Metrics(http.HandlerFunc(h.mockHandler)))
			testServer := httptest.NewServer(wrapped)
			defer testServer.Close()
			body := make([]byte, tt.args.reqLength)
			rand.Read(body)
			var bodyReader io.Reader
			if tt.args.chunkedRequest {
				// a singleton multireader is enough to trick http.Request into 'chunked' mode
				bodyReader = io.MultiReader(bytes.NewBuffer(body))
			} else {
				bodyReader = bytes.NewBuffer(body)
			}
			req, reqErr := http.NewRequest(tt.args.method, testServer.URL, bodyReader)
			assert.NoError(t, reqErr)
			if !tt.args.chunkedRequest && tt.args.reqLength > 0 {
				req.ContentLength = int64(len(body))
			}
			res, err := http.DefaultClient.Do(req)
			if assert.NoError(t, err) {
				assert.Equal(t, tt.expectedHandler.handled, h.handled, "handler called")
				assert.Equal(t, tt.expectedStatus, res.StatusCode, "status code")
				assert.Equal(t, tt.expectedHandler.bytesRead, h.bytesRead, "bytes read")
				assert.Equal(t, tt.expectedHandler.lengthHeader, h.lengthHeader, "length header")
				assert.Equal(t, tt.expectedHandler.transferEncoding, h.transferEncoding, "transfer encoding")

				metricsName := fmt.Sprintf("http.server.%s.unknown_route.%d.", tt.args.method, res.StatusCode)
				timer := metrics.GetOrRegisterTimer(metricsName, metrics.DefaultRegistry)
				// ensure that we count the response once, regardless of the status code
				assert.Equal(t, int64(1), timer.Count())
				timer.Clear()
			}
		})
	}
}

func TestMaxRequestSizeRepeatedRequests(t *testing.T) {
	type args struct {
		reqLength int
	}
	tests := []struct {
		name           string
		args           args
		expectedStatus int
	}{
		{
			name: "compliant request",
			args: args{
				reqLength: 10,
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "compliant request again",
			args: args{
				reqLength: 90,
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "non-compliant too-long request, length required",
			args: args{
				reqLength: 110,
			},
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}
	h := mockHandler{}
	wrapped := MaxRequestSize(100, true)(http.HandlerFunc(h.mockHandler))
	testServer := httptest.NewServer(wrapped)
	defer testServer.Close()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := make([]byte, tt.args.reqLength)
			rand.Read(body)
			var bodyReader io.Reader
			bodyReader = bytes.NewBuffer(body)
			req, reqErr := http.NewRequest(http.MethodPost, testServer.URL, bodyReader)
			assert.NoError(t, reqErr)
			req.ContentLength = int64(len(body))
			res, err := http.DefaultClient.Do(req)
			if assert.NoError(t, err) {
				assert.Equal(t, tt.expectedStatus, res.StatusCode, "status code")
			}
		})
	}
}

func init() {
	fmetrics.Initialize(fmetrics.MetricsConfig{}, config.NewMode("test", "test"))
}

type mockHandler struct {
	transferEncoding []string
	lengthHeader     string
	bytesRead        int
	handled          bool
}

func (mh *mockHandler) mockHandler(w http.ResponseWriter, req *http.Request) {
	mh.handled = true
	mh.transferEncoding = req.TransferEncoding
	mh.lengthHeader = req.Header.Get("Content-Length")
	body, _ := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	mh.bytesRead = len(body)
	w.WriteHeader(http.StatusNoContent)
}
