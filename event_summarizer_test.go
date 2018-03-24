package ldclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var user = NewUser("key")
var esDefaultConfig = Config{
	UserKeysCapacity: 100,
}

func TestNoticeUserReturnsFalseForNeverSeenUser(t *testing.T) {
	es := newEventSummarizer(esDefaultConfig)
	result := es.noticeUser(&user)
	assert.False(t, result)
}

func TestNoticeUserReturnsTrueForPreviouslySeenUser(t *testing.T) {
	es := newEventSummarizer(esDefaultConfig)
	es.noticeUser(&user)
	user2 := user
	result := es.noticeUser(&user2)
	assert.True(t, result)
}

func TestOldestUserForgottenIfCapacityExceeded(t *testing.T) {
	config := Config{
		UserKeysCapacity: 2,
	}
	es := newEventSummarizer(config)
	user1 := NewUser("key1")
	user2 := NewUser("key2")
	user3 := NewUser("key3")
	es.noticeUser(&user1)
	es.noticeUser(&user2)
	es.noticeUser(&user3)
	assert.True(t, es.noticeUser(&user3))
	assert.True(t, es.noticeUser(&user2))
	assert.False(t, es.noticeUser(&user1))
}

func TestSummarizeEventDoesNothingForIdentifyEvent(t *testing.T) {
	es := newEventSummarizer(esDefaultConfig)
	snapshot := es.snapshot()

	event := NewIdentifyEvent(user)
	es.summarizeEvent(event)

	assert.Equal(t, snapshot, es.snapshot())
}

func TestSummarizeEventDoesNothingForCustomEvent(t *testing.T) {
	es := newEventSummarizer(esDefaultConfig)
	snapshot := es.snapshot()

	event := NewCustomEvent("whatever", user, nil)
	es.summarizeEvent(event)

	assert.Equal(t, snapshot, es.snapshot())
}

func TestSummarizeEventSetsStartAndEndDates(t *testing.T) {
	es := newEventSummarizer(esDefaultConfig)
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
	data := es.snapshot()

	assert.Equal(t, uint64(1000), data.startDate)
	assert.Equal(t, uint64(2000), data.endDate)
}

func TestSummarizeEventIncrementsCounters(t *testing.T) {
	es := newEventSummarizer(esDefaultConfig)
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
	event5 := NewFeatureRequestEvent(unknownFlagKey, nil, user, nil, "default3", "default3", nil)
	es.summarizeEvent(event1)
	es.summarizeEvent(event2)
	es.summarizeEvent(event3)
	es.summarizeEvent(event4)
	es.summarizeEvent(event5)
	data := es.snapshot()

	expectedCounters := map[counterKey]*counterValue{
		counterKey{flag1.Key, variation1, flag1.Version}: &counterValue{2, "value1", "default1"},
		counterKey{flag1.Key, variation2, flag1.Version}: &counterValue{1, "value2", "default1"},
		counterKey{flag2.Key, variation1, flag2.Version}: &counterValue{1, "value99", "default2"},
		counterKey{unknownFlagKey, 0, 0}:                 &counterValue{1, "default3", "default3"},
	}
	assert.Equal(t, expectedCounters, data.counters)
}
