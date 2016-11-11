package service

// This file contains the Service definition, and a basic service
// implementation.

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

// Service describes the service proxy to Vault.
type Service interface {
	InitStatus(ctx context.Context) (bool, error)
	//	Init(ctx context.Context, config string) (string, error)
	//	SealStatus(ctx context.Context, funcname string) (string, error)
	//	Unseal(ctx context.Context, key string) (string, error)
}

// New creates a new Service instance
func New(logger log.Logger, requestCount metrics.Counter, requestLatency metrics.Histogram) Service {
	var svc Service
	{
		svc = NewProxyService()
		svc = LoggingMiddleware(logger)(svc)
		svc = InstrumentingMiddleware(requestCount, requestLatency)(svc)
	}
	return svc
}

var (
	// ErrExample is an error to an arbitrary business rule for the "xxxxx"
	// method.
	ErrExample = errors.New("This is just a sample error.")
)

// These annoying helper functions are required to translate Go error types to
// and from strings, which is the type we use in our IDLs to represent errors.
// There is special casing to treat empty strings as nil errors.

// String2Error translates some string to a Go error.
func String2Error(s string) error {
	if s == "" {
		return nil
	}
	return errors.New(s)
}

// Error2String translates some Go error to a string.
func Error2String(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// NewProxyService returns a naive, stateless implementation of Service.
func NewProxyService() Service {
	return proxyService{}
}

type proxyService struct{}

// InitStatus implements Service
func (s proxyService) InitStatus(_ context.Context) (bool, error) {
	client, err := NewVaultClient()
	if err != nil {
		return false, err
	}

	inited, err := client.Sys().InitStatus()
	return inited, err
}

// NewVaultClient creates a Vault client by starting with Vault's DefaultConfig.
// Next, it checks if necessary flags were set in Armor (via viper) and
// finally, checks for existence of same environment variables as the Vault
// client CLI (e.g. VAULT_ADDR).
func NewVaultClient() (*vaultapi.Client, error) {
	tlsConfig, err := DefaultTLSConfig()
	if err != nil {
		return nil, err
	}

	config := vaultapi.DefaultConfig()
	if viper.IsSet("vaultAddr") && viper.GetString("vaultAddr") != "" {
		config.Address = viper.GetString("vaultAddr")
	} else if v := os.Getenv("VAULT_ADDR"); v != "" {
		config.Address = v
	}

	config.ConfigureTLS(tlsConfig)
	client, err := vaultapi.NewClient(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// DefaultTLSConfig builds a Vault client-compatible TLS configuration. It
// first checks if necessary flags were set in Armor (via viper) and
// secondarily checks for existence of same environment variables as the Vault
// client CLI (e.g. VAULT_CACERT).
func DefaultTLSConfig() (*vaultapi.TLSConfig, error) {
	config := &vaultapi.TLSConfig{}

	if viper.IsSet("vaultCACert") && viper.GetString("vaultCACert") != "" {
		config.CACert = viper.GetString("vaultCACert")
	} else if v := os.Getenv("VAULT_CACERT"); v != "" {
		config.CACert = v
	}

	if viper.IsSet("vaultCAPath") && viper.GetString("vaultCAPath") != "" {
		config.CAPath = viper.GetString("vaultCAPath")
	} else if v := os.Getenv("VAULT_CAPATH"); v != "" {
		config.CAPath = v
	}

	if viper.IsSet("vaultClientCert") && viper.GetString("vaultClientCert") != "" {
		config.ClientCert = viper.GetString("vaultClientCert")
	} else if v := os.Getenv("VAULT_CLIENT_CERT"); v != "" {
		config.ClientCert = v
	}

	if viper.IsSet("vaultClientKey") && viper.GetString("vaultClientKey") != "" {
		config.ClientKey = viper.GetString("vaultClientKey")
	} else if v := os.Getenv("VAULT_CLIENT_KEY"); v != "" {
		config.ClientKey = v
	}

	if viper.IsSet("vaultTLSSkipVerify") {
		config.Insecure = viper.GetBool("vaultTLSSkipVerify")
	} else if v := os.Getenv("VAULT_SKIP_VERIFY"); v != "" {
		var err error
		var envInsecure bool
		envInsecure, err = strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("Could not parse VAULT_SKIP_VERIFY")
		}
		config.Insecure = envInsecure
	}

	return config, nil
}
