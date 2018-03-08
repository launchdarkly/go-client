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
	eventsUri  string
	client     *http.Client
	eventsIn   chan eventInput
	closer     chan struct{}
	closeOnce  sync.Once
	closed     bool
	summarizer *eventSummarizer
}

// Payload of the eventsIn channel. If event is nil, this is a flush request. The reply
// channel is non-nil if the caller has requested a reply.
type eventInput struct {
	event Event
	reply chan error
}

// Serializable form of a feature request event. This differs from the event that was
// passed in to us in that it has a user key instead of a user object, and it only shows
// the flag value, not the variation index.
type featureRequestEventOutput struct {
	Kind         string      `json:"kind"`
	CreationDate uint64      `json:"creationDate"`
	Key          string      `json:"key"`
	UserKey      *string     `json:"userKey"`
	Value        interface{} `json:"value"`
	Default      interface{} `json:"default"`
	Version      *int        `json:"version,omitempty"`
	PrereqOf     *string     `json:"prereqOf,omitempty"`
	Debug        *bool       `json:"debug,omitempty"`
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
	UserKey      *string     `json:"userKey"`
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
		eventsUri:  config.EventsUri + "/bulk",
		client:     client,
		eventsIn:   make(chan eventInput, 100),
		closer:     make(chan struct{}),
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
			case eventIn := <-res.eventsIn:
				if eventIn.event == nil {
					res.dispatchFlush(eventIn.reply)
				} else {
					res.dispatchEvent(eventIn.event, eventIn.reply)
				}
			case <-flushTicker.C:
				res.dispatchFlush(nil)
			case <-usersResetTicker.C:
				res.summarizer.resetUsers()
			case <-res.closer:
				flushTicker.Stop()
				usersResetTicker.Stop()
				waitCh := make(chan error)
				res.dispatchFlush(waitCh)
				<-waitCh
				return
			}
		}
	}()

	return res
}

func (ep *eventProcessor) close() {
	ep.closeOnce.Do(func() {
		close(ep.closer)
	})
}

func (ep *eventProcessor) flush() error {
	input := eventInput{
		event: nil,
		reply: make(chan error),
	}
	ep.eventsIn <- input
	// Wait for response
	err := <-input.reply
	return err
}

func (ep *eventProcessor) dispatchFlush(replyCh chan error) {
	if len(ep.queue) == 0 || ep.closed {
		if replyCh != nil {
			replyCh <- nil
		}
		return
	}

	events := ep.queue
	ep.queue = make([]interface{}, 0)

	summaryState := ep.summarizer.snapshot()

	go func() {
		err := ep.flushInternal(events, summaryState)
		if replyCh != nil {
			replyCh <- err
		}
	}()
}

func (ep *eventProcessor) flushInternal(events []interface{}, summaryState summaryEventsState) error {
	outputEvents := make([]interface{}, 0, len(events)+1) // leave room for summary, if any
	for _, e := range events {
		oe := ep.makeOutputEvent(e)
		if oe != nil {
			outputEvents = append(outputEvents, oe)
		}
	}

	if len(summaryState.counters) > 0 {
		se := summaryEventOutput{
			summaryOutput: ep.summarizer.output(summaryState),
			Kind:          SUMMARY_EVENT,
		}
		outputEvents = append(outputEvents, se)
	}

	payload, marshalErr := json.Marshal(outputEvents)

	if marshalErr != nil {
		ep.config.Logger.Printf("Unexpected error marshalling event json: %+v", marshalErr)
		return marshalErr
	}

	req, reqErr := http.NewRequest("POST", ep.eventsUri, bytes.NewReader(payload))

	if reqErr != nil {
		ep.config.Logger.Printf("Unexpected error while creating event request: %+v", reqErr)
		return reqErr
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
		return respErr
	}
	err := checkStatusCode(resp.StatusCode, ep.eventsUri)
	if err != nil {
		ep.config.Logger.Printf("Unexpected status code when sending events: %+v", err)
		if err != nil && err.Code == 401 {
			ep.config.Logger.Printf("Received 401 error, no further events will be posted since SDK key is invalid")
			ep.closed = true
			return err
		}
	} else {
		t, err := http.ParseTime(resp.Header.Get("Date"))
		if err == nil {
			ep.summarizer.setLastKnownPastTime(toUnixMillis(t))
		}
	}
	return nil
}

