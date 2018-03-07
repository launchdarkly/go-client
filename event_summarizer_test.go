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

	assert.Equal(t, 3, len(data.Features))

	df1 := data.Features[flag1.Key]
	assert.NotNil(t, df1)
	assert.Equal(t, "default1", df1.Default)
	assert.Equal(t, 2, len(df1.Counters))
	df1c1 := findCounter(df1.Counters, "value1")
	assert.NotNil(t, df1c1)
	assert.Equal(t, flag1.Version, *df1c1.Version)
	assert.Equal(t, 2, df1c1.Count)
	df1c2 := findCounter(df1.Counters, "value2")
	assert.NotNil(t, df1c2)
	assert.Equal(t, flag1.Version, *df1c2.Version)
	assert.Equal(t, 1, df1c2.Count)

	df2 := data.Features[flag2.Key]
	assert.NotNil(t, df2)
	assert.Equal(t, "default2", df2.Default)
	assert.Equal(t, 1, len(df2.Counters))
	assert.Equal(t, "value99", df2.Counters[0].Value)
	assert.Equal(t, flag2.Version, *df2.Counters[0].Version)
	assert.Nil(t, df2.Counters[0].Unknown)

	df3 := data.Features[unknownFlagKey]
	assert.NotNil(t, df3)
	assert.Equal(t, "default3", df3.Default)
	assert.Equal(t, 1, len(df3.Counters))
	assert.Equal(t, true, *df3.Counters[0].Unknown)
}

func findCounter(counters []flagCounterData, value interface{}) *flagCounterData {
	for _, c := range counters {
		if c.Value == value {
			return &c
		}
	}
	return nil
}
