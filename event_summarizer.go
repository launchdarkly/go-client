package ldclient

import (
	"sync"
)

// Manages the state of summarizable information for the EventProcessor, including the
// event counters and user deduplication.
type eventSummarizer struct {
	eventsState       summaryEventsState
	lastKnownPastTime uint64
	userKeysSeen      map[string]struct{}
	userCapacity      int
	flagsLock         *sync.Mutex
}

type summaryEventsState struct {
	counters  map[counterKey]*counterValue
	startDate uint64
	endDate   uint64
}

type counterKey struct {
	key       string
	variation int
	version   int
}

type counterValue struct {
	count       int
	flagValue   interface{}
	flagDefault interface{}
}

type flagSummaryData struct {
	Default  interface{}       `json:"default"`
	Counters []flagCounterData `json:"counters"`
}

type flagCounterData struct {
	Value   interface{} `json:"value"`
	Version *int        `json:"version,omitempty"`
	Count   int         `json:"count"`
	Unknown *bool       `json:"unknown,omitempty"`
}

type summaryOutput struct {
	StartDate uint64                     `json:"startDate"`
	EndDate   uint64                     `json:"endDate"`
	Features  map[string]flagSummaryData `json:"features"`
}

func NewEventSummarizer(config Config) *eventSummarizer {
	return &eventSummarizer{
		eventsState:  newSummaryEventsState(),
		userKeysSeen: make(map[string]struct{}),
		userCapacity: config.UserKeysCapacity,
		flagsLock:    &sync.Mutex{},
	}
}

func newSummaryEventsState() summaryEventsState {
	return summaryEventsState{
		counters: make(map[counterKey]*counterValue),
	}
}

// Add to the set of users we've noticed, and return true if the user was already known to us.
func (s *eventSummarizer) noticeUser(user *User) bool {
	if user == nil || user.Key == nil {
		return false
	}
	s.flagsLock.Lock()
	defer s.flagsLock.Unlock()
	if _, ok := s.userKeysSeen[*user.Key]; ok {
		return true
	}
	if len(s.userKeysSeen) < s.userCapacity {
		s.userKeysSeen[*user.Key] = struct{}{}
	}
	return false
}

// Clears the set of users we've noticed.
func (s *eventSummarizer) resetUsers() {
	s.flagsLock.Lock()
	defer s.flagsLock.Unlock()
	s.userKeysSeen = make(map[string]struct{})
}

// Check whether this is a kind of event that we should summarize; if so, add it to our
// counters and return true. False means that the event should be sent individually.
func (s *eventSummarizer) summarizeEvent(evt Event) bool {
	var fe FeatureRequestEvent
	var ok bool
	if fe, ok = evt.(FeatureRequestEvent); !ok {
		return false
	}
	if fe.TrackEvents {
		return false
	}

	s.flagsLock.Lock()
	defer s.flagsLock.Unlock()

	if fe.DebugEventsUntilDate != nil {
		// The "last known past time" comes from the last HTTP response we got from the server.
		// In case the client's time is set wrong, at least we know that any expiration date
		// earlier than that point is definitely in the past.
		if *fe.DebugEventsUntilDate > s.lastKnownPastTime &&
			*fe.DebugEventsUntilDate > now() {
			return false
		}
	}

	key := counterKey{key: fe.Key}
	if fe.Variation != nil {
		key.variation = *fe.Variation
	}
	if fe.Version != nil {
		key.version = *fe.Version
	}

	if value, ok := s.eventsState.counters[key]; ok {
		value.count++
	} else {
		s.eventsState.counters[key] = &counterValue{
			count:       1,
			flagValue:   fe.Value,
			flagDefault: fe.Default,
		}
	}

	creationDate := fe.CreationDate
	if s.eventsState.startDate == 0 || creationDate < s.eventsState.startDate {
		s.eventsState.startDate = creationDate
	}
	if creationDate > s.eventsState.endDate {
		s.eventsState.endDate = creationDate
	}

	return true
}

// Marks the given timestamp (received from the server) as being in the past, in case the
// client-side time is unreliable.
func (s *eventSummarizer) setLastKnownPastTime(t uint64) {
	s.flagsLock.Lock()
	defer s.flagsLock.Unlock()
	if s.lastKnownPastTime == 0 || s.lastKnownPastTime < t {
		s.lastKnownPastTime = t
	}
}

// Returns a snapshot of the current summarized event data, and resets this state.
func (s *eventSummarizer) snapshot() summaryEventsState {
	s.flagsLock.Lock()
	defer s.flagsLock.Unlock()
	state := s.eventsState
	s.eventsState = newSummaryEventsState()
	return state
}

// Transforms the summary data into the format used for event sending.
func (s *eventSummarizer) output(snapshot summaryEventsState) summaryOutput {
	features := make(map[string]flagSummaryData)
	for key, value := range snapshot.counters {
		var flagData flagSummaryData
		var known bool
		if flagData, known = features[key.key]; !known {
			flagData = flagSummaryData{
				Default:  value.flagDefault,
				Counters: make([]flagCounterData, 0, 2),
			}
		}
		data := flagCounterData{
			Value: value.flagValue,
			Count: value.count,
		}
		if key.version == 0 {
			unknown := true
			data.Unknown = &unknown
		} else {
			version := key.version
			data.Version = &version
		}
		flagData.Counters = append(flagData.Counters, data)
		features[key.key] = flagData
	}

	return summaryOutput{
		StartDate: snapshot.startDate,
		EndDate:   snapshot.endDate,
		Features:  features,
	}
}