// Posts an event asynchronously.
func (ep *eventProcessor) sendEvent(evt Event) {
	ep.eventsIn <- eventInput{
		event: evt,
		reply: nil,
	}
}

// Posts an event and waits until it has been processed, returning the error result if any.
func (ep *eventProcessor) sendEventSync(evt Event) error {
	if evt == nil {
		return nil
	}
	input := eventInput{
		event: evt,
		reply: make(chan error),
	}
	ep.eventsIn <- input
	err := <-input.reply
	return err
}

func (ep *eventProcessor) dispatchEvent(evt Event, replyCh chan error) {
	err := ep.sendEventInternal(evt)
	if replyCh != nil {
		replyCh <- err
	}
}

func (ep *eventProcessor) sendEventInternal(evt Event) error {
	if !ep.config.SendEvents {
		return nil
	}

	// For each user we haven't seen before, we add an index event - unless this is already
	// an identify event for that user.
	user := evt.GetBase().User
	if !ep.summarizer.noticeUser(&user) {
		if _, ok := evt.(IdentifyEvent); !ok {
			indexEvent := indexEventOutput{
				Kind:         INDEX_EVENT,
				CreationDate: evt.GetBase().CreationDate,
				User:         &user,
			}
			if err := ep.queueEvent(indexEvent); err != nil {
				return err
			}
		}
	}

	if ep.summarizer.summarizeEvent(evt) {
		return nil
	}

	if ep.config.SamplingInterval > 0 && rand.Int31n(ep.config.SamplingInterval) != 0 {
		return nil
	}

	// Queue the event as-is; we'll transform it into an output event when we're flushing
	// (to avoid doing that work on our main goroutine).
	return ep.queueEvent(evt)
}

func (ep *eventProcessor) makeOutputEvent(evt interface{}) interface{} {
	switch evt := evt.(type) {
	case FeatureRequestEvent:
		fe := featureRequestEventOutput{
			Kind:         FEATURE_REQUEST_EVENT,
			CreationDate: evt.BaseEvent.CreationDate,
			Key:          evt.Key,
			UserKey:      evt.User.Key,
			Value:        evt.Value,
			Default:      evt.Default,
			Version:      evt.Version,
			PrereqOf:     evt.PrereqOf,
		}
		if !evt.TrackEvents && evt.DebugEventsUntilDate != nil {
			debug := true
			fe.Debug = &debug
		}
		return fe
	case CustomEvent:
		return customEventOutput{
			Kind:         CUSTOM_EVENT,
			CreationDate: evt.BaseEvent.CreationDate,
			Key:          evt.Key,
			UserKey:      evt.User.Key,
			Data:         evt.Data,
		}
	case IdentifyEvent:
		user := scrubUser(evt.User, ep.config.AllAttributesPrivate, ep.config.PrivateAttributeNames)
		return identifyEventOutput{
			Kind:         IDENTIFY_EVENT,
			CreationDate: evt.BaseEvent.CreationDate,
			User:         user,
		}
	case indexEventOutput:
		evt.User = scrubUser(*evt.User, ep.config.AllAttributesPrivate, ep.config.PrivateAttributeNames)
		return evt
	default:
		ep.config.Logger.Printf("Found unknown event type in output queue: %T", evt)
		return nil
	}
}

func (ep *eventProcessor) queueEvent(event interface{}) error {
	if ep.closed {
		return nil
	}
	if len(ep.queue) >= ep.config.Capacity {
		message := "Exceeded event queue capacity. Increase capacity to avoid dropping events."
		ep.config.Logger.Printf("WARN: %s", message)
		return errors.New(message)
	}
	ep.queue = append(ep.queue, event)
	return nil
}

func (ep *eventProcessor) dedupUser(evt Event) (string, *User) {
	// identify events don't get deduplicated and always include the full user data
	if _, ok := evt.(IdentifyEvent); ok {
		return "", nil
	}
	user := evt.GetBase().User
	var userKey string
	if user.Key != nil {
		userKey = *user.Key
	}
	if ep.summarizer.noticeUser(&user) {
		return userKey, nil
	} else {
		return userKey, scrubUser(user, ep.config.AllAttributesPrivate, ep.config.PrivateAttributeNames)
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
