package pubsub

import msg "github.com/launchdarkly/foundation/message"

// Pubsub interface definitions for SSE events

type PubSub interface {
	IsConnected() bool
	Close()
}

// publishes message's SSE payload to its channel ('subject' in NATS)
// The actual message that gets published is the result of message.Payload(true)
type Publisher interface {
	PubSub
	Publish(message msg.BaseMessage) error
}

// For SSE messages.
type Subscriber interface {
	PubSub
	// Starts subscribing and calls cb on the message's data when it arrives.
	// The message is expected to be ready to publish as-is to connected SSE clients
	SubscribeCallback(channel string, cb func(payload []byte)) (Subscription, error)
}

type Subscription interface {
	Unsubscribe() error
}

type NoopPublisher struct{}

func (p *NoopPublisher) Publish(message msg.BaseMessage) error { return nil }
func (p *NoopPublisher) IsConnected() bool                     { return true }
func (p *NoopPublisher) Close()                                {}

type NoopSubscriber struct{}

func (p *NoopSubscriber) IsConnected() bool { return true }
func (p *NoopSubscriber) Close()            {}
func (p *NoopSubscriber) SubscribeCallback(channel string, cb func(payload []byte)) (Subscription, error) {
	return NoopSubscription{}, nil
}

type NoopSubscription struct{}

func (s NoopSubscription) Unsubscribe() error { return nil }
