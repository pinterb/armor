// Package grpc provides a gRPC client for the vault proxy service.
package grpc

import (
	"time"

	jujuratelimit "github.com/juju/ratelimit"
	stdopentracing "github.com/opentracing/opentracing-go"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"

	"github.com/cdwlabs/armor/pb"
	vaultendpoints "github.com/cdwlabs/armor/pkg/proxy/endpoints"
	vaultgrpc "github.com/cdwlabs/armor/pkg/proxy/grpc"
	vaultservice "github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/tracing/opentracing"
	grpctransport "github.com/go-kit/kit/transport/grpc"
)

// New returns a Vault proxy Service backed by a gRPC client connection.  It is
// the responsibility of the caller to dial, and later close, the connection.
func New(conn *grpc.ClientConn, tracer stdopentracing.Tracer, logger log.Logger) vaultservice.Service {
	// We construct a single ratelimiter middleware, to limit the total outgoing
	// QPS from this client to all methods on the remote instance.  We also
	// construct per-endpoint circuitbreaker middleware to demonstrate how that's
	// done, although they could easily be cominbed into a single breaker for
	// then entire remote instance.
	limiter := ratelimit.NewTokenBucketLimiter(jujuratelimit.NewBucketWithRate(100, 100))

	var initStatusEndpoint endpoint.Endpoint
	{
		initStatusEndpoint = grpctransport.NewClient(
			conn,
			"Vault",
			"InitStatus",
			vaultgrpc.EncodeInitStatusRequest,
			vaultgrpc.DecodeInitStatusResponse,
			pb.InitStatusResponse{},
			grpctransport.ClientBefore(opentracing.ToGRPCRequest(tracer, logger)),
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
	}
}
