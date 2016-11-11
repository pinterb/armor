package commands

import (
	//	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mitchellh/go-homedir"

	lightstep "github.com/lightstep/lightstep-tracer-go"
	stdopentracing "github.com/opentracing/opentracing-go"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"sourcegraph.com/sourcegraph/appdash"
	appdashot "sourcegraph.com/sourcegraph/appdash/opentracing"

	"github.com/cdwlabs/armor/pb"
	"github.com/cdwlabs/armor/pkg/proxy/endpoints"
	vaultgrpc "github.com/cdwlabs/armor/pkg/proxy/grpc"
	vaulthttp "github.com/cdwlabs/armor/pkg/proxy/http"
	"github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"

	flag "github.com/spf13/pflag"

	"regexp"

	"github.com/cdwlabs/armor/cmd/helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// commandError is an error used to signal different error situations in
// command handling.
type commandError struct {
	s         string
	userError bool
}

func (c commandError) Error() string {
	return c.s
}

func (c commandError) isUserError() bool {
	return c.userError
}

// Catch some of the obvious user erros from Cobra.
// We don't want to show the usage message for every error.
// The below may be generic...until proven otherwise.
var userErrorRegexp = regexp.MustCompile("argument|flag|shorthand")

func isUserError(err error) bool {
	if cErr, ok := err.(commandError); ok && cErr.isUserError() {
		return true
	}

	return userErrorRegexp.MatchString(err.Error())
}

// ArmorCmd is Armor's root command.
// Every other command attached to ArmorCmd is a child command to it.
var ArmorCmd = &cobra.Command{
	Use:   "armor",
	Short: "armor starts a proxy to a Vault server",
	Long: `armor is the main command, used to start your proxy.

Armor is a proxy to HashiCorp Vault.
It addition to proxying the Vault APIs, it provides
features for managing Vault roles, policies and tokens.

Complete documentation is available at https://github.com/cdwlabs/armor `,
	RunE: Start,
}

var armorCmdV *cobra.Command

// Flags that are to be added to commands.
var (
	debugAddr          string
	httpAddr           string
	grpcAddr           string
	appdashAddr        string
	lightstepToken     string
	vaultAddr          string
	vaultCACert        string
	vaultCAPath        string
	vaultTLSSkipVerify bool
	cfgFile            string
)

// Execute adds all the child commands to the root command ArmorCmd and sets
// flags appropriately.
func Execute() {
	ArmorCmd.SetGlobalNormalizationFunc(helpers.NormalizeArmorFlags)

	ArmorCmd.SilenceUsage = true

	AddCommands()

	if c, err := ArmorCmd.ExecuteC(); err != nil {
		if isUserError(err) {
			c.Println("")
			c.Println(c.UsageString())
		}

		os.Exit(-1)
	}
}

// AddCommands adds child commands to the root command ArmorCmd.
func AddCommands() {

}

// initRootPersistentFlags initialize common flags related to running the
// Armor server. Called by initArmorFlags()
func initRootPersistentFlags() {
	// hack alert: Not sure how to format raw strings to they display
	// consistently on the cli
	// debug address
	debugAddrDesc := `Debug and metrics listen address. (default ":8080")
                                 Overrides the ARMOR_DEBUG_ADDR environment variable if set.
                                 NOT YET SUPPORTED!!
																 `
	ArmorCmd.PersistentFlags().StringVar(&debugAddr, "debug-addr", "", debugAddrDesc)
	viper.BindPFlag("debugAddr", ArmorCmd.PersistentFlags().Lookup("debug-addr"))
	viper.BindEnv("debugAddr", "ARMOR_DEBUG_ADDR")
	viper.SetDefault("debugAddr", ":8080")

	// http address
	httpAddrDesc := `HTTP listen address. (default ":8081")
                                 Overrides the ARMOR_HTTP_ADDR environment variable if set.
`
	ArmorCmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", httpAddrDesc)
	viper.BindPFlag("httpAddr", ArmorCmd.PersistentFlags().Lookup("http-addr"))
	viper.BindEnv("httpAddr", "ARMOR_HTTP_ADDR")
	viper.SetDefault("httpAddr", ":8081")

	// grpc address
	grpcAddrDesc := `gRPC listen address. (default ":8082")
                                 Overrides the ARMOR_GRPC_ADDR environment variable if set.
									 `
	ArmorCmd.PersistentFlags().StringVar(&grpcAddr, "grpc-addr", "", grpcAddrDesc)
	viper.BindPFlag("grpcAddr", ArmorCmd.PersistentFlags().Lookup("grpc-addr"))
	viper.BindEnv("grpcAddr", "ARMOR_GRPC_ADDR")
	viper.SetDefault("grpcAddr", ":8082")

	// Appdash perf tracing
	appdashAddrDesc := `Enable Appdash tracing via an Appdash server host:port.
                                 Overrides the ARMOR_APPDASH_ADDR environment variable if set.
																 `
	ArmorCmd.PersistentFlags().StringVar(&appdashAddr, "appdash-addr", "", appdashAddrDesc)
	viper.BindPFlag("appdashAddr", ArmorCmd.PersistentFlags().Lookup("appdash-addr"))
	viper.BindEnv("appdashAddr", "ARMOR_APPDASH_ADDR")
	viper.SetDefault("appdashAddr", "")

	// LightStep distributed tracing
	lightstepTokenDesc := `Enable LightStep tracing via a LightStep access token.
                                 Overrides the ARMOR_LIGHTSTEP_TOKEN environment variable if set.
																	`
	ArmorCmd.PersistentFlags().StringVar(&lightstepToken, "lightstep-token", "", lightstepTokenDesc)
	viper.BindPFlag("lightstepToken", ArmorCmd.PersistentFlags().Lookup("lightstep-token"))
	viper.BindEnv("lightstepToken", "ARMOR_LIGHTSTEP_TOKEN")
	viper.SetDefault("lightstepToken", "")

	// Vault server listen address
	vaultAddrDesc := `The address of the Vault server. (default "https://127.0.0.1:8200")
                                 Overrides the ARMOR_VAULT_ADDR environment variable if set
													 `
	ArmorCmd.PersistentFlags().StringVar(&vaultAddr, "vault-addr", "", vaultAddrDesc)
	viper.BindPFlag("vaultAddr", ArmorCmd.PersistentFlags().Lookup("vault-addr"))
	viper.BindEnv("vaultAddr", "ARMOR_VAULT_ADDR")
	viper.SetDefault("vaultAddr", "https://127.0.0.1:8200")

	// Vault server ca cert
	vaultCACertDesc := `Path to PEM encoded CA cert file.
                                 Overrides the ARMOR_VAULT_CA_CERT environment variable if set.
														`
	ArmorCmd.PersistentFlags().StringVar(&vaultCACert, "vault-ca-cert", "", vaultCACertDesc)
	viper.BindPFlag("vaultCACert", ArmorCmd.PersistentFlags().Lookup("vault-ca-cert"))
	viper.BindEnv("vaultCACert", "ARMOR_VAULT_CA_CERT")
	viper.SetDefault("vaultCACert", "")

	// Vault server ca path
	vaultCAPathDesc := `Path to a directory of PEM encoded CA cert files.
                                 Overrides the ARMOR_VAULT_CA_PATH environment variable if set.
															 `
	ArmorCmd.PersistentFlags().StringVar(&vaultCAPath, "vault-ca-path", "", vaultCAPathDesc)
	viper.BindPFlag("vaultCAPath", ArmorCmd.PersistentFlags().Lookup("vault-ca-path"))
	viper.BindEnv("vaultCAPath", "ARMOR_VAULT_CA_PATH")
	viper.SetDefault("vaultCAPath", "")

	// Vault server skip tls verification
	vaultTLSSkipVerifyDesc := `Do not verify TLS certificate. This is highly not recommended. (default false)
	                         Verification will also be skipped if the ARMOR_VAULT_SKIP_VERIFY 
                                 environment variable is set.
																	 `
	ArmorCmd.PersistentFlags().BoolVar(&vaultTLSSkipVerify, "vault-skip-verify", false, vaultTLSSkipVerifyDesc)
	viper.BindPFlag("vaultTLSSkipVerify", ArmorCmd.PersistentFlags().Lookup("vault-skip-verify"))
	viper.BindEnv("vaultTLSSkipVerify", "ARMOR_VAULT_SKIP_VERIFY")
	viper.SetDefault("vaultTLSSkipVerify", false)

	// Armor configuration file location
	cfgFileDesc := `config file (default is path/armor.yaml|json|toml) NOT YET SUPPORTED!!
	`
	ArmorCmd.PersistentFlags().StringVar(&cfgFile, "config", "", cfgFileDesc)
	viper.BindPFlag("cfgFile", ArmorCmd.PersistentFlags().Lookup("config"))
	viper.BindEnv("cfgFile", "ARMOR_CONFIG")
	viper.SetDefault("cfgFile", "")

	// Set bash-completion
	validConfigFilenames := []string{"json", "js", "yaml", "yml", "toml", "tml"}
	ArmorCmd.PersistentFlags().SetAnnotation("config", cobra.BashCompFilenameExt, validConfigFilenames)
}

func init() {
	viperConfigSettings()
	initRootPersistentFlags()
	armorCmdV = ArmorCmd
}

// InitializeConfig initializes config with sensible default
// configuration flags.
func InitializeConfig(subCmdVs ...*cobra.Command) error {
	if err := loadGlobalConfig(cfgFile); err != nil {
		return err
	}

	for _, cmdV := range append([]*cobra.Command{armorCmdV}, subCmdVs...) {

		if flagChanged(cmdV.PersistentFlags(), "debugAddr") {
			viper.Set("debugAddr", debugAddr)
		}
		if flagChanged(cmdV.PersistentFlags(), "httpAddr") {
			viper.Set("httpAddr", httpAddr)
		}
		if flagChanged(cmdV.PersistentFlags(), "grpcAddr") {
			viper.Set("grpcAddr", grpcAddr)
		}
		if flagChanged(cmdV.PersistentFlags(), "appdashAddr") {
			viper.Set("appdashAddr", appdashAddr)
		}
		if flagChanged(cmdV.PersistentFlags(), "lightstepToken") {
			viper.Set("lightstepToken", lightstepToken)
		}
		if flagChanged(cmdV.PersistentFlags(), "vaultAddr") {
			viper.Set("vaultAddr", vaultAddr)
		}
		if flagChanged(cmdV.PersistentFlags(), "vaultCACert") {
			viper.Set("vaultCACert", vaultCACert)
		}
		if flagChanged(cmdV.PersistentFlags(), "vaultCAPath") {
			viper.Set("vaultCAPath", vaultCAPath)
		}
		if flagChanged(cmdV.PersistentFlags(), "vaultTLSSkipVerify") {
			viper.Set("vaultTLSSkipVerify", vaultTLSSkipVerify)
		}

	}

	return nil
}

func loadGlobalConfig(configFilename string) error {
	viper.SetConfigFile(configFilename)
	viper.AddConfigPath(".")

	dir, err := homedir.Dir()
	if err != nil {
		return err
	}
	viper.AddConfigPath(dir)

	err = viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigParseError); ok {
			return err
		}
		// For now, let's not assume we require a config file...
		//return fmt.Errorf("Unable to locate Config file. Perhaps you need to
		//create a new config file.\n       Run `armor help config` for details. (%s)\n", err)
	}

	return nil
}

