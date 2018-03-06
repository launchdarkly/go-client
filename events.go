package ldclient

import (
	"time"
)

type Event interface {
	GetBase() BaseEvent
}

type BaseEvent struct {
	CreationDate uint64
	User         User
}

type FeatureRequestEvent struct {
	BaseEvent
	Key                       string
	Variation                 *int
	Value                     interface{}
	Default                   interface{}
	Version                   *int
	PrereqOf                  *string
	TrackEvents               bool
	TrackEventsExpirationDate *uint64
}

type CustomEvent struct {
	BaseEvent
	Key  string
	Data interface{}
}

type IdentifyEvent struct {
	BaseEvent
}

// Used to just create the event. Normally, you don't need to call this;
// the event is created and queued automatically during feature flag evaluation.
func NewFeatureRequestEvent(key string, flag *FeatureFlag, user User, variation *int, value, defaultVal interface{}, prereqOf *string) FeatureRequestEvent {
	fre := FeatureRequestEvent{
		BaseEvent: BaseEvent{
			CreationDate: now(),
			User:         user,
		},
		Key:       key,
		Variation: variation,
		Value:     value,
		Default:   defaultVal,
		PrereqOf:  prereqOf,
	}
	if flag != nil {
		fre.Version = &flag.Version
		fre.TrackEvents = flag.TrackEvents
		fre.TrackEventsExpirationDate = flag.TrackEventsExpirationDate
	}
	return fre
}

func (evt FeatureRequestEvent) GetBase() BaseEvent {
	return evt.BaseEvent
}

// Constructs a new custom event, but does not send it. Typically, Track should be used to both create the
// event and send it to LaunchDarkly.
func NewCustomEvent(key string, user User, data interface{}) CustomEvent {
	return CustomEvent{
		BaseEvent: BaseEvent{
			CreationDate: now(),
			User:         user,
		},
		Key:  key,
		Data: data,
	}
}

func (evt CustomEvent) GetBase() BaseEvent {
	return evt.BaseEvent
}

// Constructs a new identify event, but does not send it. Typically, Identify should be used to both create the
// event and send it to LaunchDarkly.
func NewIdentifyEvent(user User) IdentifyEvent {
	return IdentifyEvent{
		BaseEvent: BaseEvent{
			CreationDate: now(),
			User:         user,
		},
	}
}

func (evt IdentifyEvent) GetBase() BaseEvent {
	return evt.BaseEvent
}

func now() uint64 {
	return toUnixMillis(time.Now())
}

func toUnixMillis(t time.Time) uint64 {
	ms := time.Duration(t.UnixNano()) / time.Millisecond

	return uint64(ms)
}
