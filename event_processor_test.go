package ldclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var BuiltinAttributes = []string{
	"avatar",
	"country",
	"email",
	"firstName",
	"ip",
	"lastName",
	"name",
	"secondary",
}

var epDefaultConfig = Config{
	SendEvents:            true,
	Capacity:              1000,
	FlushInterval:         1 * time.Hour,
	Logger:                log.New(os.Stderr, "[LaunchDarkly]", log.LstdFlags),
	UserKeysCapacity:      1000,
	UserKeysFlushInterval: 1 * time.Hour,
}

var epDefaultUser = User{
	Key:  strPtr("userKey"),
	Name: strPtr("Red"),
}

var userJson = map[string]interface{}{"key": "userKey", "name": "Red"}
var filteredUserJson = map[string]interface{}{"key": "userKey", "privateAttrs": []interface{}{"name"}}

const (
	sdkKey = "SDK_KEY"
)

type stubTransport struct {
	messageSent *http.Request
	statusCode  int
	serverTime  uint64
	error       error
}

var epoch = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

func init() {
	sort.Strings(BuiltinAttributes)
}

func TestIdentifyEventIsQueued(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	ie := NewIdentifyEvent(epDefaultUser)
	ep.SendEvent(ie)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 1, len(output))

	ieo := output[0]
	expected := jsonMap(map[string]interface{}{
		"kind":         "identify",
		"creationDate": float64(ie.CreationDate),
		"key":          *epDefaultUser.Key,
		"user":         userJson,
	})
	assert.Equal(t, expected, ieo)
}

func TestUserDetailsAreScrubbedInIdentifyEvent(t *testing.T) {
	config := epDefaultConfig
	config.AllAttributesPrivate = true
	ep, st := createEventProcessor(config)
	defer ep.Close()

	ie := NewIdentifyEvent(epDefaultUser)
	ep.SendEvent(ie)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 1, len(output))

	ieo := output[0]
	expected := jsonMap(map[string]interface{}{
		"kind":         "identify",
		"creationDate": float64(ie.CreationDate),
		"key":          "userKey",
		"user":         filteredUserJson,
	})
	assert.Equal(t, expected, ieo)
}

func TestFeatureEventIsSummarizedAndNotTrackedByDefault(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	flag := FeatureFlag{
		Key:     "flagkey",
		Version: 11,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, &variation, value, nil, nil)
	ep.SendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertIndexEventMatches(t, fe, userJson, output[0])

	assertSummaryEventHasCounter(t, flag, value, 1, output[1])
}

func TestIndividualFeatureEventIsQueuedWhenTrackEventsIsTrue(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	flag := FeatureFlag{
		Key:         "flagkey",
		Version:     11,
		TrackEvents: true,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, &variation, value, nil, nil)
	ep.SendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 3, len(output))

	assertIndexEventMatches(t, fe, userJson, output[0])

	assertFeatureEventMatches(t, fe, flag, value, epDefaultUser, false, false, output[1])

	assertSummaryEventHasCounter(t, flag, value, 1, output[2])
}

func TestUserDetailsAreScrubbedInIndexEvent(t *testing.T) {
	config := epDefaultConfig
	config.AllAttributesPrivate = true
	ep, st := createEventProcessor(config)
	defer ep.Close()

	flag := FeatureFlag{
		Key:         "flagkey",
		Version:     11,
		TrackEvents: true,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, &variation, value, nil, nil)
	ep.SendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 3, len(output))

	assertIndexEventMatches(t, fe, filteredUserJson, output[0])

	assertFeatureEventMatches(t, fe, flag, value, epDefaultUser, false, false, output[1])

	assertSummaryEventHasCounter(t, flag, value, 1, output[2])
}

func TestFeatureEventCanContainInlineUser(t *testing.T) {
	config := epDefaultConfig
	config.InlineUsersInEvents = true
	ep, st := createEventProcessor(config)
	defer ep.Close()

	flag := FeatureFlag{
		Key:         "flagkey",
		Version:     11,
		TrackEvents: true,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, &variation, value, nil, nil)
	ep.SendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertFeatureEventMatches(t, fe, flag, value, epDefaultUser, false, true, output[0])

	assertSummaryEventHasCounter(t, flag, value, 1, output[1])
}

func TestEventKindIsDebugIfFlagIsTemporarilyInDebugMode(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	futureTime := now() + 1000000
	flag := FeatureFlag{
		Key:                  "flagkey",
		Version:              11,
		TrackEvents:          false,
		DebugEventsUntilDate: &futureTime,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, &variation, value, nil, nil)
	ep.SendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 3, len(output))

	assertIndexEventMatches(t, fe, userJson, output[0])

	assertFeatureEventMatches(t, fe, flag, value, epDefaultUser, true, false, output[1])

	assertSummaryEventHasCounter(t, flag, value, 1, output[2])
}

