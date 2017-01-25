package commands

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	lightstep "github.com/lightstep/lightstep-tracer-go"
	stdopentracing "github.com/opentracing/opentracing-go"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"sourcegraph.com/sourcegraph/appdash"
	appdashot "sourcegraph.com/sourcegraph/appdash/opentracing"

	"github.com/cdwlabs/armor/cmd/helpers"
	"github.com/cdwlabs/armor/pb"
	"github.com/cdwlabs/armor/pkg/config"
	"github.com/cdwlabs/armor/pkg/proxy/endpoints"
	armorgrpc "github.com/cdwlabs/armor/pkg/proxy/grpc"
	armorhealth "github.com/cdwlabs/armor/pkg/proxy/health"
	armorhttp "github.com/cdwlabs/armor/pkg/proxy/http"
	"github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/spf13/cobra"
	"regexp"
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

// Our Cobra root command and global config
var (
	cfg config.Provider
)

// Flags that are to be added to commands.
var (
	adminAddr          string
	httpAddr           string
	grpcAddr           string
	appdashAddr        string
	lightstepToken     string
	vaultAddr          string
	vaultCACert        string
	vaultCAPath        string
	vaultTLSSkipVerify bool
	cfgFile            string
	policyConfigPath   string
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
// Armor server.
func initRootPersistentFlags() {
	// admin address
	adminAddrDesc := fmt.Sprintf("Admin listen address. This address provides health, readiness, debug and metrics endpoints. Overrides the %s environment variable if set. (default \"%s\") NOT YET FULLY SUPPORTED!!!\n", config.AdminAddrEnvVar, config.AdminAddrDefault)
	ArmorCmd.PersistentFlags().StringVar(&adminAddr, "admin-address", "", adminAddrDesc)

	// http address
	httpAddrDesc := fmt.Sprintf("HTTP listen address. Overrides the %s environment variable if set. (default \"%s\")\n", config.HTTPAddrEnvVar, config.HTTPAddrDefault)
	ArmorCmd.PersistentFlags().StringVar(&httpAddr, "http-address", "", httpAddrDesc)

	// grpc address
	grpcAddrDesc := fmt.Sprintf("gRPC listen address. Overrides the %s environment variable if set. (default \"%s\")\n", config.GrpcAddrEnvVar, config.GrpcAddrDefault)
	ArmorCmd.PersistentFlags().StringVar(&grpcAddr, "grpc-address", "", grpcAddrDesc)

	// Appdash perf tracing
	appdashAddrDesc := fmt.Sprintf("Enable Appdash tracing via an Appdash server host:port. Overrides the %s environment variable if set.\n%s", config.AppDashAddrEnvVar, "")
	ArmorCmd.PersistentFlags().StringVar(&appdashAddr, "appdash-address", "", appdashAddrDesc)

	// LightStep distributed tracing
	lightstepTokenDesc := fmt.Sprintf("Enable LightStep tracing via a LightStep access token. Overrides the %s environment variable if set.\n%s", config.LightstepTokenEnvVar, "")
	ArmorCmd.PersistentFlags().StringVar(&lightstepToken, "lightstep-token", "", lightstepTokenDesc)

	// Vault server listen address
	vaultAddrDesc := fmt.Sprintf("The address of the Vault server. Overrides the %s environment variable if set. (default \"%s\")\n", config.VaultAddrEnvVar, config.VaultAddrDefault)
	ArmorCmd.PersistentFlags().StringVar(&vaultAddr, "vault-address", "", vaultAddrDesc)

	// Vault server ca cert
	vaultCACertDesc := fmt.Sprintf("Path to PEM encoded CA cert file. Overrides the %s environment variable if set.\n%s", config.VaultCACertEnvVar, "")
	ArmorCmd.PersistentFlags().StringVar(&vaultCACert, "vault-ca-cert", "", vaultCACertDesc)

	// Vault server ca path
	vaultCAPathDesc := fmt.Sprintf("Path to a directory of PEM encoded CA cert files. Overrides the %s environment variable if set. \n%s", config.VaultCAPathEnvVar, "")
	ArmorCmd.PersistentFlags().StringVar(&vaultCAPath, "vault-ca-path", "", vaultCAPathDesc)

	// Vault server skip tls verification
	vaultTLSSkipVerifyDesc := fmt.Sprintf("Do not verify TLS certificate. This is highly not recommended. Verification will also be skipped if the %s environment variable is set. (default %v)\n", config.VaultSkipVerifyEnvVar, config.VaultSkipVerifyDefault)
	ArmorCmd.PersistentFlags().BoolVar(&vaultTLSSkipVerify, "vault-skip-verify", config.VaultSkipVerifyDefault, vaultTLSSkipVerifyDesc)

	// Armor policy directory
	policyConfigDesc := fmt.Sprintf("Local download destination for configuring Vault with remote policy/configuration repositories. Overrides the %s environment variable if set. (default \"%s\")\n", config.PolicyConfigPathEnvVar, config.PolicyConfigPathDefault)
	ArmorCmd.PersistentFlags().StringVar(&policyConfigPath, "policy-config-dir", "", policyConfigDesc)

	// Armor configuration file location
	cfgFileDesc := fmt.Sprintf("Path to a yaml configuration file. Use this configuration file when you don't want to use CLI arguments or set environment variables. Overrides the %s environment variable if set. NOTE: Environment variables take precedence over the configuration file.  And CLI arguments take precedence over environment variables. \n", config.ArmorConfigFileEnvVar)
	ArmorCmd.PersistentFlags().StringVar(&cfgFile, "config", "", cfgFileDesc)

	// Set bash-completion
	validConfigFilenames := []string{"yaml", "yml"}
	ArmorCmd.PersistentFlags().SetAnnotation("config", cobra.BashCompFilenameExt, validConfigFilenames)
}

func init() {
	initRootPersistentFlags()
}

// Start initializes the config and then starts the services based
// configuration provided.
func Start(cmd *cobra.Command, args []string) error {
	var err error
	cfg, err = config.BindWithCobra(cmd)
	if err != nil {
		return err
	}

	startServices()
	return nil
}

func startServices() {
	var (
		vadminAddr      = cfg.GetString("admin_address")
		vhttpAddr       = cfg.GetString("http_address")
		vgrpcAddr       = cfg.GetString("grpc_address")
		vappdashAddr    = cfg.GetString("appdash_address")
		vlightstepToken = cfg.GetString("lightstep_token")
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
	errChan := make(chan error)
	ctx := context.Background()

	// Interrupt handler.
	//	go func() {
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)
	//errChan <- fmt.Errorf("%s", <-stopChan)
	//	}()

	// Admin listener.
	go func() {
		logger := log.NewContext(logger).With("transport", "admin")

		m := http.NewServeMux()
		m.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		m.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		m.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		m.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		m.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
		m.Handle("/metrics", stdprometheus.Handler())
		m.HandleFunc("/healthz", armorhealth.HealthzHandler)
		m.HandleFunc("/readiness", armorhealth.ReadinessHandler)
		m.HandleFunc("/healthz/status", armorhealth.HealthzStatusHandler)
		//m.HandleFunc("/readiness/status", armorhealth.ReadinessStatusHandler)

		logger.Log("addr", vadminAddr)
		errChan <- http.ListenAndServe(vadminAddr, m)
	}()

	// Mechanical domain.
	// HTTP transport.
	go func() {
		logger := log.NewContext(logger).With("transport", "HTTP")
		mux := armorhttp.NewHandler(ctx, eps, tracer, logger)
		logger.Log("addr", vhttpAddr)
		errChan <- http.ListenAndServe(vhttpAddr, mux)
	}()

	// gRPC transport.
	go func() {
		logger := log.NewContext(logger).With("transport", "gRPC")

		ln, err := net.Listen("tcp", vgrpcAddr)
		if err != nil {
			errChan <- err
			return
		}

		srv := armorgrpc.NewHandler(ctx, eps, tracer, logger)
		s := grpc.NewServer()
		pb.RegisterVaultServer(s, srv)

		logger.Log("addr", vgrpcAddr)
		errChan <- s.Serve(ln)
	}()

	// Run!
	//logger.Log("exit", <-errChan)
	for {
		select {
		case err := <-errChan:
			logger.Log("exit", err)
		case s := <-stopChan:
			logger.Log("msg", fmt.Sprintf("captured %v. exiting...goodbye", s))
			// TODO: graceful shutdown is not yet implemented. Wait 'till Golang v1.8
			// is released (est. Feb. 2017).
			// Example: https://tylerchr.blog/golang-18-whats-coming/
			//armorhealth.SetReadinessStatus(http.StatusServiceUnavailable)
			//httpServer.BlockingClose()
			os.Exit(0)
		}
	}
}
