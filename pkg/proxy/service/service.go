package service

// This file contains the Service definition, and a basic service
// implementation.

import (
	"errors"
	"fmt"
	dbackend "github.com/cdwlabs/armor/pkg/backend/data"
	"github.com/cdwlabs/armor/pkg/config"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	vaultapi "github.com/hashicorp/vault/api"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"gopkg.in/go-playground/validator.v9"
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
	SecretShares          int      `json:"secret_shares" validate:"required,gte=1,lte=10"`
	SecretThreshold       int      `json:"secret_threshold"`
	StoredShares          int      `json:"stored_shares"`
	PGPKeys               []string `json:"pgp_keys"`
	RecoveryShares        int      `json:"recovery_shares"`
	RecoveryThreshold     int      `json:"recovery_threshold"`
	RecoveryPGPKeys       []string `json:"recovery_pgp_keys"`
	RootTokenPGPKey       string   `json:"root_token_pgp_key"`
	RootTokenHolderEmail  string   `json:"root_token_holder_email" validate:"required,email"` // recipient of the root token
	SecretKeyHolderEmails []string `json:"secret_key_holder_emails" validate:"required"`      // recipients of the secret keys used for unsealing
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
	err := opts.validate()
	if err != nil {
		return InitKeys{}, err
	}

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

	// current timestamp
	t := time.Now()
	rfc := t.Format(time.RFC3339)

	// persist the root token holder
	tokenHolder := dbackend.NewTokenHolder()
	tokenHolder.Email = opts.RootTokenHolderEmail
	tokenHolder.Token = resp.RootToken
	tokenHolder.TokenType = dbackend.RootTokenType
	tokenHolder.DateCreated = rfc
	tokenHolder.DateInitialized = rfc
	err = tokenHolder.PutItem()
	if err != nil {
		return initResp, err
	}

	// persist each secret key
	for i, v := range resp.Keys {
		tokenHolder := dbackend.NewTokenHolder()
		tokenHolder.Email = opts.SecretKeyHolderEmails[i]
		tokenHolder.Token = v
		tokenHolder.TokenType = dbackend.UnsealTokenType
		tokenHolder.DateCreated = rfc
		tokenHolder.DateInitialized = rfc
		err = tokenHolder.PutItem()
		if err != nil {
			return initResp, err
		}
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

func (opts *InitOptions) validate() error {
	validate := config.Validator()
	validate.RegisterStructValidation(initOptionsStructLevelValidation, InitOptions{})
	err := validate.Struct(opts)

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
		return fmt.Errorf("Invalid init option(s)")
	}
	return nil
}

func initOptionsStructLevelValidation(sl validator.StructLevel) {

	opts := sl.Current().Interface().(InitOptions)

	if opts.SecretShares > 0 && len(opts.SecretKeyHolderEmails) != opts.SecretShares {
		sl.ReportError(opts.SecretKeyHolderEmails, "SecretKeyHolderEmails", "secretholders", "secretholders", "")
	}
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