func TestDebugModeExpiresBasedOnCurrentTimeIfCurrentTimeIsLater(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	// Pick a server time that is somewhat behind the client time
	serverTime := now() - 20000
	st.serverTime = serverTime

	// Send and flush an event we don't care about, just to set the last server time
	ie := NewIdentifyEvent(User{Key: strPtr("otherUser")})
	ep.SendEvent(ie)
	ep.Flush()

	// Now send an event with debug mode on, with a "debug until" time that is further in
	// the future than the server time, but in the past compared to the client.
	debugUntil := serverTime + 1000
	flag := FeatureFlag{
		Key:                  "flagkey",
		Version:              11,
		TrackEvents:          false,
		DebugEventsUntilDate: &debugUntil,
	}
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, nil, nil, nil, nil)
	ep.SendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertIndexEventMatches(t, fe, userJson, output[0])

	// should get a summary event only, not a full feature event
	assertSummaryEventHasCounter(t, flag, nil, 1, output[1])
}

func TestDebugModeExpiresBasedOnServerTimeIfServerTimeIsLater(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	// Pick a server time that is somewhat ahead of the client time
	serverTime := now() + 20000
	st.serverTime = serverTime

	// Send and flush an event we don't care about, just to set the last server time
	ie := NewIdentifyEvent(User{Key: strPtr("otherUser")})
	ep.SendEvent(ie)
	ep.Flush()

	// Now send an event with debug mode on, with a "debug until" time that is further in
	// the future than the client time, but in the past compared to the server.
	debugUntil := serverTime - 1000
	flag := FeatureFlag{
		Key:                  "flagkey",
		Version:              11,
		TrackEvents:          false,
		DebugEventsUntilDate: &debugUntil,
	}
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, nil, nil, nil, nil)
	ep.SendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertIndexEventMatches(t, fe, userJson, output[0])

	// should get a summary event only, not a full feature event
	assertSummaryEventHasCounter(t, flag, nil, 1, output[1])
}

func TestTwoFeatureEventsForSameUserGenerateOnlyOneIndexEvent(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	flag1 := FeatureFlag{
		Key:         "flagkey1",
		Version:     11,
		TrackEvents: true,
	}
	flag2 := FeatureFlag{
		Key:         "flagkey2",
		Version:     22,
		TrackEvents: true,
	}
	variation := 1
	value := "value"
	fe1 := NewFeatureRequestEvent(flag1.Key, &flag1, epDefaultUser, &variation, value, nil, nil)
	fe2 := NewFeatureRequestEvent(flag2.Key, &flag2, epDefaultUser, &variation, value, nil, nil)
	ep.SendEvent(fe1)
	ep.SendEvent(fe2)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 4, len(output))

	assertIndexEventMatches(t, fe1, userJson, output[0])

	assertFeatureEventMatches(t, fe1, flag1, value, epDefaultUser, false, false, output[1])

	assertFeatureEventMatches(t, fe2, flag2, value, epDefaultUser, false, false, output[2])

	assertSummaryEventHasCounter(t, flag1, value, 1, output[3])
	assertSummaryEventHasCounter(t, flag2, value, 1, output[3])
}

func TestNonTrackedEventsAreSummarized(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	flag1 := FeatureFlag{
		Key:         "flagkey1",
		Version:     11,
		TrackEvents: false,
	}
	flag2 := FeatureFlag{
		Key:         "flagkey2",
		Version:     22,
		TrackEvents: false,
	}
	variation := 1
	value := "value"
	fe1 := NewFeatureRequestEvent(flag1.Key, &flag1, epDefaultUser, &variation, value, nil, nil)
	fe2 := NewFeatureRequestEvent(flag2.Key, &flag2, epDefaultUser, &variation, value, nil, nil)
	fe3 := NewFeatureRequestEvent(flag2.Key, &flag2, epDefaultUser, &variation, value, nil, nil)
	ep.SendEvent(fe1)
	ep.SendEvent(fe2)
	ep.SendEvent(fe3)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertIndexEventMatches(t, fe1, userJson, output[0])

	seo := output[1]
	assertSummaryEventHasCounter(t, flag1, value, 1, seo)
	assertSummaryEventHasCounter(t, flag2, value, 2, seo)
	assert.Equal(t, float64(fe1.CreationDate), seo["startDate"])
	assert.Equal(t, float64(fe2.CreationDate), seo["endDate"])
}

func TestCustomEventIsQueuedWithUser(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	data := map[string]interface{}{
		"thing": "stuff",
	}
	ce := NewCustomEvent("eventkey", epDefaultUser, data)
	ep.SendEvent(ce)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertIndexEventMatches(t, ce, userJson, output[0])

	ceo := output[1]
	expected := map[string]interface{}{
		"kind":         "custom",
		"creationDate": float64(ce.CreationDate),
		"key":          ce.Key,
		"data":         data,
		"userKey":      *epDefaultUser.Key,
	}
	assert.Equal(t, expected, ceo)
}

