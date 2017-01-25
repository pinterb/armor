package service

// This file contains the Service definition, and a basic service
// implementation.

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/cdwlabs/armor/pkg/config"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	vaultapi "github.com/hashicorp/vault/api"
	"golang.org/x/net/context"
)

// Service describes the service proxy to Vault.
type Service interface {
	InitStatus(ctx context.Context) (bool, error)
	Init(ctx context.Context, opts InitOptions) (InitKeys, error)
	SealStatus(ctx context.Context) (SealState, error)
	Unseal(ctx context.Context, opts UnsealOptions) (SealState, error)
	Configure(ctx context.Context, opts ConfigOptions) (ConfigState, error)
}

// InitOptions maps to InitRequest structs in Vault.
type InitOptions struct {
	SecretShares      int      `json:"secret_shares"`
	SecretThreshold   int      `json:"secret_threshold"`
	StoredShares      int      `json:"stored_shares"`
	PGPKeys           []string `json:"pgp_keys"`
	RecoveryShares    int      `json:"recovery_shares"`
	RecoveryThreshold int      `json:"recovery_threshold"`
	RecoveryPGPKeys   []string `json:"recovery_pgp_keys"`
	RootTokenPGPKey   string   `json:"root_token_pgp_key"`
}

// InitKeys is the result of successfully initializing a Vault instance.
// It currently maps exactly to InitResponse struct in Vault.
type InitKeys struct {
	Keys            []string `json:"keys"`
	KeysB64         []string `json:"keys_base64"`
	RecoveryKeys    []string `json:"recovery_keys"`
	RecoveryKeysB64 []string `json:"recovery_keys_base64"`
	RootToken       string   `json:"root_token"`
}

// SealState represents the current state of Vault during the process
// of unsealing it with required number of keys.
type SealState struct {
	Sealed      bool   `json:"sealed"`
	T           int    `json:"t"`
	N           int    `json:"n"`
	Progress    int    `json:"progress"`
	Version     string `json:"version"`
	ClusterName string `json:"cluster_name"`
	ClusterID   string `json:"cluster_id"`
}

