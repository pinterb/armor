package data

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"gopkg.in/go-playground/validator.v9"
	"time"
)

// use a single instance of Validate, it caches struct info
var (
	validate *validator.Validate
)

// validation errors
var (
	ErrTokenHolderEmailUnset   = errors.New("token holder email address not set")
	ErrTokenHolderTooManyItems = errors.New("token holder get/query/scan returned too many items")
	ErrGetItemOutputMissingKey = errors.New("GetItemOutput missing expected key")
	ErrAttributeValueMissing   = errors.New("expected AttributeValue is missing")
	ErrTokenHolderNotFound     = errors.New("token holder not found")
)

// TokenHolder identifies the person (by email address) who possesses either
// a root token or an unseal token
type TokenHolder struct {
	Email           string `json:"email" dynamodbav:"email" validate:"required,email"`      // token holder is identified by email address
	Token           string `json:"token" dynamodbav:"token,omitempty"`                      // actual token
	TokenType       string `json:"token_type" dynamodbav:"tokenType,omitempty"`             // either root or unseal token
	DateCreated     string `json:"date_created" dynamodbav:"dateCreated,omitempty"`         // date token holder was identified
	DateInitialized string `json:"date_initialized" dynamodbav:"dateInitialized,omitempty"` // date Vault was initialized
	DateDelivered   string `json:"date_delivered" dynamodbav:"dateDelivered,omitempty"`     // date last delivered to token holder
}

const (
	// RootTokenType is the constant used to identify root token type.
	RootTokenType string = "root"
	// UnsealTokenType is the constant used to identify unseal token type. The
	// Vault documentation refers to this as a secret key. There should multiple
	// tokens of this type.
	UnsealTokenType string = "unseal"
)

// These constants are used to map DynamoDB AttributeValue's to TokenHolder
// struct
const (
	emailAttrNm           string = "email"
	tokenAttrNm           string = "token"
	tokenTypeAttrNm       string = "token_type"
	dateCreatedAttrNm     string = "date_created"
	dateInitializedAttrNm string = "date_initialized"
	dateDeliveredAttrNm   string = "date_delivered"
)

// NewTokenHolder creates a new TokenHolder. This can be used for both read and
// write operations in AWS.
func NewTokenHolder() *TokenHolder {
	t := time.Now()
	rfc := t.Format(time.RFC3339)

	hldr := &TokenHolder{
		Email:           "",
		Token:           "",
		TokenType:       "",
		DateCreated:     rfc,
		DateInitialized: "",
		DateDelivered:   "",
	}

	return hldr
}

// Validate the TokenHolder struct
func (tokenHolder *TokenHolder) Validate() error {

	validate = validator.New()
	err := validate.Struct(tokenHolder)
	if err != nil {
		validationerr := ""
		for _, err := range err.(validator.ValidationErrors) {
			//fmt.Printf("Error: ActualTag: %s", err.ActualTag())
			//fmt.Printf("Error: Field: %s", err.Field())
			//fmt.Printf("Error: Kind: %s", err.Kind())
			//fmt.Printf("Error: Namespace: %s", err.Namespace())
			//fmt.Printf("Error: Param: %s", err.Param())
			//fmt.Printf("Error: StructNamespace: %s", err.StructNamespace())
			//fmt.Printf("Error: StructField: %s", err.StructField())
			//fmt.Printf("Error: Tag: %s", err.Tag())
			validationerr = fmt.Sprintf("%s validation failed on '%s' check", err.Namespace(), err.Tag())
			break
		}

		if validationerr != "" {
			return fmt.Errorf(validationerr)
		}
		return fmt.Errorf("Invalid token holder")
	}

	return nil
}

