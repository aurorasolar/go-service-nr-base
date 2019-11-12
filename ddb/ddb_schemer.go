package ddb

import (
	"context"
	. "github.com/aurorasolar/go-service-nr-base/visibility"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"strings"
)

type DynamoDbSchemer struct {
	Suffix    string
	AwsConfig aws.Config
	TestMode  bool
}

func NewDynamoDbSchemer(suffix string, config aws.Config, testMode bool) *DynamoDbSchemer {
	return &DynamoDbSchemer{
		Suffix:    suffix,
		AwsConfig: config,
		TestMode:  testMode,
	}
}

type Table struct {
	Name         string
	HashKeyName  string
	TtlFieldName string
}

func (db *DynamoDbSchemer) InitSchema(ctx context.Context, tablesToCreate []Table) error {
	CL(ctx).Info("Describing tables")

	svc := dynamodb.New(db.AwsConfig)

	var tables = make(map[string]int64)
	lti := dynamodb.ListTablesInput{}
	for {
		output, err := svc.ListTablesRequest(&lti).Send(ctx)
		if err != nil {
			return err
		}

		for _, t := range output.TableNames {
			tables[strings.TrimSuffix(t, db.Suffix)] = 1
		}

		if output.LastEvaluatedTableName == nil {
			break
		}
		lti.ExclusiveStartTableName = output.LastEvaluatedTableName
	}

	// Now create the missing tables
	for _, t := range tablesToCreate {
		if _, ok := tables[t.Name]; ok {
			CLS(ctx).Infof("Table %s exists", t.Name)
			err := db.ensureTtlIsSet(ctx, svc, t.Name + db.Suffix, t.TtlFieldName)
			if err != nil {
				return err
			}
			continue
		}

		newTableName := t.Name + db.Suffix

		CLS(ctx).Infof("Creating table: %s", newTableName)

		var iops *dynamodb.ProvisionedThroughput
		if db.TestMode {
			iops = &dynamodb.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(100),
				WriteCapacityUnits: aws.Int64(100),
			}
		}

		request := svc.CreateTableRequest(&dynamodb.CreateTableInput{
			TableName: aws.String(newTableName),
			AttributeDefinitions: []dynamodb.AttributeDefinition{
				{AttributeName: aws.String(t.HashKeyName), AttributeType: "S"}},
			KeySchema: []dynamodb.KeySchemaElement{
				{
					AttributeName: aws.String(t.HashKeyName), KeyType: "HASH",
				},
			},
			BillingMode:           dynamodb.BillingModePayPerRequest,
			ProvisionedThroughput: iops,
		})

		_, err := request.Send(ctx)
		if err != nil {
			return err
		}

		//noinspection GoUnhandledErrorResult
		svc.WaitUntilTableExists(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(newTableName),
		})

		err = db.ensureTtlIsSet(ctx, svc, newTableName, t.TtlFieldName)
		if err != nil {
			return err
		}
	}

	CLS(ctx).Infof("All tables are ready")
	return nil
}

func (db *DynamoDbSchemer) ensureTtlIsSet(ctx context.Context,
	client *dynamodb.Client, tableName string, ttlField string) error {

	if ttlField == "" {
		return nil
	}

	response, err := client.DescribeTimeToLiveRequest(&dynamodb.DescribeTimeToLiveInput{
		TableName: aws.String(tableName)}).Send(ctx)
	if err != nil {
		return err
	}

	if response.TimeToLiveDescription == nil ||
		response.TimeToLiveDescription.TimeToLiveStatus == dynamodb.TimeToLiveStatusDisabled {

		CLS(ctx).Infof("Setting TTL field on %s to %s", tableName, ttlField)
		_, err := client.UpdateTimeToLiveRequest(&dynamodb.UpdateTimeToLiveInput{
			TableName: aws.String(tableName),
			TimeToLiveSpecification: &dynamodb.TimeToLiveSpecification{
				AttributeName: aws.String(ttlField),
				Enabled:       aws.Bool(true),
			},
		}).Send(ctx)
		if err != nil {
			return err
		}
		CLS(ctx).Infof("Updated the TTL field on %s to %s", tableName, ttlField)
	}

	return nil
}

