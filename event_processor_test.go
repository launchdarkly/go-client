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

const (
	sdkKey = "SDK_KEY"
)

type stubTransport struct {
	messageSent *http.Request
	statusCode  int
	error       error
}

func init() {
	sort.Strings(BuiltinAttributes)
}

func TestIdentifyEventIsQueued(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	ie := NewIdentifyEvent(epDefaultUser)
	ep.sendEvent(ie)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 1, len(output))

	ieo := output[0]
	expected := jsonMap(map[string]interface{}{
		"kind":         "identify",
		"creationDate": float64(ie.CreationDate),
		"key":          *epDefaultUser.Key,
		"user":         epDefaultUser,
	})
	assert.Equal(t, expected, ieo)
}

func TestUserDetailsAreScrubbedInIdentifyEvent(t *testing.T) {
	config := epDefaultConfig
	config.AllAttributesPrivate = true
	ep, st := createEventProcessor(config)
	defer ep.close()

	ie := NewIdentifyEvent(epDefaultUser)
	ep.sendEvent(ie)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 1, len(output))

	ieo := output[0]
	expected := jsonMap(map[string]interface{}{
		"kind":         "identify",
		"creationDate": float64(ie.CreationDate),
		"key":          "userKey",
		"user": map[string]interface{}{
			"key":          "userKey",
			"privateAttrs": []interface{}{"name"},
		},
	})
	assert.Equal(t, expected, ieo)
}

func TestFeatureEventIsSummarizedAndNotTrackedByDefault(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	flag := FeatureFlag{
		Key:     "flagkey",
		Version: 11,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, &variation, value, nil, nil)
	ep.sendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertIndexEventMatches(t, fe, epDefaultUser, output[0])

	assertSummaryEventHasCounter(t, flag, value, 1, output[1])
}

func TestIndividualFeatureEventIsQueuedWhenTrackEventsIsTrue(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	flag := FeatureFlag{
		Key:         "flagkey",
		Version:     11,
		TrackEvents: true,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, &variation, value, nil, nil)
	ep.sendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 3, len(output))

	assertIndexEventMatches(t, fe, epDefaultUser, output[0])

	assertFeatureEventMatches(t, fe, flag, value, epDefaultUser, false, false, output[1])

	assertSummaryEventHasCounter(t, flag, value, 1, output[2])
}

func TestFeatureEventCanContainInlineUser(t *testing.T) {
	config := epDefaultConfig
	config.InlineUsersInEvents = true
	ep, st := createEventProcessor(config)
	defer ep.close()

	flag := FeatureFlag{
		Key:         "flagkey",
		Version:     11,
		TrackEvents: true,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, epDefaultUser, &variation, value, nil, nil)
	ep.sendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertFeatureEventMatches(t, fe, flag, value, epDefaultUser, false, true, output[0])

	assertSummaryEventHasCounter(t, flag, value, 1, output[1])
}

func TestEventKindIsDebugIfFlagIsTemporarilyInDebugMode(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

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
	ep.sendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 3, len(output))

	assertIndexEventMatches(t, fe, epDefaultUser, output[0])

	assertFeatureEventMatches(t, fe, flag, value, epDefaultUser, true, false, output[1])

	assertSummaryEventHasCounter(t, flag, value, 1, output[2])
}

func TestTwoFeatureEventsForSameUserGenerateOnlyOneIndexEvent(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

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
	ep.sendEvent(fe1)
	ep.sendEvent(fe2)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 4, len(output))

	assertIndexEventMatches(t, fe1, epDefaultUser, output[0])

	assertFeatureEventMatches(t, fe1, flag1, value, epDefaultUser, false, false, output[1])

	assertFeatureEventMatches(t, fe2, flag2, value, epDefaultUser, false, false, output[2])

	assertSummaryEventHasCounter(t, flag1, value, 1, output[3])
	assertSummaryEventHasCounter(t, flag2, value, 1, output[3])
}

