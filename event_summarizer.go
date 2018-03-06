package ldclient

import (
	"bytes"
	"sync"

	"github.com/launchdarkly/foundation/wiltfilter"
	"github.com/launchdarkly/go-metrics"
)

// Manages the state of summarizable information for the EventProcessor, including the
// event counters and user deduplication.
type eventSummarizer struct {
	currentFlags      map[counterKey]*counterValue
	startDate         uint64
	endDate           uint64
	lastKnownPastTime uint64
	userFilter        wiltfilter.RefreshingWiltFilter
	flagsLock         *sync.Mutex
}

type counterKey struct {
	key       string
	variation *int
	version   *int
}

type counterValue struct {
	count     int
	flagValue interface{}
}

type counterData struct {
	Key     string      `json:"key"`
	Value   interface{} `json:"value"`
	Version *int        `json:"version"`
	Count   int         `json:"count"`
}

type summaryOutput struct {
	StartDate uint64        `json:"startDate"`
	EndDate   uint64        `json:"endDate"`
	Counters  []counterData `json:"counters"`
}

func NewEventSummarizer(config Config) *eventSummarizer {
	filterConfig := wiltfilter.RefreshConfig{
		Interval: wiltfilter.Manually,
	}
	var registry metrics.Registry
	userFilter := wiltfilter.NewWiltFilterWithRefreshConfig(filterConfig, "userFilter", registry)

	return &eventSummarizer{
		currentFlags: make(map[counterKey]*counterValue),
		userFilter:   userFilter,
		flagsLock:    &sync.Mutex{},
	}
}

// Add to the set of users we've noticed, and return true if the user is new to us.
func (s *eventSummarizer) noticeUser(user *User) bool {
	return s.userFilter.AlreadySeen(user)
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

	key := counterKey{
		key:       fe.Key,
		variation: fe.Variation,
		version:   fe.Version,
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

	// Reset the set of users we've seen (TODO: need to add a manual refresh method to wiltfilter)
	s.userFilter.Refresh()

	counters := make([]counterData, len(s.currentFlags))
	i := 0
	for key, value := range s.currentFlags {
		counters[i] = counterData{
			Key:     key.key,
			Value:   value.flagValue,
			Version: key.version,
			Count:   value.count,
		}
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

// Makes wiltfilter work with Users.
func (user *User) UniqueBytes() *bytes.Buffer {
	if user == nil || user.Key == nil {
		return nil
	}
	return bytes.NewBufferString(*user.Key)
}
