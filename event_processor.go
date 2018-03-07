package ldclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type eventProcessor struct {
	queue      []interface{}
	sdkKey     string
	config     Config
	closed     bool
	mu         *sync.Mutex
	client     *http.Client
	closer     chan struct{}
	summarizer *eventSummarizer
}

// Serializable form of a feature request event. This differs from the event that was
// passed in to us in that it has a user key instead of a user object, and it only shows
// the flag value, not the variation index.
type featureRequestEventOutput struct {
	Kind         string      `json:"kind"`
	CreationDate uint64      `json:"creationDate"`
	Key          string      `json:"key"`
	UserKey      string      `json:"userKey"`
	Value        interface{} `json:"value"`
	Default      interface{} `json:"default"`
	Version      *int        `json:"version,omitempty"`
	PrereqOf     *string     `json:"prereqOf,omitempty"`
}

// Serializable form of an identify event.
type identifyEventOutput struct {
	Kind         string `json:"kind"`
	CreationDate uint64 `json:"creationDate"`
	User         *User  `json:"user"`
}

// Serializable form of a custom event. It has a user key instead of a user object.
type customEventOutput struct {
	Kind         string      `json:"kind"`
	CreationDate uint64      `json:"creationDate"`
	Key          string      `json:"key"`
	UserKey      string      `json:"userKey"`
	Data         interface{} `json:"data"`
}

// Serializable form of an index event. This is not generated by an explicit client call,
// but is created automatically whenever we see a user we haven't seen before in a feature
// request event or custom event.
type indexEventOutput struct {
	Kind         string `json:"kind"`
	CreationDate uint64 `json:"creationDate"`
	User         *User  `json:"user"`
}

// Serializable form of a summary event, containing data generated by EventSummarizer.
type summaryEventOutput struct {
	summaryOutput
	Kind string `json:"kind"`
}

const (
	FEATURE_REQUEST_EVENT = "feature"
	CUSTOM_EVENT          = "custom"
	IDENTIFY_EVENT        = "identify"
	INDEX_EVENT           = "index"
	SUMMARY_EVENT         = "summary"
)

func newEventProcessor(sdkKey string, config Config, client *http.Client) *eventProcessor {
	if client == nil {
		client = &http.Client{}
	}

	res := &eventProcessor{
		queue:      make([]interface{}, 0),
		sdkKey:     sdkKey,
		config:     config,
		client:     client,
		closer:     make(chan struct{}),
		mu:         &sync.Mutex{},
		summarizer: NewEventSummarizer(config),
	}

	go func() {
		if err := recover(); err != nil {
			res.config.Logger.Printf("Unexpected panic in event processing thread: %+v", err)
		}

		flushInterval := config.FlushInterval
		if flushInterval <= 0 {
			flushInterval = DefaultConfig.FlushInterval
		}
		userKeysFlushInterval := config.UserKeysFlushInterval
		if userKeysFlushInterval <= 0 {
			userKeysFlushInterval = DefaultConfig.UserKeysFlushInterval
		}
		flushTicker := time.NewTicker(flushInterval)
		usersResetTicker := time.NewTicker(userKeysFlushInterval)
		for {
			select {
			case <-flushTicker.C:
				res.flush()
			case <-usersResetTicker.C:
				res.summarizer.resetUsers()
			case <-res.closer:
				flushTicker.Stop()
				usersResetTicker.Stop()
				return
			}
		}
	}()

	return res
}

func (ep *eventProcessor) close() {
	ep.mu.Lock()
	closed := ep.closed
	ep.closed = true
	ep.mu.Unlock()

	if !closed {
		close(ep.closer)
		ep.flush()
	}
}

func (ep *eventProcessor) flush() {
	uri := ep.config.EventsUri + "/bulk"
	ep.mu.Lock()

	if len(ep.queue) == 0 || ep.closed {
		ep.mu.Unlock()
		return
	}

	events := ep.queue
	ep.queue = make([]interface{}, 0)
	ep.mu.Unlock()

	summaryData := ep.summarizer.flush()
	if len(summaryData.Counters) > 0 {
		se := summaryEventOutput{
			summaryOutput: summaryData,
			Kind:          SUMMARY_EVENT,
		}
		// note that the queue size limit does not include the summary event, if any
		events = append(events, se)
	}

	payload, marshalErr := json.Marshal(events)

	if marshalErr != nil {
		ep.config.Logger.Printf("Unexpected error marshalling event json: %+v", marshalErr)
	}

	req, reqErr := http.NewRequest("POST", uri, bytes.NewReader(payload))

	if reqErr != nil {
		ep.config.Logger.Printf("Unexpected error while creating event request: %+v", reqErr)
	}

	req.Header.Add("Authorization", ep.sdkKey)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "GoClient/"+Version)

	resp, respErr := ep.client.Do(req)

	defer func() {
		if resp != nil && resp.Body != nil {
			ioutil.ReadAll(resp.Body)
			resp.Body.Close()
		}
	}()

	if respErr != nil {
		ep.config.Logger.Printf("Unexpected error while sending events: %+v", respErr)
		return
	}
	err := checkStatusCode(resp.StatusCode, uri)
	if err != nil {
		ep.config.Logger.Printf("Unexpected status code when sending events: %+v", err)
		if err != nil && err.Code == 401 {
			ep.config.Logger.Printf("Received 401 error, no further events will be posted since SDK key is invalid")
			ep.mu.Lock()
			ep.closed = true
			ep.mu.Unlock()
		}
	} else {
		t, err := http.ParseTime(resp.Header.Get("Date"))
		if err == nil {
			ep.summarizer.setLastKnownPastTime(toUnixMillis(t))
		}
	}
}

