package ldclient

import (
	"fmt"
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
	data := es.flush()

	assert.Equal(t, uint64(1000), data.StartDate)
	assert.Equal(t, uint64(2000), data.EndDate)
}

func TestSummarizeEventIncrementsCounters(t *testing.T) {
	es := NewEventSummarizer(esDefaultConfig)
	flag1 := FeatureFlag{
		Key:     "key1",
		Version: 1,
	}
	flag2 := FeatureFlag{
		Key:     "key2",
		Version: 2,
	}
	variation1 := 1
	variation2 := 2
	event1 := NewFeatureRequestEvent(flag1.Key, &flag1, user, &variation1, "value1", nil, nil)
	event2 := NewFeatureRequestEvent(flag1.Key, &flag1, user, &variation2, "value2", nil, nil)
	event3 := NewFeatureRequestEvent(flag2.Key, &flag2, user, &variation1, "value99", nil, nil)
	event4 := NewFeatureRequestEvent(flag1.Key, &flag1, user, &variation1, "value1", nil, nil)
	es.summarizeEvent(event1)
	es.summarizeEvent(event2)
	es.summarizeEvent(event3)
	es.summarizeEvent(event4)
	data := es.flush()

	assert.Equal(t, 3, len(data.Counters))
	dataMap := make(map[interface{}]counterData)
	for _, c := range data.Counters {
		dataMap[fmt.Sprintf("%s-%s", c.Key, c.Value)] = c
	}
	assert.Equal(t, flag1.Key, dataMap["key1-value1"].Key)
	assert.Equal(t, flag1.Version, *dataMap["key1-value1"].Version)
	assert.Equal(t, 2, dataMap["key1-value1"].Count)
	assert.Equal(t, flag1.Key, dataMap["key1-value2"].Key)
	assert.Equal(t, flag1.Version, *dataMap["key1-value2"].Version)
	assert.Equal(t, 1, dataMap["key1-value2"].Count)
	assert.Equal(t, flag2.Key, dataMap["key2-value99"].Key)
	assert.Equal(t, flag2.Version, *dataMap["key2-value99"].Version)
	assert.Equal(t, 1, dataMap["key2-value99"].Count)
}
