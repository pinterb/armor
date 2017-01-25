// Package http provides an HTTP client for the Vault proxy service.
package http

import (
	"net/url"
	"strings"
	"time"

	jujuratelimit "github.com/juju/ratelimit"
	stdopentracing "github.com/opentracing/opentracing-go"
	"github.com/sony/gobreaker"

	vaultendpoints "github.com/cdwlabs/armor/pkg/proxy/endpoints"
	vaulthttp "github.com/cdwlabs/armor/pkg/proxy/http"
	vaultservice "github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/tracing/opentracing"
	httptransport "github.com/go-kit/kit/transport/http"
)

// New creates http client
func New(instance string, tracer stdopentracing.Tracer, logger log.Logger) (vaultservice.Service, error) {
	if !strings.HasPrefix(instance, "http") {
		instance = "http://" + instance
	}
	u, err := url.Parse(instance)
	if err != nil {
		return nil, err
	}

	// We construct a single ratelimiter middleware, to limit the total outgoing
	// QPS from this client to all methods on the remote instance.  We also
	// construct per-endpoint circuitbreaker middleware to demonstrate how that's
	// done, although they could easily be cominbed into a single breaker for
	// then entire remote instance.
	limiter := ratelimit.NewTokenBucketLimiter(jujuratelimit.NewBucketWithRate(100, 100))

	var initStatusEndpoint endpoint.Endpoint
	{
		initStatusEndpoint = httptransport.NewClient(
			"GET",
			copyURL(u, "/init/status"),
			vaulthttp.EncodeGenericRequest,
			vaulthttp.DecodeInitStatusResponse,
			httptransport.ClientBefore(opentracing.ToHTTPRequest(tracer, logger)),
		).Endpoint()
		initStatusEndpoint = opentracing.TraceClient(tracer, "InitStatus")(initStatusEndpoint)
		initStatusEndpoint = limiter(initStatusEndpoint)
		initStatusEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "InitStatus",
			Timeout: 30 * time.Second,
		}))(initStatusEndpoint)
	}

	var initEndpoint endpoint.Endpoint
	{
		initEndpoint = httptransport.NewClient(
			"PUT",
			copyURL(u, "/init"),
			vaulthttp.EncodeInitRequest,
			vaulthttp.DecodeInitResponse,
			httptransport.ClientBefore(opentracing.ToHTTPRequest(tracer, logger)),
		).Endpoint()
		initEndpoint = opentracing.TraceClient(tracer, "Init")(initEndpoint)
		initEndpoint = limiter(initEndpoint)
		initEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Init",
			Timeout: 30 * time.Second,
		}))(initEndpoint)
	}

	var sealStatusEndpoint endpoint.Endpoint
	{
		sealStatusEndpoint = httptransport.NewClient(
			"GET",
			copyURL(u, "/seal/status"),
			vaulthttp.EncodeGenericRequest,
			vaulthttp.DecodeSealStatusResponse,
			httptransport.ClientBefore(opentracing.ToHTTPRequest(tracer, logger)),
		).Endpoint()
		sealStatusEndpoint = opentracing.TraceClient(tracer, "SealStatus")(sealStatusEndpoint)
		sealStatusEndpoint = limiter(sealStatusEndpoint)
		sealStatusEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "SealStatus",
			Timeout: 30 * time.Second,
		}))(sealStatusEndpoint)
	}

	var unsealEndpoint endpoint.Endpoint
	{
		unsealEndpoint = httptransport.NewClient(
			"PUT",
			copyURL(u, "/unseal"),
			vaulthttp.EncodeGenericRequest,
			vaulthttp.DecodeUnsealResponse,
			httptransport.ClientBefore(opentracing.ToHTTPRequest(tracer, logger)),
		).Endpoint()
		unsealEndpoint = opentracing.TraceClient(tracer, "Unseal")(unsealEndpoint)
		unsealEndpoint = limiter(unsealEndpoint)
		unsealEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Unseal",
			Timeout: 30 * time.Second,
		}))(unsealEndpoint)
	}

	var configureEndpoint endpoint.Endpoint
	{
		configureEndpoint = httptransport.NewClient(
			"POST",
			copyURL(u, "/configure"),
			vaulthttp.EncodeGenericRequest,
			vaulthttp.DecodeConfigureResponse,
			httptransport.ClientBefore(opentracing.ToHTTPRequest(tracer, logger)),
		).Endpoint()
		configureEndpoint = opentracing.TraceClient(tracer, "Configure")(configureEndpoint)
		configureEndpoint = limiter(configureEndpoint)
		configureEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Configure",
			Timeout: 30 * time.Second,
		}))(configureEndpoint)
	}

	return vaultendpoints.Endpoints{
		InitStatusEndpoint: initStatusEndpoint,
		InitEndpoint:       initEndpoint,
		SealStatusEndpoint: sealStatusEndpoint,
		UnsealEndpoint:     unsealEndpoint,
		ConfigureEndpoint:  configureEndpoint,
	}, nil
}

func copyURL(base *url.URL, path string) *url.URL {
	next := *base
	next.Path = path
	return &next
}
