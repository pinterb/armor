package health

import (
	//"errors"
	"fmt"
	//"net/http"
	"os"
	"strconv"

	"github.com/cdwlabs/armor/pkg/config"
	vaultapi "github.com/hashicorp/vault/api"
)

func vaulthealth() error {
	client, err := newVaultClient()
	if err != nil {
		return err
	}

	_, err = client.Sys().InitStatus()
	return err
}

// newVaultClient creates a Vault client by starting with Vault's DefaultConfig.
// Next, it checks if necessary flags were set in Armor (via viper) and
// finally, checks for existence of same environment variables as the Vault
// client CLI (e.g. VAULT_ADDR).
func newVaultClient() (*vaultapi.Client, error) {
	tlsConfig, err := defaultTLSConfig()
	if err != nil {
		return nil, err
	}

	cfg := config.Config()
	config := vaultapi.DefaultConfig()
	if cfg.IsSet("vault_address") && cfg.GetString("vault_address") != "" {
		config.Address = cfg.GetString("vault_address")
	} else if v := os.Getenv("VAULT_ADDRESS"); v != "" {
		config.Address = v
	}

	config.ConfigureTLS(tlsConfig)
	client, err := vaultapi.NewClient(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// defaultTLSConfig builds a Vault client-compatible TLS configuration. It
// first checks if necessary flags were set in Armor (via viper) and
// secondarily checks for existence of same environment variables as the Vault
// client CLI (e.g. VAULT_CACERT).
func defaultTLSConfig() (*vaultapi.TLSConfig, error) {
	cfg := config.Config()
	config := &vaultapi.TLSConfig{}

	if cfg.IsSet("vault_ca_cert") && cfg.GetString("vault_ca_cert") != "" {
		config.CACert = cfg.GetString("vault_ca_cert")
	} else if v := os.Getenv("VAULT_CACERT"); v != "" {
		config.CACert = v
	}

	if cfg.IsSet("vault_ca_path") && cfg.GetString("vault_ca_path") != "" {
		config.CAPath = cfg.GetString("vault_ca_path")
	} else if v := os.Getenv("VAULT_CAPATH"); v != "" {
		config.CAPath = v
	}

	if cfg.IsSet("vault_client_cert") && cfg.GetString("vault_client_cert") != "" {
		config.ClientCert = cfg.GetString("vault_client_cert")
	} else if v := os.Getenv("VAULT_CLIENT_CERT"); v != "" {
		config.ClientCert = v
	}

	if cfg.IsSet("vault_client_key") && cfg.GetString("vault_client_key") != "" {
		config.ClientKey = cfg.GetString("vault_client_key")
	} else if v := os.Getenv("VAULT_CLIENT_KEY"); v != "" {
		config.ClientKey = v
	}

	if cfg.IsSet("vault_skip_verify") {
		config.Insecure = cfg.GetBool("vault_skip_verify")
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
