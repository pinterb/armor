package config

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
)

var defaultAWSConfig *aws.Config

var defaultAWSSession *session.Session

// AWSConfig returns the default configuration which is bound to defined
// configuration at start-up.
func AWSConfig() *aws.Config {
	return defaultAWSConfig
}

// AWSSession returns the default AWS Session. This session is created the
// first time NewAWSSession is called.
func AWSSession() client.ConfigProvider {
	if defaultAWSSession == nil {
		defaultAWSSession, _ = session.NewSession(AWSConfig())
	}

	return defaultAWSSession
}

// NewAWSSession returns a new AWS Session. These sessions are most commonly
// used with AWS services.
//func NewAWSSession() (*session.Session, error) {
func NewAWSSession() (client.ConfigProvider, error) {
	sess, err := session.NewSession(AWSConfig())
	if err == nil && defaultAWSSession == nil {
		defaultAWSSession = sess
	}

	return sess, err
}

func init() {
	defaultAWSConfig = aws.NewConfig().WithCredentials(NewArmorAWSCredentials())
}
