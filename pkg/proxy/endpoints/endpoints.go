package endpoints

// This file contains methods to make individual endpoints from services,
// request and response types to serve those endpoints, as well as encoders
// and decoders for those types, for all of our supported transport
// serialization formats. It also includes endpoint middlewares.

import (
	"golang.org/x/net/context"

	"github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/tracing/opentracing"
	rl "github.com/juju/ratelimit"
	stdopentracing "github.com/opentracing/opentracing-go"
	"github.com/sony/gobreaker"
)

// New returns an Endpoints that wraps the provided server, and wires in all of
// the expected endpoint middlewares via the various parameters.
func New(svc service.Service, logger log.Logger, duration metrics.Histogram, trace stdopentracing.Tracer) Endpoints {
	var initStatusEndpoint endpoint.Endpoint
	{
		initStatusEndpoint = MakeInitStatusEndpoint(svc)
		initStatusEndpoint = opentracing.TraceServer(trace, "InitStatus")(initStatusEndpoint)
		initStatusEndpoint = ratelimit.NewTokenBucketLimiter(rl.NewBucketWithRate(1, 1))(initStatusEndpoint)
		initStatusEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(initStatusEndpoint)
		initStatusEndpoint = LoggingMiddleware(log.NewContext(logger).With("method", "InitStatus"))(initStatusEndpoint)
		initStatusEndpoint = InstrumentingMiddleware(duration.With("method", "InitStatus"))(initStatusEndpoint)
	}

	return Endpoints{
		InitStatusEndpoint: initStatusEndpoint,
	}
}

// Endpoints collects all of the endpoints that compose a vault proxy service.  It's
// meant to be used as a helper struct, to collect all of the endpoints into a
// single parameter.
//
// In a server, it's useful for functions that need to operate on
// a per-endpoint basis.  For example, you might pass an Endpoints to
// a function that produces an http.Handler, with each method (endpoint) wired
// up to specific path. (It is probably a mistake in design to invoke the
// Service methods on the Endpoints struct in a server).
//
// In a client, it's useful to collect individually constructed endpoints into
// a single type that implements the Service interface.  For example, you
// might construct individual endpoints using transport/http.NewClient, combine
// them into an Endpoints, and return it to the caller as a Service.
type Endpoints struct {
	InitStatusEndpoint endpoint.Endpoint
}

// InitStatus implements Service. Primarily useful in a client
func (e Endpoints) InitStatus(ctx context.Context) (bool, error) {
	request := InitStatusRequest{}
	response, err := e.InitStatusEndpoint(ctx, request)
	if err != nil {
		return false, err
	}
	return response.(InitStatusResponse).V, response.(InitStatusResponse).Err
}

// MakeInitStatusEndpoint returns an endpoint that invokes InitStatus on the
// service.  Primarily useful in a server.
func MakeInitStatusEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		v, err := s.InitStatus(ctx)
		return InitStatusResponse{
			V:   v,
			Err: err,
		}, nil
	}
}

// Failer is an interface that should be implemented by response types.
// Response encoders can check if responses are Failer, and if so they've
// failed and should then encode them using a separate write path based on the
// error.
type Failer interface {
	Failed() error
}

// InitStatusRequest collects the request parameters (if any) for the
// InitStatus method.
type InitStatusRequest struct{}

// InitStatusResponse collects the response values for the InitiStatus method.
type InitStatusResponse struct {
	V   bool  `json:"v"`
	Err error `json:"-"` // should be intercepted by Failed/errorEncoder
}

// Failed implementes Failer.
func (r InitStatusResponse) Failed() error { return r.Err }
