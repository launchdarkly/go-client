package fdynamodb

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/cenkalti/backoff"
	"github.com/davecgh/go-spew/spew"
	"github.com/imdario/mergo"
	c "github.com/launchdarkly/foundation/concurrent"
	"github.com/launchdarkly/foundation/ftime"
	"github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/go-metrics"
)

const (
	defaultActorChannelLimit     = 600000
	defaultUpdatePoolWorkerLimit = 24

	updateBackOffInitialInterval = 1 * time.Second
	updateBackOffMaxInterval     = 30 * time.Second
	updateBackOffMaxElapsedTime  = 120 * time.Second

	batchBackOffInitialInterval = 1 * time.Second
	batchBackOffMaxInterval     = 20 * time.Second
	batchBackOffMaxElapsedTime  = 0 * time.Second // = unlimited
)

var (
	dynamoMetrics     = metrics.NewPrefixedChildRegistry(metrics.DefaultRegistry, "dynamoDao.")
	batchRequestTimer = metrics.GetOrRegisterTimer("writeBatch.timer", dynamoMetrics)
	mailboxWaitTimer  = metrics.GetOrRegisterTimer("mailboxWait.timer", dynamoMetrics)
)

func (dao *dynamoDao) WriteToDynamo(req *WriteUpdateRequest, fingerprint *int64, tableName string) {
	dao.WriteToDynamoWithCallback(req, fingerprint, tableName, func() {})
}

func (dao *dynamoDao) WriteToDynamoWithCallback(req *WriteUpdateRequest, fingerprint *int64, tableName string, callback func()) {
	dao.actor.startActorLoop()
	dao.actor.mailbox <- dynamoWriteMessage{
		req:         req,
		fingerprint: fingerprint,
		tableName:   tableName,
		received:    ftime.Now(),
		callback:    callback,
	}
}

type dynamoWriteMessage struct {
	req         *WriteUpdateRequest
	fingerprint *int64
	tableName   string
	received    ftime.UnixMillis
	callback    func()
}

type tableFingerprint struct {
	table       string
	fingerprint int64
}

type dynamoPublisherActor struct {
	// the mailbox holds messages that should be spooled out to dynamodb, not yet seen/counted by the actor
	mailbox chan dynamoWriteMessage
	// the queue is waiting to become a batch to be sent to dynamo when it becomes large enough
	fingerprintedMessages   map[tableFingerprint]dynamoWriteMessage
	unfingerprintedMessages []dynamoWriteMessage
	dao                     *dynamoDao
	actorLoopOnce           sync.Once
}

func (pa *dynamoPublisherActor) queue() []dynamoWriteMessage {
	ret := pa.unfingerprintedMessages
	for _, v := range pa.fingerprintedMessages {
		ret = append(ret, v)
	}
	return ret
}

func (pa *dynamoPublisherActor) queueLen() int {
	return len(pa.unfingerprintedMessages) + len(pa.fingerprintedMessages)
}

// listens for new events in the channel, and flushes when necessary
// In the happy path, this will never return; it will just keep listening.
// If it does return, the caller will need to deal with the error, and
// restart this method.
func (pa *dynamoPublisherActor) act() error {
	logger.Info.Println("Starting dynamo publisher actor")
	for true {
		timeout := time.After(1 * time.Second)
		select {
		case msg := <-pa.mailbox:
			mailboxWaitTimer.Update(time.Since(msg.received.ToTime()))
			if msg.req.isDynamoWriteRequest() { // these can be batched
				if msg.fingerprint == nil {
					pa.unfingerprintedMessages = append(pa.unfingerprintedMessages, msg)
				} else {
					key := tableFingerprint{
						table:       msg.tableName,
						fingerprint: *msg.fingerprint,
					}
					// merge with existing request (if any)
					if existing, ok := pa.fingerprintedMessages[key]; ok {
						// prefer PutItem over DeleteItem in case of conflict
						if existing.req.PutRequest != nil && msg.req.PutRequest == nil {
							msg = existing
						} else if existing.req.PutRequest != nil && msg.req.PutRequest != nil {
							// merge the fields in the PutRequest
							if err := mergo.Merge(&msg.req.PutRequest.Item, existing.req.PutRequest.Item); err != nil {
								logger.Error.Printf("Error merging conflicting PutRequests: %s", spew.Sdump(err))
							} else {
								oldCb := msg.callback
								msg.callback = func() {
									oldCb()
									existing.callback()
								}
							}

						}
					}
					pa.fingerprintedMessages[key] = msg
				}
				if pa.queueLen() >= 100*pa.dao.DynamoBatchLimit {
					if err := pa.flush(); err != nil {
						return err
					}
				}
			} else { // update requests cannot be batched, so send it inline
				pa.dao.sendUpdateRequest(&msg)
			}
		case <-timeout:
			if err := pa.flush(); err != nil {
				return err
			}
		}
	}
	return nil
}