func (ep *eventProcessor) sendEvent(evt Event) error {
	if !ep.config.SendEvents {
		return nil
	}

	creationDate := evt.GetBase().CreationDate
	var userKey string
	var newUser *User
	if userKey, newUser = ep.dedupUser(evt); newUser != nil {
		indexEvent := indexEventOutput{
			Kind:         INDEX_EVENT,
			CreationDate: creationDate,
			User:         newUser,
		}
		if err := ep.queueEventOutput(indexEvent); err != nil {
			return err
		}
	}

	if ep.summarizer.summarizeEvent(evt) {
		return nil
	}

	if ep.config.SamplingInterval > 0 && rand.Int31n(ep.config.SamplingInterval) != 0 {
		return nil
	}

	var eventOutput interface{}
	switch evt := evt.(type) {
	case FeatureRequestEvent:
		fe := featureRequestEventOutput{
			Kind:         FEATURE_REQUEST_EVENT,
			CreationDate: creationDate,
			Key:          evt.Key,
			UserKey:      userKey,
			Value:        evt.Value,
			Default:      evt.Default,
			Version:      evt.Version,
			PrereqOf:     evt.PrereqOf,
		}
		eventOutput = fe
	case CustomEvent:
		ce := customEventOutput{
			Kind:         CUSTOM_EVENT,
			CreationDate: creationDate,
			Key:          evt.Key,
			UserKey:      userKey,
			Data:         evt.Data,
		}
		eventOutput = ce
	case IdentifyEvent:
		user := scrubUser(evt.User, ep.config.AllAttributesPrivate, ep.config.PrivateAttributeNames)
		ep.summarizer.noticeUser(user)
		ie := identifyEventOutput{
			Kind:         IDENTIFY_EVENT,
			CreationDate: creationDate,
			User:         user,
		}
		eventOutput = ie
	default:
		return errors.New("unknown event type")
	}

	return ep.queueEventOutput(eventOutput)
}

func (ep *eventProcessor) queueEventOutput(eventOutput interface{}) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.closed {
		return nil
	}
	if len(ep.queue) >= ep.config.Capacity {
		return errors.New("Exceeded event queue capacity. Increase capacity to avoid dropping events.")
	}
	ep.queue = append(ep.queue, eventOutput)
	return nil
}

func (ep *eventProcessor) dedupUser(evt Event) (string, *User) {
	// identify events don't get deduplicated and always include the full user data
	if _, ok := evt.(IdentifyEvent); ok {
		return "", nil
	}
	user := scrubUser(evt.GetBase().User, ep.config.AllAttributesPrivate, ep.config.PrivateAttributeNames)
	var userKey string
	if user.Key != nil {
		userKey = *user.Key
	}
	if ep.summarizer.noticeUser(user) {
		return userKey, nil
	} else {
		return userKey, user
	}
}

func scrubUser(user User, allAttributesPrivate bool, globalPrivateAttributes []string) *User {
	user.PrivateAttributes = nil

	if len(user.PrivateAttributeNames) == 0 && len(globalPrivateAttributes) == 0 && !allAttributesPrivate {
		return &user
	}

	isPrivate := map[string]bool{}
	for _, n := range globalPrivateAttributes {
		isPrivate[n] = true
	}
	for _, n := range user.PrivateAttributeNames {
		isPrivate[n] = true
	}

	if user.Custom != nil {
		var custom = map[string]interface{}{}
		for k, v := range *user.Custom {
			if allAttributesPrivate || isPrivate[k] {
				user.PrivateAttributes = append(user.PrivateAttributes, k)
			} else {
				custom[k] = v
			}
		}
		user.Custom = &custom
	}

	if !isEmpty(user.Avatar) && (allAttributesPrivate || isPrivate["avatar"]) {
		user.Avatar = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "avatar")
	}

	if !isEmpty(user.Country) && (allAttributesPrivate || isPrivate["country"]) {
		user.Country = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "country")
	}

	if !isEmpty(user.Ip) && (allAttributesPrivate || isPrivate["ip"]) {
		user.Ip = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "ip")
	}

	if !isEmpty(user.FirstName) && (allAttributesPrivate || isPrivate["firstName"]) {
		user.FirstName = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "firstName")
	}

	if !isEmpty(user.LastName) && (allAttributesPrivate || isPrivate["lastName"]) {
		user.LastName = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "lastName")
	}

	if !isEmpty(user.Name) && (allAttributesPrivate || isPrivate["name"]) {
		user.Name = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "name")
	}

	if !isEmpty(user.Secondary) && (allAttributesPrivate || isPrivate["secondary"]) {
		user.Secondary = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "secondary")
	}

	if !isEmpty(user.Email) && (allAttributesPrivate || isPrivate["email"]) {
		user.Email = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "email")
	}

	return &user
}

func isEmpty(s *string) bool {
	return s == nil || *s == ""
}
