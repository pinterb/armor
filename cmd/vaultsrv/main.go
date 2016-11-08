package main

import (
	"flag"
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

	"github.com/cdwlabs/armor/pb"
	"github.com/cdwlabs/armor/pkg/proxy/endpoints"
	vaultgrpc "github.com/cdwlabs/armor/pkg/proxy/grpc"
	vaulthttp "github.com/cdwlabs/armor/pkg/proxy/http"
	"github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
)

func main() {
	var (
		debugAddr      = flag.String("debug.addr", ":8080", "Debug and metrics listen address")
		httpAddr       = flag.String("http.addr", ":8081", "HTTP listen address")
		grpcAddr       = flag.String("grpc.addr", ":8082", "gRPC (HTTP) listen address")
		appdashAddr    = flag.String("appdash.addr", "", "Enable Appdash tracing via an Appdash server host:port")
		lightstepToken = flag.String("lightstep.token", "", "Enable LightStep tracing via a LightStep access token")
	)
	flag.Parse()

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
		if *appdashAddr != "" {
			logger := log.NewContext(logger).With("tracer", "Appdash")
			logger.Log("addr", *appdashAddr)
			tracer = appdashot.NewTracer(appdash.NewRemoteCollector(*appdashAddr))
		} else if *lightstepToken != "" {
			logger := log.NewContext(logger).With("tracer", "LightStep")
			logger.Log() // probably don't want to print out the token :)
			tracer = lightstep.NewTracer(lightstep.Options{
				AccessToken: *lightstepToken,
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
		//		m.Handle("/metrics", stdprometheus.Handler())

		logger.Log("addr", *debugAddr)
		errc <- http.ListenAndServe(*debugAddr, m)
	}()

	// Mechanical domain.
	// HTTP transport.
	go func() {
		logger := log.NewContext(logger).With("transport", "HTTP")
		mux := vaulthttp.NewHandler(ctx, eps, tracer, logger)
		logger.Log("addr", *httpAddr)
		errc <- http.ListenAndServe(*httpAddr, mux)
	}()

	// gRPC transport.
	go func() {
		logger := log.NewContext(logger).With("transport", "gRPC")

		ln, err := net.Listen("tcp", *grpcAddr)
		if err != nil {
			errc <- err
			return
		}

		srv := vaultgrpc.NewHandler(ctx, eps, tracer, logger)
		s := grpc.NewServer()
		pb.RegisterVaultServer(s, srv)

		logger.Log("addr", *grpcAddr)
		errc <- s.Serve(ln)
	}()

	// Run!
	logger.Log("exit", <-errc)
}