// PutItem persists a TokenHolder in AWS DynamoDB.
func (tokenHolder *TokenHolder) PutItem() error {
	item, err := dynamodbattribute.MarshalMap(tokenHolder)
	//item, err := tokenHolder.attributeValues()
	if err != nil {
		return err
	}

	params := &dynamodb.PutItemInput{
		TableName: aws.String(TokenHolderTableName()),
		Item:      item,
	}

	svc := NewDynamoDBClient()
	resp, err := svc.PutItem(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "ConditionalCheckFailedException":
				// A condition specified in the operation could not be evaluated.
				return err
			case "ProvisionedThroughputExceededException":
				// Your request rate is too high. The AWS SDKs for DynamoDB
				// automatically retry requests that receive this exception. Your
				// request is eventually successful, unless your retry queue is too
				// large to finish. Reduce the frequency of requests and use
				// exponential backoff.
				return err
			case "ResourceNotFoundException":
				// The operation tried to access a nonexistent table or index. The
				// resource might not be specified correctly, or its status might not
				// be ACTIVE.
				return err
			case "ItemCollectionSizeLimitExceededException":
				// An item collection is too large. This exception is only returned
				// for tables that have one or more local secondary indexes.
				return err
			case "InternalServerError":
				// An error occurred on the server side.
				return err
			default:
				return err
			}
		}
		return err
	}

	err = dynamodbattribute.UnmarshalMap(resp.Attributes, tokenHolder)
	//err = tokenHolder.refreshWithAttributeValues(resp.Attributes)

	return err
}

// GetItem populates TokenHolder with data from AWS DynamoDB.
func (tokenHolder *TokenHolder) GetItem() error {
	svc := NewDynamoDBClient()

	params := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			emailAttrNm: {
				S: aws.String(tokenHolder.Email),
			},
		},
		TableName:      aws.String(TokenHolderTableName()),
		ConsistentRead: aws.Bool(true),
	}

	list, err := svc.GetItem(params)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "ResourceNotFoundException":
				//   The operation tried to access a nonexistent table or index. The
				//   resource might not be specified correctly, or its status might not
				//   be ACTIVE.
				return err
			case "ProvisionedThroughputExceededException":
				// Your request rate is too high. The AWS SDKs for DynamoDB
				// automatically retry requests that receive this exception. Your
				// request is eventually successful, unless your retry queue is too
				// large to finish.
				return err
			case "InternalServerError":
				// An error occurred on the server side.
				return err
			default:
				return err
			}
		}
		return err
	}

	err = dynamodbattribute.UnmarshalMap(list.Item, tokenHolder)
	//err = tokenHolder.refreshWithAttributeValues(list.Item)

	return err
}

// Map a TokenHolder to DynamoDB AttributeValues. This is useful during PutItem
// operations.
func (tokenHolder *TokenHolder) attributeValues() (map[string]*dynamodb.AttributeValue, error) {
	item := make(map[string]*dynamodb.AttributeValue)

	if tokenHolder.Email != "" {
		item[emailAttrNm] = &dynamodb.AttributeValue{
			S: aws.String(tokenHolder.Email),
		}
	}

	if tokenHolder.Token != "" {
		item[tokenAttrNm] = &dynamodb.AttributeValue{
			S: aws.String(tokenHolder.Token),
		}
	}

	if tokenHolder.TokenType != "" {
		item[tokenTypeAttrNm] = &dynamodb.AttributeValue{
			S: aws.String(tokenHolder.TokenType),
		}
	}

	if tokenHolder.DateCreated != "" {
		item[dateCreatedAttrNm] = &dynamodb.AttributeValue{
			S: aws.String(tokenHolder.DateCreated),
		}
	}

	if tokenHolder.DateInitialized != "" {
		item[dateInitializedAttrNm] = &dynamodb.AttributeValue{
			S: aws.String(tokenHolder.DateInitialized),
		}
	}

	if tokenHolder.DateDelivered != "" {
		item[dateDeliveredAttrNm] = &dynamodb.AttributeValue{
			S: aws.String(tokenHolder.DateDelivered),
		}
	}

	return item, nil
}

