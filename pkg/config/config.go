package config

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Provider defines a set of read-only methods for accessing the application
// configuration params as defined in one of the config files.
type Provider interface {
	ConfigFileUsed() string
	Get(key string) interface{}
	GetBool(key string) bool
	GetDuration(key string) time.Duration
	GetFloat64(key string) float64
	GetInt(key string) int
	GetInt64(key string) int64
	GetSizeInBytes(key string) uint
	GetString(key string) string
	GetStringMap(key string) map[string]interface{}
	GetStringMapString(key string) map[string]string
	GetStringMapStringSlice(key string) map[string][]string
	GetStringSlice(key string) []string
	GetTime(key string) time.Time
	InConfig(key string) bool
	IsSet(key string) bool
}

var defaultConfig *viper.Viper

// Config returns the default configuration which is bound to defined
// environment variables.
func Config() Provider {
	return defaultConfig
}

// BindWithCobra binds a Cobra command to the default configuration and then
// returns it. Using Cobra leverages different binding mechanisms including the
// configuation via an external file.
func BindWithCobra(cmd *cobra.Command) (Provider, error) {
	if bindCobra(cmd) {
		err := loadConfig()
		if err != nil {
			return nil, err
		}
		return defaultConfig, nil
	}
	return nil, fmt.Errorf("Unable to bind config to Cobra")
}

func init() {
	defaultConfig = viperConfig("armor")
}

func viperConfig(appName string) *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix(appName)
	v.AutomaticEnv()
	replacer := strings.NewReplacer("-", "_")
	v.SetEnvKeyReplacer(replacer)

	// admin address
	v.BindEnv("admin_address", AdminAddrEnvVar)
	v.SetDefault("admin_address", AdminAddrDefault)

	// http address
	v.BindEnv("http_address", HTTPAddrEnvVar)
	v.SetDefault("http_address", HTTPAddrDefault)

	// grpc address
	v.BindEnv("grpc_address", GrpcAddrEnvVar)
	v.SetDefault("grpc_address", GrpcAddrDefault)

	// appdash perf tracing
	v.BindEnv("appdash_address", AppDashAddrEnvVar)
	v.SetDefault("appdash_address", "")

	// lightstep distributed tracing
	v.BindEnv("lightstep_token", LightstepTokenEnvVar)
	v.SetDefault("lightstep_token", "")

	// vault server listen address
	v.BindEnv("vault_address", VaultAddrEnvVar)
	v.SetDefault("vault_address", VaultAddrDefault)

	// vault server ca cert
	v.BindEnv("vault_ca_cert", VaultCACertEnvVar)
	v.SetDefault("vault_ca_cert", "")

	// vault server ca path
	v.BindEnv("vault_ca_path", VaultCAPathEnvVar)
	v.SetDefault("vault_ca_path", "")

	// vault server skip tls verification
	v.BindEnv("vault_skip_verify", VaultSkipVerifyEnvVar)
	v.SetDefault("vault_skip_verify", VaultSkipVerifyDefault)

	// armor policy download directory
	v.BindEnv("policy_config_dir", PolicyConfigPathEnvVar)
	v.SetDefault("policy_config_dir", PolicyConfigPathDefault)

	// armor configuration file location
	v.BindEnv("cfgFile", ArmorConfigFileEnvVar)
	v.SetDefault("cfgFile", "")
	v.SetConfigType("yaml")

	return v
}

