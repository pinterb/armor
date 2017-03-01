package config

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

// ArmorAWSProviderName is what we're calling the custom AWS Credentials
// Provider.
const ArmorAWSProviderName = "ArmorAWSProvider"

var (
	// ErrArmorAccessKeyIDNotFound is returned when the AWS Access Key ID can't
	// be found in Armor's own config (i.e. via Viper)
	//
	// @readonly
	ErrArmorAccessKeyIDNotFound = awserr.New("ArmorAccessKeyNotFound", "AWS_ACCESS_KEY_ID not found in Armor's config", nil)

	// ErrArmorSecretAccessKeyNotFound is returned when the AWS Secret Access Key
	// can't be found in Armor's own config (i.e. via Viper)
	//
	// @readonly
	ErrArmorSecretAccessKeyNotFound = awserr.New("ArmorSecretNotFound", "AWS_SECRET_ACCESS_KEY not found in Armor's config", nil)
)

// A ArmorAWSProvider retrieves credentials from Armor's own configuration
// provider (i.e. Viper)
//
// Armor config variables used:
//
// * Access Key ID: aws_access_key_id
// * Secret Access Key: aws_secret_access_key
type ArmorAWSProvider struct {
	retrieved bool
}

// NewArmorAWSCredentials returns a pointer to a new Credentials object
// wrapping Armor's configuration provider.
func NewArmorAWSCredentials() *credentials.Credentials {
	return credentials.NewCredentials(&ArmorAWSProvider{})
}

// Retrieve retrieves the keys from Armor's Viper configration provider.
func (e *ArmorAWSProvider) Retrieve() (credentials.Value, error) {
	cfg := Config()
	e.retrieved = false

	id := ""
	if cfg.IsSet("aws_access_key_id") && cfg.GetString("aws_access_key_id") != "" {
		id = cfg.GetString("aws_access_key_id")
	}

	secret := ""
	if cfg.IsSet("aws_secret_access_key") && cfg.GetString("aws_secret_access_key") != "" {
		secret = cfg.GetString("aws_secret_access_key")
	}

	if id == "" {
		return credentials.Value{ProviderName: ArmorAWSProviderName}, ErrArmorAccessKeyIDNotFound
	}

	if secret == "" {
		return credentials.Value{ProviderName: ArmorAWSProviderName}, ErrArmorSecretAccessKeyNotFound
	}

	e.retrieved = true
	return credentials.Value{
		AccessKeyID:     id,
		SecretAccessKey: secret,
		SessionToken:    "",
		ProviderName:    ArmorAWSProviderName,
	}, nil
}

// IsExpired returns if the credentials have been retrieved.
func (e *ArmorAWSProvider) IsExpired() bool {
	return !e.retrieved
}