// this should only be called from inside the act loop; it is not threadsafe
func (pa *dynamoPublisherActor) flush() error {
	msgs := pa.queue()
	if len(msgs) > 0 {
		ret := pa.dao.batchEventsToDynamo(msgs)
		if ret == nil {
			pa.unfingerprintedMessages = make([]dynamoWriteMessage, 0)
			pa.fingerprintedMessages = make(map[tableFingerprint]dynamoWriteMessage)
		}
		return ret
	}
	return nil
}

// starts the actor flushing in a loop, if it is not already started. If the actor dies, it will be restarted (logging the error).
func (pa *dynamoPublisherActor) startActorLoop() {
	onceBody := func() {

		go func() {
			for range time.Tick(1 * time.Minute) {
				metrics.GetOrRegisterGauge("mailboxSize", dynamoMetrics).Update(int64(len(pa.mailbox)))
				metrics.GetOrRegisterGauge("fingerprintedMessages", dynamoMetrics).Update(int64(len(pa.fingerprintedMessages)))
				metrics.GetOrRegisterGauge("unfingerprintedMessages", dynamoMetrics).Update(int64(len(pa.unfingerprintedMessages)))
			}
		}()

		c.GoSafely(func() {
			for true {
				err := pa.act()
				logger.Error.Printf("Error in raw dynamo publisher, restarting: %+v", err)
			}
		})
	}
	pa.actorLoopOnce.Do(onceBody)
}

func (dao *dynamoDao) newDynamoPublisherActor() *dynamoPublisherActor {
	var mailbox chan dynamoWriteMessage
	if dao.actor != nil {
		mailbox = dao.actor.mailbox
	} else {
		mailbox = make(chan dynamoWriteMessage, dao.actorChannelLimit)
	}
	return &dynamoPublisherActor{
		unfingerprintedMessages: make([]dynamoWriteMessage, 0),
		fingerprintedMessages:   make(map[tableFingerprint]dynamoWriteMessage),
		mailbox:                 mailbox,
		dao:                     dao,
	}
}

func (dao *dynamoDao) batchEventsToDynamo(msgs []dynamoWriteMessage) error {
	for len(msgs) > 0 {
		size := dao.DynamoBatchLimit
		if len(msgs) < dao.DynamoBatchLimit {
			size = len(msgs)
		}
		batch := msgs[:size]
		if err := dao.writeEventBatchToDynamo(batch); err != nil {
			return err
		}
		msgs = msgs[size:]
	}
	return nil
}

func (dao *dynamoDao) sendUpdateRequest(msg *dynamoWriteMessage) {
	metricPrefix := msg.tableName + ".updates"
	if msg.req.UptateItemInput == nil {
		return
	}
	tableName := dao.makeTableName(msg.tableName)
	msg.req.UptateItemInput.TableName = tableName
	// We start the work asynchronously here, but we want to control how many updates are in
	// flight at the same time. The dao.updatePoolSem channel holds a fixed number of tokens, so we
	// can only queue work for the pool once we can put a token in.  The token is removed after the
	// work is completed.
	dao.updatePoolSem <- true
	c.GoSafely(func() {
		defer func() { <-dao.updatePoolSem }()
		bo := newUpdateExponentialBackoff()
		for numTries := 0; ; numTries++ {
			var err error
			metrics.GetOrRegisterTimer(metricPrefix+".timer", dynamoMetrics).
				Time(func() {
					_, err = dao.Dynamo.UpdateItem(msg.req.UptateItemInput)
				})
			if err != nil {
				if awsErr, ok := err.(awserr.Error); !ok || awsErr.Code() != "ProvisionedThroughputExceededException" {
					logger.Error.Printf("Error sending events to Dynamo. err: %s", spew.Sdump(err))
					metrics.GetOrRegisterCounter(metricPrefix+".err", dynamoMetrics).Inc(1)
					return
				} else {
					waitFor := bo.NextBackOff()
					if waitFor == backoff.Stop {
						logger.Warn.Printf("Giving up sending update request to Dynamo to table: %s after %d attempts. "+
							"Total elapsed time: %v. Waiting %v before returning",
							msg.tableName, numTries, bo.GetElapsedTime(), bo.MaxInterval)

						metrics.GetOrRegisterCounter(metricPrefix+".ranOutOfRetries", dynamoMetrics).Inc(1)
						time.Sleep(bo.MaxInterval)
						return
					}

					logger.Debug.Printf("Got throttled update request for table: %s after %d attempts. "+
						"Waiting %v before trying again. Total elapsed time: %v",
						msg.tableName, numTries, waitFor, bo.GetElapsedTime())

					metrics.GetOrRegisterCounter(metricPrefix+".throttle", dynamoMetrics).Inc(1)
					time.Sleep(waitFor)
				}
			} else {
				c.GoSafely(msg.callback)
				metrics.GetOrRegisterCounter(metricPrefix+".ok", dynamoMetrics).Inc(1)
				return
			}
		}
	})
	return
}

