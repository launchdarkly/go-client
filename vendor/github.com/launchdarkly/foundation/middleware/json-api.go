package middleware

import (
	"mime"
	"net/http"

	"github.com/launchdarkly/foundation/ferror"
	"github.com/launchdarkly/foundation/logger"
)

func isApplicationJson(s string) bool {
	mt, _, err := mime.ParseMediaType(s)

	if err == nil && mt == "application/json" {
		return true
	}

	return false
}

func isApplicationJsonPatch(s string) bool {
	mt, _, err := mime.ParseMediaType(s)

	if err == nil && mt == "application/json-patch+json" {
		return true
	}

	return false
}

var JSONOnly = func(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For all requests, set the Content-Type header to application/json
		w.Header().Set("Content-Type", "application/json")
		contentType := r.Header.Get("Content-Type")
		reqId := r.Header.Get(REQ_ID_HDR)

		// For PUT or POST requests, ensure that the Content-Type header is set to application/json
		if (r.Method == "POST" || r.Method == "PUT") && !isApplicationJson(contentType) {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			err := ferror.NewInvalidRequest("Content-Type must be application/json", http.StatusUnsupportedMediaType, nil, reqId)
			w.Write(err.Json())
			return
		} else if r.Method == "PATCH" && !isApplicationJson(contentType) && !isApplicationJsonPatch(contentType) {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			err := ferror.NewInvalidRequest("Content-Type must be application/json", http.StatusUnsupportedMediaType, nil, reqId)
			w.Write(err.Json())
			return
		}

		// TODO: (someday) If the Accept header is present, assert that application/json is acceptable

		h.ServeHTTP(w, r)
	})
}

func LogAndRenderJsonServerError(w http.ResponseWriter, req *http.Request, ferr *ferror.Error) {
	if ferr != nil && ferr.StatusCode >= 500 {
		logger.Error.Printf("Returning %d for request: %s %s error: %s", ferr.StatusCode, req.Method, req.URL.String(), ferr.Log())
	}
	w.WriteHeader(ferr.StatusCode)
	w.Write(ferr.Json())
}
