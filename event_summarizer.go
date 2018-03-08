package ldclient

// Manages the state of summarizable information for the EventProcessor, including the
// event counters and user deduplication. Note that the methods for this type are
// deliberately not thread-safe, because they should always be called from EventProcessor's
// single event-processing goroutine.
type eventSummarizer struct {
	eventsState summaryEventsState
	userKeys    lruCache
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
		eventsState: newSummaryEventsState(),
		userKeys:    newLruCache(config.UserKeysCapacity),
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
		return true
	}
	return s.userKeys.add(*user.Key)
}

// Clears the set of users we've noticed.
func (s *eventSummarizer) resetUsers() {
	s.userKeys.clear()
}

// Adds this event to our counters, if it is a type of event we need to count.
func (s *eventSummarizer) summarizeEvent(evt Event) {
	var fe FeatureRequestEvent
	var ok bool
	if fe, ok = evt.(FeatureRequestEvent); !ok {
		return
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
}

// Returns a snapshot of the current summarized event data, and resets this state.
func (s *eventSummarizer) snapshot() summaryEventsState {
	state := s.eventsState
	s.eventsState = newSummaryEventsState()
	return state
}

// Transforms the summary data into the format used for event sending.
func makeSummaryOutput(snapshot summaryEventsState) summaryOutput {
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
