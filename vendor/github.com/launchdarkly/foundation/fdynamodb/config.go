package fdynamodb

import (
	"math"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/launchdarkly/go-metrics"
)

type DynamoConfig struct {
	Endpoint          string
	Region            string
	TablePrefix       string
	BatchLimit        int
	ActorChannelLimit int `gcfg:"maxPendingWorkItems"`
	UpdatePoolWorkers int
	Local             bool
}

func (dao *dynamoDao) IsInitialized() bool {
	return dao.isInitialized
}

func Initialize(c DynamoConfig) DynamoDao {
	config := defaults.Get().Config
	config.Endpoint = aws.String(c.Endpoint)
	config.Region = aws.String(c.Region)
	sess := session.New(config)

	dynamoBatchLimit := 15
	if c.BatchLimit > 0 {
		dynamoBatchLimit = int(math.Max(float64(25), float64(c.BatchLimit)))
	}

	actorChannelLimit := defaultActorChannelLimit
	if c.ActorChannelLimit > 0 {
		actorChannelLimit = c.ActorChannelLimit
	}
	updatePoolWorkers := defaultUpdatePoolWorkerLimit
	if c.UpdatePoolWorkers > 0 {
		updatePoolWorkers = c.UpdatePoolWorkers
	}

	ret := dynamoDao{
		TablePrefix:       c.TablePrefix,
		Dynamo:            dynamodb.New(sess),
		DynamoBatchLimit:  dynamoBatchLimit,
		isInitialized:     true,
		actorChannelLimit: actorChannelLimit,
		updatePoolSem:     make(chan bool, updatePoolWorkers),
		local:             c.Local,
	}
	ret.actor = ret.newDynamoPublisherActor()
	metrics.NewRegisteredFunctionalGauge("actorChannelSize", dynamoMetrics, func() int64 {
		return int64(len(ret.actor.mailbox))
	})
	metrics.NewRegisteredFunctionalGauge("actorBatchQueueSize", dynamoMetrics, func() int64 {
		return int64(ret.actor.queueLen())
	})
	return &ret
}
