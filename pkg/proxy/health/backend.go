package health

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	awsdyno "github.com/cdwlabs/armor/pkg/backend/data"
	"github.com/cdwlabs/armor/pkg/config"
	"github.com/go-kit/kit/log"
)

// This function is a wrapper to all the other backend health/readiness checks.
func backendDataHealth(logger log.Logger) error {
	err := awsCredentialsHealth(logger)
	if err != nil {
		return err
	}

	err = tokenHolderTableHealth(logger)
	return err
}

func awsCredentialsHealth(logger log.Logger) error {
	cfg := config.Config()

	if !cfg.IsSet("aws_access_key_id") || cfg.GetString("aws_access_key_id") == "" {
		return errors.New("AWS access key id is not set")
	}

	if !cfg.IsSet("aws_secret_access_key") || cfg.GetString("aws_secret_access_key") == "" {
		return errors.New("AWS secret access key is not set")
	}

	svc := iam.New(config.AWSSession())

	params := &iam.GetUserInput{}

	_, err := svc.GetUser(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "NoSuchEntity":
				// The request was rejected because it referenced an entity that does not
				// exist.
				return err
			case "ServiceFailure":
				// The request processing has failed because of an unknown error,
				// exception or failure.
				return err
			default:
				return err
			}
		}
		return err
	}

	return nil
}

func tokenHolderTableHealth(logger log.Logger) error {

	// For now, just verify that the dynamodb table exists
	exists, err := awsdyno.TokenHolderTableExists()
	if err != nil {
		logger.Log("msg", fmt.Sprintf("error checking token holder table: %s", err.Error()))
	} else if !exists {
		logger.Log("msg", fmt.Sprintf("creating missing token holder table on aws dynamodb: %s", awsdyno.TokenHolderTableName()))
		err = awsdyno.CreateTokenHolderTable()
		return err
	}

	return nil
}
