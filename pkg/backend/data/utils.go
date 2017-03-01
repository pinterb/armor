package data

import (
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/cdwlabs/armor/pkg/config"
)

const (
	tableTokenHolders string = "TokenHolders"
)

// TokenHolderTableName is the name of the table that tracks individuals
// responsible for keeping Vault's root and unseal tokens.
func TokenHolderTableName() string {
	return tableTokenHolders
}

// NewDynamoDBClient uses default Session to create a DynamoDB client.
func NewDynamoDBClient() *dynamodb.DynamoDB {
	svc := dynamodb.New(config.AWSSession())
	return svc
}