func bindCobra(cmd *cobra.Command) bool {
	defaultConfig.BindPFlag("admin_address", cmd.PersistentFlags().Lookup("admin-address"))
	defaultConfig.BindPFlag("http_address", cmd.PersistentFlags().Lookup("http-address"))
	defaultConfig.BindPFlag("grpc_address", cmd.PersistentFlags().Lookup("grpc-address"))
	defaultConfig.BindPFlag("appdash_address", cmd.PersistentFlags().Lookup("appdash-address"))
	defaultConfig.BindPFlag("lightstep_token", cmd.PersistentFlags().Lookup("lightstep-token"))
	defaultConfig.BindPFlag("vault_address", cmd.PersistentFlags().Lookup("vault-address"))
	defaultConfig.BindPFlag("vault_ca_cert", cmd.PersistentFlags().Lookup("vault-ca-cert"))
	defaultConfig.BindPFlag("vault_ca_path", cmd.PersistentFlags().Lookup("vault-ca-path"))
	defaultConfig.BindPFlag("vault_skip_verify", cmd.PersistentFlags().Lookup("vault-skip-verify"))
	defaultConfig.BindPFlag("policy_config_dir", cmd.PersistentFlags().Lookup("policy-config-dir"))
	defaultConfig.BindPFlag("cfgFile", cmd.PersistentFlags().Lookup("config"))
	return true
}

// loadConfig reads the configuration from the given path. If the path is empty
// it's a no-op.
func loadConfig() error {
	//if path != "" {
	if defaultConfig.IsSet("cfgFile") && defaultConfig.GetString("cfgFile") != "" {
		path := defaultConfig.GetString("cfgFile")
		_, err := os.Stat(path)
		if err != nil {
			return err
		}

		filename := filepath.Base(path)
		defaultConfig.SetConfigName(filename[:len(filename)-len(filepath.Ext(filename))])
		defaultConfig.AddConfigPath(filepath.Dir(path))

		if err := defaultConfig.ReadInConfig(); err != nil {
			return fmt.Errorf("Failed to read config file (%s): %s\n", path, err.Error())
		}
	}
	return nil
}

const (
	// AdminAddrDefault is the default admin listener address
	AdminAddrDefault string = ":8080"

	// AdminAddrEnvVar is the env variable set for the admin listener address
	AdminAddrEnvVar string = "ARMOR_ADMIN_ADDRESS"

	// HTTPAddrDefault is the default http listener address
	HTTPAddrDefault string = ":8081"

	// HTTPAddrEnvVar is the env variable set for the http listener address
	HTTPAddrEnvVar string = "ARMOR_HTTP_ADDRESS"

	// GrpcAddrDefault is the default gRPC listener address
	GrpcAddrDefault string = ":8082"

	// GrpcAddrEnvVar is the env variable set for the gRPC listener address
	GrpcAddrEnvVar string = "ARMOR_GRPC_ADDRESS"

	// AppDashAddrEnvVar is the env variable set for the AppDash listener address
	AppDashAddrEnvVar string = "ARMOR_APPDASH_ADDRESS"

	// LightstepTokenEnvVar is the env variable set for the LightStep access token
	LightstepTokenEnvVar string = "ARMOR_LIGHTSTEP_TOKEN"

	// VaultAddrDefault is the default Vault server address
	VaultAddrDefault string = "https://127.0.0.1:8200"

	// VaultAddrEnvVar is the env variable set for the Vault server address
	VaultAddrEnvVar string = "ARMOR_VAULT_ADDRESS"

	// VaultCACertEnvVar is the env variable set for the Vault server CA cert
	VaultCACertEnvVar string = "ARMOR_VAULT_CA_CERT"

	// VaultCAPathEnvVar is the env variable set for the Vault server CA path
	VaultCAPathEnvVar string = "ARMOR_VAULT_CA_PATH"

	// VaultSkipVerifyDefault is the default for Vault TLS verification skipping
	VaultSkipVerifyDefault bool = false

	// VaultSkipVerifyEnvVar is the env variable set for Vault TLS verification
	// skipping
	VaultSkipVerifyEnvVar string = "ARMOR_VAULT_SKIP_VERIFY"

	// PolicyConfigPathDefault is the default for Armor's policy config destination path
	PolicyConfigPathDefault string = "/tmp/armor/policy"

	// PolicyConfigPathEnvVar is the env variable set for Armor's policy config
	// destination path
	PolicyConfigPathEnvVar string = "ARMOR_POLICY_CONFIG_DIR"

	// ArmorConfigFileEnvVar is the env variable set for Armor's config file
	ArmorConfigFileEnvVar string = "ARMOR_CONFIG"
)
