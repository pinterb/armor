package health

import (
	//"errors"
	"fmt"
	//"net/http"
	"os"
	"strconv"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/spf13/viper"
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

	config := vaultapi.DefaultConfig()
	if viper.IsSet("vault_address") && viper.GetString("vault_address") != "" {
		config.Address = viper.GetString("vault_address")
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
	config := &vaultapi.TLSConfig{}

	if viper.IsSet("vault_ca_cert") && viper.GetString("vault_ca_cert") != "" {
		config.CACert = viper.GetString("vault_ca_cert")
	} else if v := os.Getenv("VAULT_CACERT"); v != "" {
		config.CACert = v
	}

	if viper.IsSet("vault_ca_path") && viper.GetString("vault_ca_path") != "" {
		config.CAPath = viper.GetString("vault_ca_path")
	} else if v := os.Getenv("VAULT_CAPATH"); v != "" {
		config.CAPath = v
	}

	if viper.IsSet("vault_client_cert") && viper.GetString("vault_client_cert") != "" {
		config.ClientCert = viper.GetString("vault_client_cert")
	} else if v := os.Getenv("VAULT_CLIENT_CERT"); v != "" {
		config.ClientCert = v
	}

	if viper.IsSet("vault_client_key") && viper.GetString("vault_client_key") != "" {
		config.ClientKey = viper.GetString("vault_client_key")
	} else if v := os.Getenv("VAULT_CLIENT_KEY"); v != "" {
		config.ClientKey = v
	}

	if viper.IsSet("vault_skip_verify") {
		config.Insecure = viper.GetBool("vault_skip_verify")
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
