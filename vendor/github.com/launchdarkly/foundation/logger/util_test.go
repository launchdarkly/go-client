package logger

import (
	"net/http"
	"testing"

	"github.com/launchdarkly/foundation/core"
	"github.com/stretchr/testify/assert"
)

func TestReqMsgGet(t *testing.T) {
	r, _ := http.NewRequest("GET", "", nil)
	r.RequestURI = "/path?query=param"
	r.Header.Add("User-Agent", "user agent")
	r.Header.Add(core.REQ_ID_HDR, "requestId")
	assert.Equal(t, "[GET /path?query=param User-Agent: user agent Request Id: requestId]", ReqMsg(r))
}

func TestReqMsgPost(t *testing.T) {
	r, _ := http.NewRequest("POST", "", nil)
	r.RequestURI = "/path?query=param"
	r.Header.Add("User-Agent", "user agent")
	r.Header.Add(core.REQ_ID_HDR, "requestId")
	assert.Equal(t, "[POST /path?query=param User-Agent: user agent Request Id: requestId Body: 0 bytes]", ReqMsg(r))
}

func TestReqMsgPut(t *testing.T) {
	r, _ := http.NewRequest("PUT", "", nil)
	r.RequestURI = "/path?query=param"
	r.Header.Add("User-Agent", "user agent")
	r.Header.Add(core.REQ_ID_HDR, "requestId")
	assert.Equal(t, "[PUT /path?query=param User-Agent: user agent Request Id: requestId Body: 0 bytes]", ReqMsg(r))
}

func TestReqMsgNil(t *testing.T) {
	assert.Equal(t, "[nil request]", ReqMsg(nil))
}