func TestNonTrackedEventsAreSummarized(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

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
	ep.sendEvent(fe1)
	ep.sendEvent(fe2)
	ep.sendEvent(fe3)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertIndexEventMatches(t, fe1, epDefaultUser, output[0])

	seo := output[1]
	assertSummaryEventHasCounter(t, flag1, value, 1, seo)
	assertSummaryEventHasCounter(t, flag2, value, 2, seo)
	assert.Equal(t, float64(fe1.CreationDate), seo["startDate"])
	assert.Equal(t, float64(fe2.CreationDate), seo["endDate"])
}

func TestCustomEventIsQueuedWithUser(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	data := map[string]interface{}{
		"thing": "stuff",
	}
	ce := NewCustomEvent("eventkey", epDefaultUser, data)
	ep.sendEvent(ce)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	assertIndexEventMatches(t, ce, epDefaultUser, output[0])

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
	defer ep.close()

	data := map[string]interface{}{
		"thing": "stuff",
	}
	ce := NewCustomEvent("eventkey", epDefaultUser, data)
	ep.sendEvent(ce)

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

func TestUserDetailsAreScrubbedInIndexEvent(t *testing.T) {
	config := epDefaultConfig
	config.AllAttributesPrivate = true
	ep, st := createEventProcessor(config)
	defer ep.close()

	ce := NewCustomEvent("eventkey", epDefaultUser, nil)
	ep.sendEvent(ce)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	ieo := output[0]
	expected := map[string]interface{}{
		"kind":         "index",
		"creationDate": float64(ce.CreationDate),
		"user": map[string]interface{}{
			"key":          "userKey",
			"privateAttrs": []interface{}{"name"},
		},
	}
	assert.Equal(t, expected, ieo)
}

func TestSendEventSyncReturnsErrorIfQueueFull(t *testing.T) {
	config := epDefaultConfig
	config.Capacity = 1
	ep, _ := createEventProcessor(config)
	defer ep.close()

	ie := NewIdentifyEvent(epDefaultUser)
	err := ep.sendEventSync(ie)
	assert.NoError(t, err)

	err = ep.sendEventSync(ie)
	assert.Error(t, err)
}

func TestSdkKeyIsSent(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	ie := NewIdentifyEvent(epDefaultUser)
	ep.sendEvent(ie)

	ep.flush()
	assert.Equal(t, sdkKey, st.messageSent.Header.Get("Authorization"))
}

func TestFlushReturnsHttpGeneralError(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	expectedErr := fmt.Errorf("problems")
	st.error = expectedErr

	ie := NewIdentifyEvent(epDefaultUser)
	ep.sendEvent(ie)

	err := ep.flush()
	assert.Equal(t, "Post /bulk: "+expectedErr.Error(), err.Error())
}

func TestFlushReturnsHttpResponseError(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	st.statusCode = 400

	ie := NewIdentifyEvent(epDefaultUser)
	ep.sendEvent(ie)

	err := ep.flush()
	assert.NoError(t, err)
}

func TestScrubUser(t *testing.T) {
	t.Run("private built-in attributes per user", func(t *testing.T) {
		user := User{
			Key:       strPtr("user-key"),
			FirstName: strPtr("sam"),
			LastName:  strPtr("smith"),
			Name:      strPtr("sammy"),
			Country:   strPtr("freedonia"),
			Avatar:    strPtr("my-avatar"),
			Ip:        strPtr("123.456.789"),
			Email:     strPtr("me@example.com"),
			Secondary: strPtr("abcdef"),
		}

		for _, attr := range BuiltinAttributes {
			user.PrivateAttributeNames = []string{attr}
			scrubbedUser := *scrubUser(user, false, nil)
			assert.Equal(t, []string{attr}, scrubbedUser.PrivateAttributes)
			scrubbedUser.PrivateAttributes = nil
			assert.NotEqual(t, user, scrubbedUser)
		}
	})

	t.Run("global private built-in attributes", func(t *testing.T) {
		user := User{
			Key:       strPtr("user-key"),
			FirstName: strPtr("sam"),
			LastName:  strPtr("smith"),
			Name:      strPtr("sammy"),
			Country:   strPtr("freedonia"),
			Avatar:    strPtr("my-avatar"),
			Ip:        strPtr("123.456.789"),
			Email:     strPtr("me@example.com"),
			Secondary: strPtr("abcdef"),
		}

		for _, attr := range BuiltinAttributes {
			scrubbedUser := *scrubUser(user, false, []string{attr})
			assert.Equal(t, []string{attr}, scrubbedUser.PrivateAttributes)
			scrubbedUser.PrivateAttributes = nil
			assert.NotEqual(t, user, scrubbedUser)
		}
	})

	t.Run("private custom attribute", func(t *testing.T) {
		userKey := "userKey"
		user := User{
			Key: &userKey,
			PrivateAttributeNames: []string{"my-secret-attr"},
			Custom: &map[string]interface{}{
				"my-secret-attr": "my secret value",
			}}

		scrubbedUser := *scrubUser(user, false, nil)

		assert.Equal(t, []string{"my-secret-attr"}, scrubbedUser.PrivateAttributes)
		assert.NotContains(t, *scrubbedUser.Custom, "my-secret-attr")
	})

	t.Run("all attributes private", func(t *testing.T) {
		userKey := "userKey"
		user := User{
			Key:       &userKey,
			FirstName: strPtr("sam"),
			LastName:  strPtr("smith"),
			Name:      strPtr("sammy"),
			Country:   strPtr("freedonia"),
			Avatar:    strPtr("my-avatar"),
			Ip:        strPtr("123.456.789"),
			Email:     strPtr("me@example.com"),
			Secondary: strPtr("abcdef"),
			Custom: &map[string]interface{}{
				"my-secret-attr": "my secret value",
			}}

		scrubbedUser := *scrubUser(user, true, nil)
		sort.Strings(scrubbedUser.PrivateAttributes)
		expectedAttributes := append(BuiltinAttributes, "my-secret-attr")
		sort.Strings(expectedAttributes)
		assert.Equal(t, expectedAttributes, scrubbedUser.PrivateAttributes)

		scrubbedUser.PrivateAttributes = nil
		assert.Equal(t, User{Key: &userKey, Custom: &map[string]interface{}{}}, scrubbedUser)
		assert.NotContains(t, *scrubbedUser.Custom, "my-secret-attr")
		assert.Nil(t, scrubbedUser.Name)
	})

	t.Run("anonymous attribute can't be private", func(t *testing.T) {
		userKey := "userKey"
		anon := true
		user := User{
			Key:       &userKey,
			Anonymous: &anon}

		scrubbedUser := *scrubUser(user, true, nil)
		assert.Equal(t, scrubbedUser, user)
	})
}

func strPtr(s string) *string {
	return &s
}

func jsonMap(o interface{}) map[string]interface{} {
	bytes, _ := json.Marshal(o)
	var result map[string]interface{}
	json.Unmarshal(bytes, &result)
	return result
}

func assertIndexEventMatches(t *testing.T, sourceEvent Event, user User, output map[string]interface{}) {
	expected := map[string]interface{}{
		"kind":         "index",
		"creationDate": float64(sourceEvent.GetBase().CreationDate),
		"user":         jsonMap(user),
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

func createEventProcessor(config Config) (*eventProcessor, *stubTransport) {
	transport := &stubTransport{
		statusCode: 200,
	}
	client := &http.Client{
		Transport: transport,
	}
	return newEventProcessor(sdkKey, config, client), transport
}

func flushAndGetEvents(ep *eventProcessor, st *stubTransport) (output []map[string]interface{}) {
	ep.flush()
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
	}
	return &resp, nil
}
