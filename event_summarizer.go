package ldclient

import (
	"sync"
)

// Manages the state of summarizable information for the EventProcessor, including the
// event counters and user deduplication.
type eventSummarizer struct {
	currentFlags      map[counterKey]*counterValue
	startDate         uint64
	endDate           uint64
	lastKnownPastTime uint64
	userKeysSeen      map[string]struct{}
	userCapacity      int
	flagsLock         *sync.Mutex
}

type counterKey struct {
	key       string
	variation int
	version   int
}

type counterValue struct {
	count     int
	flagValue interface{}
}

type counterData struct {
	Key     string      `json:"key"`
	Value   interface{} `json:"value"`
	Version *int        `json:"version,omitempty"`
	Count   int         `json:"count"`
	Unknown *bool       `json:"unknown,omitempty"`
}

type summaryOutput struct {
	StartDate uint64        `json:"startDate"`
	EndDate   uint64        `json:"endDate"`
	Counters  []counterData `json:"counters"`
}

func NewEventSummarizer(config Config) *eventSummarizer {
	return &eventSummarizer{
		currentFlags: make(map[counterKey]*counterValue),
		userKeysSeen: make(map[string]struct{}),
		userCapacity: config.UserKeysCapacity,
		flagsLock:    &sync.Mutex{},
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

	if fe.TrackEventsExpirationDate != nil {
		// The "last known past time" comes from the last HTTP response we got from the server.
		// In case the client's time is set wrong, at least we know that any expiration date
		// earlier than that point is definitely in the past.
		if *fe.TrackEventsExpirationDate > s.lastKnownPastTime &&
			*fe.TrackEventsExpirationDate > now() {
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

	if value, ok := s.currentFlags[key]; ok {
		value.count++
	} else {
		s.currentFlags[key] = &counterValue{
			count:     1,
			flagValue: fe.Value,
		}
	}

	creationDate := fe.CreationDate
	if s.startDate == 0 || creationDate < s.startDate {
		s.startDate = creationDate
	}
	if creationDate > s.endDate {
		s.endDate = creationDate
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

// Transforms all current counters into the format used for event sending, then clears them.
func (s *eventSummarizer) flush() summaryOutput {
	s.flagsLock.Lock()
	defer s.flagsLock.Unlock()

	// Reset the set of users we've seen
	s.userKeysSeen = make(map[string]struct{})

	counters := make([]counterData, len(s.currentFlags))
	i := 0
	for key, value := range s.currentFlags {
		data := counterData{
			Key:   key.key,
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
		counters[i] = data
		i++
	}
	s.currentFlags = make(map[counterKey]*counterValue)

	ret := summaryOutput{
		StartDate: s.startDate,
		EndDate:   s.endDate,
		Counters:  counters,
	}
	s.startDate = 0
	s.endDate = 0

	return ret
}
