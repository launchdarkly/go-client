package fdynamodb

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/davecgh/go-spew/spew"
	"github.com/launchdarkly/foundation/logger"
)

const (
	ttlAttribute = "ttl"
)

func (dao *dynamoDao) CreateTable(tableName_in *string, tableDef dynamodb.CreateTableInput) error {
	tableName := dao.makeTableName(*tableName_in)
	tableDef.TableName = tableName
	if _, e := dao.Dynamo.DescribeTable(&dynamodb.DescribeTableInput{TableName: tableName}); e == nil {
		// the table already exists
		return nil
	} else {
		// the error from DescribeTable might be a ResourceNotFoundException, in which case,
		// everything is actually perfectly fine, and we want to proceed. In all other cases,
		// something is terribly wrong and panic is the wisest course of action.
		if awsErr, ok := e.(awserr.Error); !ok || awsErr.Code() != "ResourceNotFoundException" {
			msg := fmt.Sprintf("Unable to get information about table %s: %s", *tableName, spew.Sdump(e))
			logger.Error.Println(msg)
			return errors.New(msg)
		}
	}

	if resp, err := dao.Dynamo.CreateTable(&tableDef); err != nil {
		msg := fmt.Sprintf("Unable to create table %s: %s", *tableName, spew.Sdump(err))
		logger.Error.Println(msg)
		return errors.New(msg)
	} else {
		tries := 1
		desc := resp.TableDescription
		for *desc.TableStatus != "ACTIVE" && tries < 20 {
			tries = tries + 1
			time.Sleep(1 * time.Second)
			if r, e := dao.Dynamo.DescribeTable(&dynamodb.DescribeTableInput{TableName: tableName}); e != nil {
				desc = r.Table
			}
		}
		logger.Info.Printf("Created table for %s", *tableName)
		// TODO: Local DynamoDB does not yet support TTL: https://github.com/aws/aws-sdk-js/issues/1527
		// When it does, remove this
		if !dao.local {
			err = dao.setTtlOnTable(*tableName)
			if err != nil {
				msg := fmt.Sprintf("Created table, but unable to set TTL: %s: %s", *tableName, spew.Sdump(err))
				logger.Error.Println(msg)
				return errors.New(msg)
			}
		} else {
			logger.Info.Printf("Running DynamoDB in local mode. Not setting TTL for table %s", *tableName)
		}
	}
	return nil
}

func (dao *dynamoDao) setTtlOnTable(name string) error {
	logger.Info.Printf("Setting TTL for table: %s using attribute: %s", name, ttlAttribute)
	ttlSpec := dynamodb.TimeToLiveSpecification{}
	ttlSpec.SetAttributeName(ttlAttribute)
	ttlSpec.SetEnabled(true)
	ttlInput := dynamodb.UpdateTimeToLiveInput{}
	ttlInput.SetTableName(name)
	ttlInput.SetTimeToLiveSpecification(&ttlSpec)

	newTtlOutput, err := dao.Dynamo.UpdateTimeToLive(&ttlInput)
	if err != nil {
		return err
	}
	logger.Info.Printf("Got Dynamo TTL Update response for table [%s]: %+v",
		name, newTtlOutput)
	return nil
}

func (dao *dynamoDao) makeTableName(name string) *string {
	return aws.String(fmt.Sprintf("%s_%s", dao.TablePrefix, name))
}
