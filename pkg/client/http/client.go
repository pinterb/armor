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

	return vaultendpoints.Endpoints{
		InitStatusEndpoint: initStatusEndpoint,
	}, nil
}

func copyURL(base *url.URL, path string) *url.URL {
	next := *base
	next.Path = path
	return &next
}