func (dao *dynamoDao) writeEventBatchToDynamo(msgs []dynamoWriteMessage) error {
	if len(msgs) > dao.DynamoBatchLimit {
		// this should really never happen, by now we've checked 3 times.
		return errors.New(fmt.Sprintf("You can only send %d operations to Dynamo at once", dao.DynamoBatchLimit))
	}
	metrics.GetOrRegisterCounter("writeBatch.messages", dynamoMetrics).Inc(int64(len(msgs)))
	items := make(map[string][]*dynamodb.WriteRequest)
	for _, e := range msgs {
		if e.req.isDynamoWriteRequest() {
			tableName := dao.makeTableName(e.tableName)
			items[*tableName] = append(items[*tableName], e.req.toDynamoWriteRequest())
		}
	}
	bo := newBatchExponentialBackoff()
	for numTries := 0; len(items) > 0; numTries++ {
		in := dynamodb.BatchWriteItemInput{
			RequestItems: items,
		}
		var out *dynamodb.BatchWriteItemOutput
		var err error
		batchRequestTimer.Time(func() {
			out, err = dao.Dynamo.BatchWriteItem(&in)
		})
		if err != nil {
			if awsErr, ok := err.(awserr.Error); !ok || awsErr.Code() != "ProvisionedThroughputExceededException" {
				logger.Error.Printf("Error sending events to Dynamo. err: %s", spew.Sdump(err))
				metrics.GetOrRegisterCounter("writeBatch.err", dynamoMetrics).Inc(1)
				return nil
			} else {
				waitFor := bo.NextBackOff()
				metrics.GetOrRegisterCounter("writeBatch.throttle", dynamoMetrics).Inc(1)

				logger.Debug.Printf("Got throttled batch request after %d attempts. "+
					"Waiting %v before trying again. Total elapsed time: %v",
					numTries, waitFor, bo.GetElapsedTime())

				time.Sleep(waitFor)
			}
		} else {
			items = out.UnprocessedItems
			for _, u := range items {
				metrics.GetOrRegisterCounter("writeBatch.unprocessed", dynamoMetrics).Inc(int64(len(u)))
			}
		}
	}
	metrics.GetOrRegisterCounter("writeBatch.ok", dynamoMetrics).Inc(1)
	for _, m := range msgs {
		c.GoSafely(m.callback)
	}
	return nil
}

// We assume that all Dynamo updates are users, and we eventually stop retrying these because
// we assume that we'll see the user again shortly.
func newUpdateExponentialBackoff() *backoff.ExponentialBackOff {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = updateBackOffInitialInterval
	bo.MaxInterval = updateBackOffMaxInterval
	bo.MaxElapsedTime = updateBackOffMaxElapsedTime
	return bo
}

// We assume that all batchable requests are events, which we care deeply about persisting and are unlikely to see again,
// so in the case of a throttling error we retry indefinitely.
func newBatchExponentialBackoff() *backoff.ExponentialBackOff {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = batchBackOffInitialInterval
	bo.MaxInterval = batchBackOffMaxInterval
	bo.MaxElapsedTime = batchBackOffMaxElapsedTime
	return bo
}