func TestCustomEventCanContainInlineUser(t *testing.T) {
	config := epDefaultConfig
	config.InlineUsersInEvents = true
	ep, st := createEventProcessor(config)
	defer ep.Close()

	data := map[string]interface{}{
		"thing": "stuff",
	}
	ce := NewCustomEvent("eventkey", epDefaultUser, data)
	ep.SendEvent(ce)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 1, len(output))

	ceo := output[0]
	expected := map[string]interface{}{
		"kind":         "custom",
		"creationDate": float64(ce.CreationDate),
		"key":          ce.Key,
		"data":         data,
		"user":         jsonMap(epDefaultUser),
	}
	assert.Equal(t, expected, ceo)
}

func TestNothingIsSentIfThereAreNoEvents(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()
	ep.Flush()

	assert.Nil(t, st.messageSent)
}

func TestSdkKeyIsSent(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	ie := NewIdentifyEvent(epDefaultUser)
	ep.SendEvent(ie)

	ep.Flush()
	assert.Equal(t, sdkKey, st.messageSent.Header.Get("Authorization"))
}

func TestFlushReturnsHttpGeneralError(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	expectedErr := fmt.Errorf("problems")
	st.error = expectedErr

	ie := NewIdentifyEvent(epDefaultUser)
	ep.SendEvent(ie)

	err := ep.Flush()
	assert.Equal(t, "Post /bulk: "+expectedErr.Error(), err.Error())
}

func TestFlushReturnsHttpResponseError(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.Close()

	st.statusCode = 400

	ie := NewIdentifyEvent(epDefaultUser)
	ep.SendEvent(ie)

	err := ep.Flush()
	assert.Equal(t, "Unexpected response code: 400 when accessing URL: /bulk", err.Error())
}

func jsonMap(o interface{}) map[string]interface{} {
	bytes, _ := json.Marshal(o)
	var result map[string]interface{}
	json.Unmarshal(bytes, &result)
	return result
}

func assertIndexEventMatches(t *testing.T, sourceEvent Event, encodedUser map[string]interface{}, output map[string]interface{}) {
	expected := map[string]interface{}{
		"kind":         "index",
		"creationDate": float64(sourceEvent.GetBase().CreationDate),
		"user":         encodedUser,
	}
	assert.Equal(t, expected, output)
}

func assertFeatureEventMatches(t *testing.T, sourceEvent FeatureRequestEvent, flag FeatureFlag,
	value interface{}, user User, debug bool, inlineUser bool, output map[string]interface{}) {
	kind := "feature"
	if debug {
		kind = "debug"
	}
	expected := map[string]interface{}{
		"kind":         kind,
		"creationDate": float64(sourceEvent.CreationDate),
		"key":          flag.Key,
		"version":      float64(flag.Version),
		"value":        value,
		"default":      nil,
	}
	if inlineUser {
		expected["user"] = jsonMap(user)
	} else {
		expected["userKey"] = *user.Key
	}
	assert.Equal(t, expected, output)
}

func assertSummaryEventHasFlag(t *testing.T, flag FeatureFlag, output map[string]interface{}) {
	assert.Equal(t, "summary", output["kind"])
	flags, _ := output["features"].(map[string]interface{})
	assert.NotNil(t, flags)
	assert.NotNil(t, flags[flag.Key])
}

func assertSummaryEventHasCounter(t *testing.T, flag FeatureFlag, value interface{}, count int, output map[string]interface{}) {
	assertSummaryEventHasFlag(t, flag, output)
	f, _ := output["features"].(map[string]interface{})[flag.Key].(map[string]interface{})
	assert.NotNil(t, f)
	expected := map[string]interface{}{
		"value":   value,
		"count":   float64(count),
		"version": float64(flag.Version),
	}
	assert.Contains(t, f["counters"], expected)
}

func createEventProcessor(config Config) (*defaultEventProcessor, *stubTransport) {
	transport := &stubTransport{
		statusCode: 200,
	}
	client := &http.Client{
		Transport: transport,
	}
	return newDefaultEventProcessor(sdkKey, config, client), transport
}

func flushAndGetEvents(ep *defaultEventProcessor, st *stubTransport) (output []map[string]interface{}) {
	ep.Flush()
	if st.messageSent == nil || st.messageSent.Body == nil {
		return
	}
	bytes, err := ioutil.ReadAll(st.messageSent.Body)
	if err != nil {
		return
	}
	json.Unmarshal(bytes, &output)
	return
}

func (t *stubTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.messageSent = request
	if t.error != nil {
		return nil, t.error
	}
	resp := http.Response{
		StatusCode: t.statusCode,
		Header:     make(http.Header),
		Request:    request,
	}
	if t.serverTime != 0 {
		ts := epoch.Add(time.Duration(t.serverTime) * time.Millisecond)
		resp.Header.Add("Date", ts.Format(http.TimeFormat))
	}
	return &resp, nil
}
