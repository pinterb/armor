package vaultsvc

// This file contains the Service definition, and a basic service
// implementation.  It also includes service middlewares.

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	"golang.org/x/net/context"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
)

// VaultService describes the service proxy to Vault
type VaultService interface {
	InitStatus(ctx context.Context) (bool, error)
	//	Init(ctx context.Context, config string) (string, error)
	//	SealStatus(ctx context.Context, funcname string) (string, error)
	//	Unseal(ctx context.Context, key string) (string, error)
}

type vaultService struct{}

// These annoying helper functions are required to translate Go error types to
// and from strings, which is the type we use in our IDLs to represent errors.
// There is special casing to treat empty strings as nil errors.

func str2err(s string) error {
	if s == "" {
		return nil
	}
	return errors.New(s)
}

func err2str(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// NewVaultService returns a stateless implementation of VaultService
func NewVaultService() VaultService {
	return vaultService{}
}

func (s vaultService) InitStatus(_ context.Context) (bool, error) {
	client := NewVaultClient()
	inited, err := client.Sys().InitStatus()

	return inited, err
}

// Middleware is a chainable behavior modifier for VaultService
type Middleware func(VaultService) VaultService

// ServiceLoggingMiddleware returns a service middleware that logs the
// parameters and result of each method invocation.
func ServiceLoggingMiddleware(logger log.Logger) Middleware {
	return func(next VaultService) VaultService {
		return serviceLoggingMiddleware{
			logger: logger,
			next:   next,
		}
	}
}

type serviceLoggingMiddleware struct {
	logger log.Logger
	next   VaultService
}

func (mw serviceLoggingMiddleware) InitStatus(ctx context.Context) (v bool, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "InitStatus",
			"result", v,
			"error", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.InitStatus(ctx)
}

// ServiceInstrumentingMiddleware returns a service middleware that instruments
// requests made over the lifetime of the service.
func ServiceInstrumentingMiddleware(requestCount metrics.Counter, requestLatency metrics.Histogram) Middleware {
	return func(next VaultService) VaultService {
		return serviceInstrumentingMiddleware{
			requestCount:   requestCount,
			requestLatency: requestLatency,
			next:           next,
		}
	}
}

type serviceInstrumentingMiddleware struct {
	requestCount   metrics.Counter
	requestLatency metrics.Histogram
	next           VaultService
}

func (mw serviceInstrumentingMiddleware) InitStatus(ctx context.Context) (bool, error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "initstatus", "error", "false"}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
	}(time.Now())

	v, err := mw.next.InitStatus(ctx)
	return v, err
}

// NewVaultClient creates a vault client based on DefaultConfig. In practice
// this means that the environment variables required to connect to vault must
// be set correctly or panic will ensue.
func NewVaultClient() *vaultapi.Client {
	// Using same environment variables as Vault client CLI
	tlsConfig, err := DefaultTLSConfig()
	if err != nil {
		panic(err)
	}

	config := vaultapi.DefaultConfig()
	config.ConfigureTLS(tlsConfig)
	client, err := vaultapi.NewClient(config)
	if err != nil {
		panic(err)
	}

	return client
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