func viperConfigSettings() {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("armor")
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)
}

func flagChanged(flags *flag.FlagSet, key string) bool {
	flag := flags.Lookup(key)
	if flag == nil {
		return false
	}
	return flag.Changed
}

// Start initializes the config and then starts the services based
// configuration provided.
func Start(cmd *cobra.Command, args []string) error {
	if err := InitializeConfig(); err != nil {
		return err
	}

	startServices()
	return nil
}

func startServices() {
	var (
		vdebugAddr      = viper.GetString("debugAddr")
		vhttpAddr       = viper.GetString("httpAddr")
		vgrpcAddr       = viper.GetString("grpcAddr")
		vappdashAddr    = viper.GetString("appdashAddr")
		vlightstepToken = viper.GetString("lightstepToken")
	)

	// Logging domain.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stdout)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}
	logger.Log("msg", "hello")
	defer logger.Log("msg", "goodbye")

	// Metrics domain.
	fieldKeys := []string{"method", "error"}
	var requestCount metrics.Counter
	{
		// Business level metrics.
		requestCount = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "mystique",
			Subsystem: "vault_proxy",
			Name:      "request_count",
			Help:      "Number of requests received.",
		}, fieldKeys)
	}
	var requestLatency metrics.Histogram
	{
		// Transport level metrics
		requestLatency = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "mystique",
			Subsystem: "vault_proxy",
			Name:      "request_latency_microseconds",
			Help:      "Total duration of requests in microseconds.",
		}, fieldKeys)
	}
	var duration metrics.Histogram
	{
		// Transport level metrics
		duration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "mystique",
			Subsystem: "vault_proxy",
			Name:      "request_duration_ns",
			Help:      "Request duration in nanoseconds.",
		}, []string{"method", "success"})

	}

	// Tracing domain.
	var tracer stdopentracing.Tracer
	{
		if vappdashAddr != "" {
			logger := log.NewContext(logger).With("tracer", "Appdash")
			logger.Log("addr", vappdashAddr)
			tracer = appdashot.NewTracer(appdash.NewRemoteCollector(vappdashAddr))
		} else if vlightstepToken != "" {
			logger := log.NewContext(logger).With("tracer", "LightStep")
			logger.Log() // probably don't want to print out the token :)
			tracer = lightstep.NewTracer(lightstep.Options{
				AccessToken: vlightstepToken,
			})
			defer lightstep.FlushLightStepTracer(tracer)
		} else {
			logger := log.NewContext(logger).With("tracer", "none")
			logger.Log()
			tracer = stdopentracing.GlobalTracer() // no-op
		}
	}

	// Business domain.
	svc := service.New(logger, requestCount, requestLatency)
	// Endpoint domain.
	eps := endpoints.New(svc, logger, duration, tracer)

	// Mechanical domain.
	errc := make(chan error)
	ctx := context.Background()

	// Interrupt handler.
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// Debug listener.
	go func() {
		logger := log.NewContext(logger).With("transport", "debug")

		m := http.NewServeMux()
		m.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		m.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		m.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		m.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		m.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
		m.Handle("/metrics", stdprometheus.Handler())

		logger.Log("addr", vdebugAddr)
		errc <- http.ListenAndServe(vdebugAddr, m)
	}()

	// Mechanical domain.
	// HTTP transport.
	go func() {
		logger := log.NewContext(logger).With("transport", "HTTP")
		mux := vaulthttp.NewHandler(ctx, eps, tracer, logger)
		logger.Log("addr", vhttpAddr)
		errc <- http.ListenAndServe(vhttpAddr, mux)
	}()

	// gRPC transport.
	go func() {
		logger := log.NewContext(logger).With("transport", "gRPC")

		ln, err := net.Listen("tcp", vgrpcAddr)
		if err != nil {
			errc <- err
			return
		}

		srv := vaultgrpc.NewHandler(ctx, eps, tracer, logger)
		s := grpc.NewServer()
		pb.RegisterVaultServer(s, srv)

		logger.Log("addr", vgrpcAddr)
		errc <- s.Serve(ln)
	}()

	// Run!
	logger.Log("exit", <-errc)
}
