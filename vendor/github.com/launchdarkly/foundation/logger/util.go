package logger

import (
	"fmt"
	"net/http"

	"github.com/launchdarkly/foundation/core"
)

//Creates a concise message from a server-side http.Request object suitable for inserting into log statements
func ReqMsg(req *http.Request) string {
	if req == nil {
		return "[nil request]"
	}
	msg := fmt.Sprintf("[%s %s User-Agent: %s Request Id: %s",
		req.Method, req.RequestURI, req.Header.Get("User-Agent"), req.Header.Get(core.REQ_ID_HDR))
	if req.Method == "POST" || req.Method == "PUT" {
		msg += fmt.Sprintf(" Body: %d bytes", req.ContentLength)
	}
	return msg + "]"
}
