package pubsub

import (
	"testing"
	"time"

	"github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/foundation/message"
	"github.com/nats-io/go-nats"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

// Requires NATS running locally (in dev repo's docker magic):

const (
	resource = "resource"
)

var (
	acctId            = bson.NewObjectId()
	envId             = bson.NewObjectId()
	stagingNatsConfig = NatsConfig{Url: []string{
		"nats://gnatsd01.stg.launchdarkly.com:4222",
		"nats://gnatsd02.stg.launchdarkly.com:4222",
		"nats://gnatsd03.stg.launchdarkly.com:4222"}}
	localNatsConfig = NatsConfig{Url: []string{nats.DefaultURL}}
)

func TestNatsPubSub(t *testing.T) {
	subject := message.Channel(acctId, envId, resource)
	natsConfig := localNatsConfig

	pubConfig := natsConfig
	pubConfig.Name = "pub"

	sub1config := natsConfig
	sub1config.Name = "sub1"

	sub2config := natsConfig
	sub2config.Name = "sub2"

	p, err := NewNatsPublisher(pubConfig)
	defer p.Close()
	assert.NoError(t, err)
	assert.True(t, p.IsConnected())

	sub1, err := NewNatsSubscriber(sub1config)
	defer sub1.Close()
	assert.NoError(t, err)
	assert.True(t, sub1.IsConnected())

	sub2, err := NewNatsSubscriber(sub2config)
	defer sub2.Close()
	assert.NoError(t, err)
	assert.True(t, sub2.IsConnected())

	actualMsgs1 := make([]string, 0)
	subscription1, err := sub1.SubscribeCallback(subject, func(msg []byte) {
		logger.Debug.Printf("subscription1 got message: %v", msg)
		actualMsgs1 = append(actualMsgs1, string(msg))
	})
	defer subscription1.Unsubscribe()
	assert.NoError(t, err)

	actualMsgs2 := make([]string, 0)
	subscription2, err := sub2.SubscribeCallback(subject, func(msg []byte) {
		logger.Debug.Printf("subscription2 got message: %v", msg)
		actualMsgs2 = append(actualMsgs2, string(msg))
	})
	defer subscription2.Unsubscribe()
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)
	msgs := []message.BaseMessage{
		newSpMessage(acctId, envId),
		newSpMessage(acctId, envId),
		newSpMessage(acctId, envId),
		newSpMessage(acctId, envId)}

	for _, m := range msgs {
		err = p.Publish(m)
		assert.NoError(t, err)
	}
	time.Sleep(2 * time.Second)
	expectedMsgs := make([]string, len(msgs))
	for i, m := range msgs {
		expectedMsgs[i] = string(m.Payload(true))
	}

	assert.Equal(t, expectedMsgs, actualMsgs1)
	assert.Equal(t, expectedMsgs, actualMsgs2)
}

func newSpMessage(acctId, envId bson.ObjectId) message.BaseMessage {
	return message.BaseMessage{
		AccountId:     acctId,
		EnvironmentId: envId,
		Name:          "name",
		Resource:      resource,
		Data:          time.Now().Format(time.RFC3339Nano),
	}
}
