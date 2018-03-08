package ldclient

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

var user = NewUser("key")
var esDefaultConfig = Config{
	UserKeysCapacity: 100,
}

func TestNoticeUserReturnsFalseForNeverSeenUser(t *testing.T) {
	es := NewEventSummarizer(esDefaultConfig)
	result := es.noticeUser(&user)
	assert.False(t, result)
}

func TestNoticeUserReturnsTrueForPreviouslySeenUser(t *testing.T) {
	es := NewEventSummarizer(esDefaultConfig)
	es.noticeUser(&user)
	user2 := user
	result := es.noticeUser(&user2)
	assert.True(t, result)
}

func TestUsersNotDeduplicatedIfCapacityExceeded(t *testing.T) {
	config := Config{
		UserKeysCapacity: 2,
	}
	es := NewEventSummarizer(config)
	user1 := NewUser("key1")
	user2 := NewUser("key2")
	user3 := NewUser("key3")
	es.noticeUser(&user1)
	es.noticeUser(&user2)
	es.noticeUser(&user3)
	result := es.noticeUser(&user3)
	assert.False(t, result)
}

func TestSummarizeEventReturnsFalseForIdentifyEvent(t *testing.T) {
	es := NewEventSummarizer(esDefaultConfig)
	event := NewIdentifyEvent(user)
	assert.False(t, es.summarizeEvent(event))
}

func TestSummarizeEventReturnsFalseForCustomEvent(t *testing.T) {
	es := NewEventSummarizer(esDefaultConfig)
	event := NewCustomEvent("whatever", user, nil)
	assert.False(t, es.summarizeEvent(event))
}

func TestSummarizeEventReturnsTrueForFeatureEventWithTrackEventsFalse(t *testing.T) {
	es := NewEventSummarizer(esDefaultConfig)
	flag := FeatureFlag{
		Key:         "key",
		TrackEvents: false,
	}
	event := NewFeatureRequestEvent(flag.Key, &flag, user, nil, nil, nil, nil)
	assert.True(t, es.summarizeEvent(event))
}

func TestSummarizeEventReturnsFalseForFeatureEventWithTrackEventsTrue(t *testing.T) {
	es := NewEventSummarizer(esDefaultConfig)
	flag := FeatureFlag{
		Key:         "key",
		TrackEvents: true,
	}
	event := NewFeatureRequestEvent(flag.Key, &flag, user, nil, nil, nil, nil)
	assert.False(t, es.summarizeEvent(event))
}

func TestSummarizeEventSetsStartAndEndDates(t *testing.T) {
	es := NewEventSummarizer(esDefaultConfig)
	flag := FeatureFlag{
		Key: "key",
	}
	event1 := NewFeatureRequestEvent(flag.Key, &flag, user, nil, nil, nil, nil)
	event2 := NewFeatureRequestEvent(flag.Key, &flag, user, nil, nil, nil, nil)
	event3 := NewFeatureRequestEvent(flag.Key, &flag, user, nil, nil, nil, nil)
	event1.BaseEvent.CreationDate = 2000
	event2.BaseEvent.CreationDate = 1000
	event3.BaseEvent.CreationDate = 1500
	es.summarizeEvent(event1)
	es.summarizeEvent(event2)
	data := es.output(es.snapshot())

	assert.Equal(t, uint64(1000), data.StartDate)
	assert.Equal(t, uint64(2000), data.EndDate)
}

func TestSummarizeEventIncrementsCounters(t *testing.T) {
	es := NewEventSummarizer(esDefaultConfig)
	flag1 := FeatureFlag{
		Key:     "key1",
		Version: 11,
	}
	flag2 := FeatureFlag{
		Key:     "key2",
		Version: 22,
	}
	unknownFlagKey := "badkey"
	variation1 := 1
	variation2 := 2
	event1 := NewFeatureRequestEvent(flag1.Key, &flag1, user, &variation1, "value1", "default1", nil)
	event2 := NewFeatureRequestEvent(flag1.Key, &flag1, user, &variation2, "value2", "default1", nil)
	event3 := NewFeatureRequestEvent(flag2.Key, &flag2, user, &variation1, "value99", "default2", nil)
	event4 := NewFeatureRequestEvent(flag1.Key, &flag1, user, &variation1, "value1", "default1", nil)
	event5 := NewFeatureRequestEvent(unknownFlagKey, nil, user, nil, nil, "default3", nil)
	es.summarizeEvent(event1)
	es.summarizeEvent(event2)
	es.summarizeEvent(event3)
	es.summarizeEvent(event4)
	es.summarizeEvent(event5)
	data := es.output(es.snapshot())

	unknownTrue := true
	expectedFeatures := map[string]flagSummaryData{
		flag1.Key: flagSummaryData{
			Default: "default1",
			Counters: []flagCounterData{
				flagCounterData{
					Version: &flag1.Version,
					Value:   "value1",
					Count:   2,
				},
				flagCounterData{
					Version: &flag1.Version,
					Value:   "value2",
					Count:   1,
				},
			},
		},
		flag2.Key: flagSummaryData{
			Default: "default2",
			Counters: []flagCounterData{
				flagCounterData{
					Version: &flag2.Version,
					Value:   "value99",
					Count:   1,
				},
			},
		},
		unknownFlagKey: flagSummaryData{
			Default: "default3",
			Counters: []flagCounterData{
				flagCounterData{
					Count:   1,
					Unknown: &unknownTrue,
				},
			},
		},
	}
	assert.Truef(t, reflect.DeepEqual(expectedFeatures, data.Features),
		"Expected features to be:\n%+v\n  but got:\n%+v", expectedFeatures, data.Features)
}

func findCounter(counters []flagCounterData, value interface{}) *flagCounterData {
	for _, c := range counters {
		if c.Value == value {
			return &c
		}
	}
	return nil
}
