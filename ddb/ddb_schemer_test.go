package ddb

import (
	"context"
	"github.com/aurorasolar/go-service-base/visibility"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"testing"
)

func TestSchemer(t *testing.T) {
	ddb := NewDdbTestContext(t, "../assets/localddb", false)
	defer ddb.Close()

	ctx := visibility.ImbueContext(context.Background(), zap.NewNop())
	ctx, seg := xray.BeginSegment(ctx, "nope")
	seg.Sampled = false

	schemer := NewDynamoDbSchemer("_suffix", ddb.Config, true)
	tables := []Table{
		{
			Name:         "tokens",
			HashKeyName:  "id",
			TtlFieldName: "validUntil",
		},
		{
			Name:        "blobs",
			HashKeyName: "blobId",
		},
	}
	err := schemer.InitSchema(ctx, tables)
	assert.NoError(t, err)

	// InitSchema is idempotent
	err = schemer.InitSchema(ctx, tables)
	assert.NoError(t, err)

	// Check a simple DDB request
	values := make(map[string]dynamodb.AttributeValue)
	values["id"] = dynamodb.AttributeValue{S: aws.String("hello")}
	values["value"] = dynamodb.AttributeValue{S: aws.String("world")}

	_, err = ddb.Conn.PutItemRequest(&dynamodb.PutItemInput{
		TableName: aws.String("tokens_suffix"),
		Item:      values,
	}).Send(ctx)
	assert.NoError(t, err)

	resp, err := ddb.Conn.GetItemRequest(&dynamodb.GetItemInput{
		TableName:      aws.String("tokens_suffix"),
		ConsistentRead: aws.Bool(true),
		Key: map[string]dynamodb.AttributeValue{
			"id": {S: aws.String("hello")}},
	}).Send(ctx)
	assert.NoError(t, err)

	assert.Equal(t, "world", *resp.Item["value"].S)
}
