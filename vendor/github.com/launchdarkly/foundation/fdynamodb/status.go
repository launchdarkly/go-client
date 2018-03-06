package fdynamodb

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/launchdarkly/foundation/logger"
	fs "github.com/launchdarkly/foundation/statuschecks"
)

// Get the number of pending events in the queue to be flushed to dynamo
func (dao *dynamoDao) DynamoBacklog() int {
	return len(dao.actor.mailbox)
}

func (dao *dynamoDao) DynamoBacklogStatus() fs.ServiceStatus {
	backlog := dao.DynamoBacklog()
	if backlog > 500 {
		if backlog >= dao.actorChannelLimit {
			return fs.DownService()
		} else {
			return fs.DegradedService()
		}
	}
	return fs.HealthyService()
}

func (dao *dynamoDao) DynamoDBStatus() fs.ServiceStatus {
	tables, err := dao.Dynamo.ListTables(&dynamodb.ListTablesInput{
		Limit: aws.Int64(1),
	})
	if err != nil {
		logger.Error.Printf("Error checking Dynamo status %s", err.Error())
		return fs.DownService()
	}
	if len(tables.TableNames) < 1 {
		return fs.DownService()
	}
	return fs.HealthyService()
}
