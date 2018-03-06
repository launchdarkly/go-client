package ldclient

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
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
	SendEvents:       true,
	Capacity:         1000,
	FlushInterval:    1 * time.Hour,
	UserKeysCapacity: 1000,
}

const (
	sdkKey = "SDK_KEY"
)

type stubTransport struct {
	messageSent *http.Request
}

func init() {
	sort.Strings(BuiltinAttributes)
}

func TestIdentifyEventIsQueued(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	user := NewUser("userkey")
	user.Name = strPtr("Red")
	ie := NewIdentifyEvent(user)
	ep.sendEvent(ie)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 1, len(output))

	ieo := jsonMap(output[0])
	assert.Equal(t, "identify", ieo["kind"])
	assert.Equal(t, float64(ie.CreationDate), ieo["creationDate"])
	assert.Equal(t, jsonMap(user), ieo["user"])
}

func TestIndividualFeatureEventIsQueuedWithIndexEvent(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	user := NewUser("userkey")
	user.Name = strPtr("Red")
	flag := FeatureFlag{
		Key:         "flagkey",
		Version:     11,
		TrackEvents: true,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, user, &variation, value, nil, nil)
	ep.sendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	ieo := output[0]
	assert.Equal(t, "index", ieo["kind"])
	assert.Equal(t, jsonMap(user), ieo["user"])

	feo := output[1]
	assert.Equal(t, "feature", feo["kind"])
	assert.Equal(t, float64(fe.CreationDate), feo["creationDate"])
	assert.Equal(t, flag.Key, feo["key"])
	assert.Equal(t, float64(flag.Version), feo["version"])
	assert.Equal(t, value, feo["value"])
	assert.Equal(t, *user.Key, feo["userKey"])
}

func TestTwoFeatureEventsForSameUserGenerateOnlyOneIndexEvent(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	user := NewUser("userkey")
	user.Name = strPtr("Red")
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
	fe1 := NewFeatureRequestEvent(flag1.Key, &flag1, user, &variation, value, nil, nil)
	fe2 := NewFeatureRequestEvent(flag2.Key, &flag2, user, &variation, value, nil, nil)
	ep.sendEvent(fe1)
	ep.sendEvent(fe2)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 3, len(output))

	ieo := output[0]
	assert.Equal(t, "index", ieo["kind"])
	assert.Equal(t, jsonMap(user), ieo["user"])

	feo1 := output[1]
	assert.Equal(t, "feature", feo1["kind"])
	assert.Equal(t, float64(fe1.CreationDate), feo1["creationDate"])
	assert.Equal(t, flag1.Key, feo1["key"])
	assert.Equal(t, float64(flag1.Version), feo1["version"])
	assert.Equal(t, value, feo1["value"])
	assert.Equal(t, *user.Key, feo1["userKey"])

	feo2 := output[2]
	assert.Equal(t, "feature", feo2["kind"])
	assert.Equal(t, float64(fe2.CreationDate), feo2["creationDate"])
	assert.Equal(t, flag2.Key, feo2["key"])
	assert.Equal(t, float64(flag2.Version), feo2["version"])
	assert.Equal(t, value, feo2["value"])
	assert.Equal(t, *user.Key, feo2["userKey"])
}

func TestNonTrackedEventsAreSummarized(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	user := NewUser("userkey")
	user.Name = strPtr("Red")
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
	fe1 := NewFeatureRequestEvent(flag1.Key, &flag1, user, &variation, value, nil, nil)
	fe2 := NewFeatureRequestEvent(flag2.Key, &flag2, user, &variation, value, nil, nil)
	ep.sendEvent(fe1)
	ep.sendEvent(fe2)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	ieo := output[0]
	assert.Equal(t, "index", ieo["kind"])
	assert.Equal(t, jsonMap(user), ieo["user"])

	seo := output[1]
	assert.Equal(t, "summary", seo["kind"])
	assert.Equal(t, float64(fe1.CreationDate), seo["startDate"])
	assert.Equal(t, float64(fe2.CreationDate), seo["endDate"])
	counters := seo["counters"].([]interface{})
	assert.Equal(t, 2, len(counters))
}

func TestCustomEventIsQueuedWithUser(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	user := NewUser("userkey")
	user.Name = strPtr("Red")
	data := map[string]interface{}{
		"thing": "stuff",
	}
	ce := NewCustomEvent("eventkey", user, data)
	ep.sendEvent(ce)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	ieo := jsonMap(output[0])
	assert.Equal(t, "index", ieo["kind"])
	assert.Equal(t, jsonMap(user), ieo["user"])

	ceo := jsonMap(output[1])
	assert.Equal(t, "custom", ceo["kind"])
	assert.Equal(t, float64(ce.CreationDate), ceo["creationDate"])
	assert.Equal(t, ce.Key, ceo["key"])
	assert.Equal(t, data, ceo["data"])
	assert.Equal(t, *user.Key, ceo["userKey"])
}

func TestSendEventSyncReturnsErrorIfQueueFull(t *testing.T) {
	config := epDefaultConfig
	config.Capacity = 1
	ep, _ := createEventProcessor(config)
	defer ep.close()

	user := NewUser("userkey")
	user.Name = strPtr("Red")
	ie := NewIdentifyEvent(user)
	err := ep.sendEventSync(ie)
	assert.NoError(t, err)

	err = ep.sendEventSync(ie)
	assert.Error(t, err)
}

func TestSdkKeyIsSent(t *testing.T) {
	ep, st := createEventProcessor(epDefaultConfig)
	defer ep.close()

	user := NewUser("userkey")
	user.Name = strPtr("Red")
	ie := NewIdentifyEvent(user)
	ep.sendEvent(ie)

	ep.flush()
	assert.Equal(t, sdkKey, st.messageSent.Header.Get("Authorization"))
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

func createEventProcessor(config Config) (*eventProcessor, *stubTransport) {
	transport := &stubTransport{}
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
	resp := http.Response{
		StatusCode: 200,
	}
	return &resp, nil
}