// UnsealOptions maps to UnsealRequest structs in Vault.
type UnsealOptions struct {
	Key   string `json:"key"`
	Reset bool   `json:"reset"`
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

// Init implements Service
func (s proxyService) Init(_ context.Context, opts InitOptions) (InitKeys, error) {
	client, err := NewVaultClient()
	if err != nil {
		return InitKeys{}, err
	}

	initRequest := &vaultapi.InitRequest{
		SecretShares:      opts.SecretShares,
		SecretThreshold:   opts.SecretThreshold,
		StoredShares:      opts.StoredShares,
		PGPKeys:           opts.PGPKeys,
		RecoveryShares:    opts.RecoveryShares,
		RecoveryThreshold: opts.RecoveryThreshold,
		RecoveryPGPKeys:   opts.RecoveryPGPKeys,
		RootTokenPGPKey:   opts.RootTokenPGPKey,
	}

	resp, err := client.Sys().Init(initRequest)
	if err != nil {
		return InitKeys{}, err
	}

	initResp := InitKeys{
		Keys:            resp.Keys,
		KeysB64:         resp.KeysB64,
		RecoveryKeys:    resp.RecoveryKeys,
		RecoveryKeysB64: resp.RecoveryKeysB64,
		RootToken:       resp.RootToken,
	}

	return initResp, err
}

// SealStatus implements Service
func (s proxyService) SealStatus(_ context.Context) (SealState, error) {
	client, err := NewVaultClient()
	if err != nil {
		return SealState{}, err
	}

	resp, err := client.Sys().SealStatus()
	if err != nil {
		return SealState{}, err
	}

	stateResp := SealState{
		Sealed:      resp.Sealed,
		T:           resp.T,
		N:           resp.N,
		Progress:    resp.Progress,
		Version:     resp.Version,
		ClusterName: resp.ClusterName,
		ClusterID:   resp.ClusterID,
	}

	return stateResp, err
}

// Unseal implements Service
func (s proxyService) Unseal(_ context.Context, opts UnsealOptions) (SealState, error) {
	client, err := NewVaultClient()
	if err != nil {
		return SealState{}, err
	}

	if !opts.Reset && opts.Key == "" {
		return SealState{}, errors.New("'key' must specified, or 'reset' set to true")
	}

	var stateResp SealState
	if opts.Reset {
		resp, err := client.Sys().ResetUnsealProcess()
		if err != nil {
			return SealState{}, err
		}

		stateResp = SealState{
			Sealed:      resp.Sealed,
			T:           resp.T,
			N:           resp.N,
			Progress:    resp.Progress,
			Version:     resp.Version,
			ClusterName: resp.ClusterName,
			ClusterID:   resp.ClusterID,
		}

	} else {
		resp, err := client.Sys().Unseal(opts.Key)
		if err != nil {
			return SealState{}, err
		}

		stateResp = SealState{
			Sealed:      resp.Sealed,
			T:           resp.T,
			N:           resp.N,
			Progress:    resp.Progress,
			Version:     resp.Version,
			ClusterName: resp.ClusterName,
			ClusterID:   resp.ClusterID,
		}
	}

	return stateResp, err
}

// Configure implements Service
func (s proxyService) Configure(_ context.Context, opts ConfigOptions) (ConfigState, error) {

	// validate incoming request
	cfgexpanded, err := opts.validate()
	if err != nil {
		return ConfigState{}, err
	}

	client, err := NewVaultClient()
	if err != nil {
		return ConfigState{}, err
	}
	client.SetToken(cfgexpanded.Token)

	state, err := cfgexpanded.handleRequests(client)
	return state, err
}

// NewVaultClient creates a Vault client by starting with Vault's DefaultConfig.
// Next, it checks if necessary flags were set in Armor and
// finally, checks for existence of same environment variables as the Vault
// client CLI (e.g. VAULT_ADDR).
func NewVaultClient() (*vaultapi.Client, error) {
	tlsConfig, err := DefaultTLSConfig()
	if err != nil {
		return nil, err
	}

	cfg := config.Config()
	vaultcfg := vaultapi.DefaultConfig()
	if cfg.IsSet("vault_address") && cfg.GetString("vault_address") != "" {
		vaultcfg.Address = cfg.GetString("vault_address")
	} else if v := os.Getenv("VAULT_ADDRESS"); v != "" {
		vaultcfg.Address = v
	}

	vaultcfg.ConfigureTLS(tlsConfig)
	client, err := vaultapi.NewClient(vaultcfg)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// DefaultTLSConfig builds a Vault client-compatible TLS configuration. It
// first checks if necessary flags were set in Armor and
// secondarily checks for existence of same environment variables as the Vault
// client CLI (e.g. VAULT_CACERT).
func DefaultTLSConfig() (*vaultapi.TLSConfig, error) {
	cfg := config.Config()
	vaultcfg := &vaultapi.TLSConfig{}

	if cfg.IsSet("vault_ca_cert") && cfg.GetString("vault_ca_cert") != "" {
		vaultcfg.CACert = cfg.GetString("vault_ca_cert")
	} else if v := os.Getenv("VAULT_CACERT"); v != "" {
		vaultcfg.CACert = v
	}

	if cfg.IsSet("vault_ca_path") && cfg.GetString("vault_ca_path") != "" {
		vaultcfg.CAPath = cfg.GetString("vault_ca_path")
	} else if v := os.Getenv("VAULT_CAPATH"); v != "" {
		vaultcfg.CAPath = v
	}

	if cfg.IsSet("vault_client_cert") && cfg.GetString("vault_client_cert") != "" {
		vaultcfg.ClientCert = cfg.GetString("vault_client_cert")
	} else if v := os.Getenv("VAULT_CLIENT_CERT"); v != "" {
		vaultcfg.ClientCert = v
	}

	if cfg.IsSet("vault_client_key") && cfg.GetString("vault_client_key") != "" {
		vaultcfg.ClientKey = cfg.GetString("vault_client_key")
	} else if v := os.Getenv("VAULT_CLIENT_KEY"); v != "" {
		vaultcfg.ClientKey = v
	}

	if cfg.IsSet("vault_skip_verify") {
		vaultcfg.Insecure = cfg.GetBool("vault_skip_verify")
	} else if v := os.Getenv("VAULT_SKIP_VERIFY"); v != "" {
		var err error
		var envInsecure bool
		envInsecure, err = strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("Could not parse VAULT_SKIP_VERIFY")
		}
		vaultcfg.Insecure = envInsecure
	}

	return vaultcfg, nil
}
