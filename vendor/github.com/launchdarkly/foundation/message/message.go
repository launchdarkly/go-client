package message

import (
	"bytes"
	"fmt"

	"github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/go-metrics"
	"github.com/manucorporat/sse"
	"gopkg.in/mgo.v2/bson"
)

const (
	// NATS has a message limit of 1 MB
	MAX_MESSAGE_SIZE_BYTES = 1000000
)

var (
	messageMetrics                  = metrics.NewPrefixedChildRegistry(metrics.DefaultRegistry, "message.")
	tooBigIndirectCounter           = metrics.GetOrRegisterCounter("tooBig.indirect", messageMetrics)
	tooBigIndirectNoResourceCounter = metrics.GetOrRegisterCounter("tooBig.indirectNoResource", messageMetrics)
	tooBigNoIndirectCounter         = metrics.GetOrRegisterCounter("tooBig.noIndirect", messageMetrics)
)

type Message interface {
	Channel() string
	Payload(indirectIfTooBig bool) []byte
}

type BaseMessage struct {
	AccountId     bson.ObjectId
	EnvironmentId bson.ObjectId
	Name          string
	Resource      string
	ResourceId    string
	Data          interface{}
}

func (msg *BaseMessage) logPrefix() string {
	return fmt.Sprintf("[acct: %s][env: %s] Resource: %s ",
		msg.AccountId.Hex(), msg.EnvironmentId.Hex(), msg.Resource)
}

func Channel(accountId, envId bson.ObjectId, resource string) string {
	return accountId.Hex() + "_" + envId.Hex() + "/stream/" + resource
}

func (msg BaseMessage) Channel() string {
	return Channel(msg.AccountId, msg.EnvironmentId, msg.Resource)
}

func (msg BaseMessage) Payload(indirectIfTooBig bool) []byte {
	var out []byte
	w := bytes.NewBuffer(out)
	eventName := msg.Name
	event := sse.Event{
		Event: eventName,
		Data:  msg.Data,
	}

	sse.Encode(w, event)

	payload := w.Bytes()

	if indirectIfTooBig && len(payload) > MAX_MESSAGE_SIZE_BYTES {
		if msg.ResourceId != "" {
			tooBigIndirectCounter.Inc(1)
			logger.Info.Printf(msg.logPrefix() + "Exceeded maximum message size while publishing message. Using indirect mode")
			var iout []byte
			w = bytes.NewBuffer(iout)

			event = sse.Event{
				Event: "indirect/" + eventName,
				Data:  msg.ResourceId,
			}
			sse.Encode(w, event)
			return w.Bytes()
		} else {
			tooBigIndirectNoResourceCounter.Inc(1)
			logger.Debug.Printf(msg.logPrefix() + "Exceeded maximum message size while publishing message, but indirect resource is not provided")
			return payload
		}
	} else if len(payload) > MAX_MESSAGE_SIZE_BYTES {
		tooBigNoIndirectCounter.Inc(1)
		logger.Warn.Printf(msg.logPrefix()+"Exceeded maximum message size while publishing message, but indirect mode is not supported for %s events. Attempting to publish message with %d bytes anyway.",
			msg.Name, len(payload))
		return payload
	} else {
		return payload
	}
}

// A NilMessage can be used to esablish a stream, but not send an initial payload
type NilMessage struct {
	AccountId     bson.ObjectId
	EnvironmentId bson.ObjectId
	Resource      string
}

func (msg NilMessage) Channel() string {
	return Channel(msg.AccountId, msg.EnvironmentId, msg.Resource)
}

func (msg NilMessage) Payload(indirectIfTooBig bool) []byte {
	return nil
}
