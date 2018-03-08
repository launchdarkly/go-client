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

var defaultConfig = Config{
	SendEvents:            true,
	Capacity:              1000,
	FlushInterval:         1 * time.Hour,
	UserKeysCapacity:      1000,
	UserKeysFlushInterval: 1 * time.Hour,
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
	ep, st := createEventProcessor(defaultConfig)
	defer ep.close()

	user := NewUser("userkey")
	user.Name = strPtr("Red")
	ie := NewIdentifyEvent(user)
	ep.sendEvent(ie)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 1, len(output))

	ieo := output[0]
	expected := jsonMap(map[string]interface{}{
		"kind":         "identify",
		"creationDate": float64(ie.CreationDate),
		"user":         user,
	})
	assert.Equal(t, expected, ieo)
}

func TestIndividualFeatureEventIsQueuedWithIndexEvent(t *testing.T) {
	ep, st := createEventProcessor(defaultConfig)
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
	expected := jsonMap(map[string]interface{}{
		"kind":         "index",
		"creationDate": float64(fe.CreationDate),
		"user":         user,
	})
	assert.Equal(t, expected, ieo)

	feo := output[1]
	expected = jsonMap(map[string]interface{}{
		"kind":         "feature",
		"creationDate": float64(fe.CreationDate),
		"key":          flag.Key,
		"version":      float64(flag.Version),
		"value":        value,
		"default":      nil,
		"userKey":      *user.Key,
	})
	assert.Equal(t, expected, feo)
}

func TestDebugFlagIsSetIfFlagIsTemporarilyInDebugMode(t *testing.T) {
	ep, st := createEventProcessor(defaultConfig)
	defer ep.close()

	user := NewUser("userkey")
	user.Name = strPtr("Red")
	futureTime := now() + 1000000
	flag := FeatureFlag{
		Key:                  "flagkey",
		Version:              11,
		TrackEvents:          false,
		DebugEventsUntilDate: &futureTime,
	}
	variation := 1
	value := "value"
	fe := NewFeatureRequestEvent(flag.Key, &flag, user, &variation, value, nil, nil)
	ep.sendEvent(fe)

	output := flushAndGetEvents(ep, st)
	assert.Equal(t, 2, len(output))

	ieo := output[0]
	expected := jsonMap(map[string]interface{}{
		"kind":         "index",
		"creationDate": float64(fe.CreationDate),
		"user":         user,
	})
	assert.Equal(t, expected, ieo)

	feo := output[1]
	expected = jsonMap(map[string]interface{}{
		"kind":         "feature",
		"creationDate": float64(fe.CreationDate),
		"key":          flag.Key,
		"version":      float64(flag.Version),
		"value":        value,
		"default":      nil,
		"userKey":      *user.Key,
		"debug":        true,
	})
	assert.Equal(t, expected, feo)
}

func TestTwoFeatureEventsForSameUserGenerateOnlyOneIndexEvent(t *testing.T) {
	ep, st := createEventProcessor(defaultConfig)
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
	expected := jsonMap(map[string]interface{}{
		"kind":         "index",
		"creationDate": float64(fe1.CreationDate),
		"user":         user,
	})
	assert.Equal(t, expected, ieo)

	feo1 := output[1]
	expected = jsonMap(map[string]interface{}{
		"kind":         "feature",
		"creationDate": float64(fe1.CreationDate),
		"key":          flag1.Key,
		"version":      float64(flag1.Version),
		"value":        value,
		"default":      nil,
		"userKey":      *user.Key,
	})
	assert.Equal(t, expected, feo1)

	feo2 := output[2]
	expected = jsonMap(map[string]interface{}{
		"kind":         "feature",
		"creationDate": float64(fe2.CreationDate),
		"key":          flag2.Key,
		"version":      float64(flag2.Version),
		"value":        value,
		"default":      nil,
		"userKey":      *user.Key,
	})
	assert.Equal(t, expected, feo2)
}

func TestNonTrackedEventsAreSummarized(t *testing.T) {
	ep, st := createEventProcessor(defaultConfig)
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
	expected := jsonMap(map[string]interface{}{
		"kind":         "index",
		"creationDate": float64(fe1.CreationDate),
		"user":         user,
	})
	assert.Equal(t, expected, ieo)

	seo := output[1]
	expected = jsonMap(map[string]interface{}{
		"kind":      "summary",
		"startDate": float64(fe1.CreationDate),
		"endDate":   float64(fe2.CreationDate),
		"features": map[string]interface{}{
			flag1.Key: map[string]interface{}{
				"default": nil,
				"counters": []interface{}{
					map[string]interface{}{
						"version": float64(flag1.Version),
						"value":   value,
						"count":   1,
					},
				},
			},
			flag2.Key: map[string]interface{}{
				"default": nil,
				"counters": []interface{}{
					map[string]interface{}{
						"version": float64(flag2.Version),
						"value":   value,
						"count":   1,
					},
				},
			},
		},
	})
	assert.Equal(t, expected, seo)
}

func TestCustomEventIsQueuedWithUser(t *testing.T) {
	ep, st := createEventProcessor(defaultConfig)
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
	expected := jsonMap(map[string]interface{}{
		"kind":         "index",
		"creationDate": float64(ce.CreationDate),
		"user":         user,
	})
	assert.Equal(t, expected, ieo)

	ceo := jsonMap(output[1])
	expected = jsonMap(map[string]interface{}{
		"kind":         "custom",
		"creationDate": float64(ce.CreationDate),
		"key":          ce.Key,
		"data":         data,
		"userKey":      *user.Key,
	})
	assert.Equal(t, expected, ceo)
}

func TestSdkKeyIsSent(t *testing.T) {
	ep, st := createEventProcessor(defaultConfig)
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
