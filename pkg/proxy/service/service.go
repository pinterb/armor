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
	"golang.org/x/net/context"
)

// Service describes the service proxy to Vault.
type Service interface {
	InitStatus(ctx context.Context) (bool, error)
	//	Init(ctx context.Context, config string) (string, error)
	//	SealStatus(ctx context.Context, funcname string) (string, error)
	//	Unseal(ctx context.Context, key string) (string, error)
}

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

// NewVaultClient creates a vault client based on DefaultConfig. In practice
// this means that the environment variables required to connect to vault must
// be set correctly or the handshake with Vault can never happen.
func NewVaultClient() (*vaultapi.Client, error) {
	// Using same environment variables as Vault client CLI
	tlsConfig, err := DefaultTLSConfig()
	if err != nil {
		return nil, err
	}

	config := vaultapi.DefaultConfig()
	config.ConfigureTLS(tlsConfig)
	client, err := vaultapi.NewClient(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// DefaultTLSConfig builds a Vault client-compatiable TLS configuration. It
// uses the same environment variables as the Vault client CLI.
func DefaultTLSConfig() (*vaultapi.TLSConfig, error) {
	config := &vaultapi.TLSConfig{}
	if v := os.Getenv("VAULT_CACERT"); v != "" {
		config.CACert = v
	}

	if v := os.Getenv("VAULT_CAPATH"); v != "" {
		config.CAPath = v
	}

	if v := os.Getenv("VAULT_CLIENT_CERT"); v != "" {
		config.ClientCert = v
	}

	if v := os.Getenv("VAULT_CLIENT_KEY"); v != "" {
		config.ClientKey = v
	}

	if v := os.Getenv("VAULT_SKIP_VERIFY"); v != "" {
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