// Map a DynamoDB AttributeValues to TokenHolder. This is useful after
// a GetItem call to AWS.
func (tokenHolder *TokenHolder) refreshWithAttributeValues(dynoAttrs map[string]*dynamodb.AttributeValue) error {

	for key, attr := range dynoAttrs {
		switch key {
		case emailAttrNm:
			tokenHolder.Email = attr.GoString()
		case tokenAttrNm:
			tokenHolder.Token = attr.GoString()
		case tokenTypeAttrNm:
			tokenHolder.TokenType = attr.GoString()
		case dateCreatedAttrNm:
			tokenHolder.DateCreated = attr.GoString()
		case dateInitializedAttrNm:
			tokenHolder.DateInitialized = attr.GoString()
		case dateDeliveredAttrNm:
			tokenHolder.DateDelivered = attr.GoString()
		default:
			return fmt.Errorf("unexpected attribute encountered: %s", key)
		}
	}

	return nil
}

// TokenHolderTableExists checks for the existence of the Token Holder table.
// It is intended to be used for health & readiness checks, and bootstrapping.
func TokenHolderTableExists() (bool, error) {
	svc := NewDynamoDBClient()

	params := &dynamodb.DescribeTableInput{
		TableName: aws.String(TokenHolderTableName()),
	}

	_, err := svc.DescribeTable(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "ResourceNotFoundException":
				return false, nil
			case "InternalServerError":
				return false, err
			default:
				return false, nil
			}
		}
	}

	return true, nil
}

// CreateTokenHolderTable creates the Token Holder table. It assumes the table
// does not exist. Call during readiness check or as part of some initial
// bootstrap step.
func CreateTokenHolderTable() error {
	svc := NewDynamoDBClient()

	params := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String(emailAttrNm),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String(emailAttrNm),
				KeyType:       aws.String("HASH"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
		TableName: aws.String(TokenHolderTableName()),
	}

	_, err := svc.CreateTable(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "ResourceInUseException":
				// The operation conflicts with the resource's availability. For
				// example, you attempted to recreate an existing table, or tried to
				// delete a table currently in the CREATING state.
				return err
			case "LimitExceededException":
				// The number of concurrent table requests (cumulative number of tables
				// in the CREATING, DELETING or UPDATING state) exceeds the maximum
				// allowed of 10.
				return err
			case "InternalServerError":
				// An error occurred on the server side.
				return err
			default:
				return err
			}
		}
		return err
	}

	waitParams := &dynamodb.DescribeTableInput{
		TableName: aws.String(TokenHolderTableName()),
	}

	err = svc.WaitUntilTableExists(waitParams)
	return err
}

// DeleteTokenHolderTable deletes the Token Holder table. It assumes the table
// exists. Since this is a destructive operation, please use caution!
func DeleteTokenHolderTable() error {
	svc := NewDynamoDBClient()

	params := &dynamodb.DeleteTableInput{
		TableName: aws.String(TokenHolderTableName()),
	}

	_, err := svc.DeleteTable(params)

	if err != nil {
		//		if awsErr, ok := err.(awserr.Error); ok {
		//			switch awsErr.Code() {
		//			case "ResourceNotFoundException":
		//				// The operation tried to access a nonexistent table or index. The
		//				// resource might not be specified correctly, or its status might not
		//				// be ACTIVE.
		//				return false, nil
		//			case "ResourceInUseException":
		//				// The operation conflicts with the resource's availability. For
		//				// example, you attempted to recreate an existing table, or tried to
		//				// delete a table currently in the CREATING state.
		//				return err
		//			case "LimitExceededException":
		//				// The number of concurrent table requests (cumulative number of tables
		//				// in the CREATING, DELETING or UPDATING state) exceeds the maximum
		//				// allowed of 10.
		//				return err
		//			case "InternalServerError":
		//				// An error occurred on the server side.
		//				return err
		//			default:
		//				return false, nil
		//			}
		//		}
		return err
	}

	waitParams := &dynamodb.DescribeTableInput{
		TableName: aws.String(TokenHolderTableName()),
	}

	err = svc.WaitUntilTableNotExists(waitParams)
	return err
}
