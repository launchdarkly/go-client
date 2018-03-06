package fdynamodb

import (
	"github.com/aws/aws-sdk-go/service/dynamodb"
	fs "github.com/launchdarkly/foundation/statuschecks"
)

type DynamoDao interface {
	IsInitialized() bool
	DynamoBacklog() int
	DynamoBacklogStatus() fs.ServiceStatus
	DynamoDBStatus() fs.ServiceStatus
	WriteToDynamo(req *WriteUpdateRequest, fingerprint *int64, tableName string)
	WriteToDynamoWithCallback(req *WriteUpdateRequest, fingerprint *int64, tableName string, callback func())
	Query(tableName string, input dynamodb.QueryInput) (*dynamodb.QueryOutput, error)
	GetItem(tableName string, input dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
	CreateTable(tableName *string, tableDef dynamodb.CreateTableInput) error
}

type WriteUpdateRequest struct {
	PutRequest      *dynamodb.PutRequest
	DeleteRequest   *dynamodb.DeleteRequest
	UptateItemInput *dynamodb.UpdateItemInput
}

func (req *WriteUpdateRequest) toDynamoWriteRequest() *dynamodb.WriteRequest {
	return &dynamodb.WriteRequest{
		PutRequest:    req.PutRequest,
		DeleteRequest: req.DeleteRequest,
	}
}

func (req *WriteUpdateRequest) isDynamoWriteRequest() bool {
	return req.DeleteRequest != nil || req.PutRequest != nil
}

type dynamoDao struct {
	Dynamo            *dynamodb.DynamoDB
	TablePrefix       string
	DynamoBatchLimit  int
	isInitialized     bool
	actor             *dynamoPublisherActor
	actorChannelLimit int
	updatePoolSem     chan bool
	local             bool
}

func (dao *dynamoDao) Query(tableName string, input dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
	input.TableName = dao.makeTableName(tableName)
	return dao.Dynamo.Query(&input)
}

func (dao *dynamoDao) GetItem(tableName string, input dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	input.TableName = dao.makeTableName(tableName)
	return dao.Dynamo.GetItem(&input)
}
