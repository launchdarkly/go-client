package middleware

import (
	"io"
	"net/http"
	"strconv"

	"github.com/alecthomas/units"
)

type LengthRequiredBool bool

const (
	LengthRequired    LengthRequiredBool = true
	LengthNotRequired LengthRequiredBool = false
)

func MaxRequestSize(maxBytes units.Base2Bytes, lengthRequired LengthRequiredBool) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contentLengthVal := r.Header.Get("Content-Length")
			if contentLength, err := strconv.ParseInt(contentLengthVal, 10, 64); err == nil {
				if contentLength > int64(maxBytes) {
					Metrics(staticResponseHandler(http.StatusRequestEntityTooLarge)).ServeHTTP(w, r)
					return
				}
				// ensure we only read the stated length
				r.Body = newLimitedReadCloser(r.Body, int64(units.Base2Bytes(contentLength)))
			} else {
				if lengthRequired {
					// don't require length on methods that don't have a body (if there was a body,
					// the limitedReadCloser and Content-Length check above still protect us)
					if methodShouldHaveLength(r.Method) {
						Metrics(staticResponseHandler(http.StatusLengthRequired)).ServeHTTP(w, r)
						return
					}
				}
				// ensure we only read up to the max length
				r.Body = newLimitedReadCloser(r.Body, int64(maxBytes))
			}
			h.ServeHTTP(w, r)
		})
	}
}

func staticResponseHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})
}

func methodShouldHaveLength(method string) bool {
	// According to https://tools.ietf.org/html/rfc7231, GET, HEAD, DELETE, OPTIONS, CONNECT
	// have no defined sematics for a payload. TRACE "MUST NOT" include a payload.
	return method != http.MethodGet &&
		method != http.MethodHead &&
		method != http.MethodDelete &&
		method != http.MethodOptions &&
		method != http.MethodConnect &&
		method != http.MethodTrace
}

func newLimitedReadCloser(rc io.ReadCloser, maxBytes int64) limitedReadCloser {
	return limitedReadCloser{
		underlying: rc,
		reader:     io.LimitReader(rc, maxBytes),
	}
}

type limitedReadCloser struct {
	underlying io.ReadCloser
	reader     io.Reader
}

func (wr limitedReadCloser) Close() error {
	return wr.underlying.Close()
}

func (wr limitedReadCloser) Read(p []byte) (int, error) {
	n, err := wr.reader.Read(p)
	if err == io.EOF {
		wr.Close()
	}
	return n, err
}
