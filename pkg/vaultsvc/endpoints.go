package vaultsvc

// This file contains methods to make individual endpoints from services,
// request and response types to serve those endpoints, as well as encoders
// and decoders for those types, for all of our supported transport
// serialization formats. It also includes endpoint middlewares.

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
)

// Endpoints collects all of the endpoints that compose a vault service.  It's
// meant to be used as a helper struct, to collect all of the endpoints into a
// single parameter.
//
// In a server, it's useful for functions that need to operate on
// a per-endpoint basis.  For example, you might pass an Endpoints to
// a function that produces an http.Handler, with each method (endpoint) wired
// up to specific path. (It is probably a mistake in design to invoke the
// VaultService methods on the Endpoints struct in a server).
//
// In a client, it's useful to collect individually constructed endpoints into
// a single type that implements the VaultService interface.  For example, you
// might construct individual endpoints using transport/http.NewClient, combine
// them into an Endpoints, and return it to the caller as a VaultService.
type Endpoints struct {
	InitStatusEndpoint endpoint.Endpoint
}

// InitStatus implements VaultService. Primarily useful in a client
func (e Endpoints) InitStatus(ctx context.Context) (bool, error) {
	request := initStatusRequest{}
	response, err := e.InitStatusEndpoint(ctx, request)
	if err != nil {
		return false, err
	}
	return response.(initStatusResponse).V, response.(initStatusResponse).Err
}

// MakeInitStatusEndpoint returns an endpoint that invokes InitStatus on the
// service.  Primarily useful in a server.
func MakeInitStatusEndpoint(s VaultService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		v, err := s.InitStatus(ctx)
		return initStatusResponse{
			V:   v,
			Err: err,
		}, nil
	}
}

// EndpointLoggingMiddleware returns an endpoint middleware that logs the
// the duration of each invocation to the passed histogram. The middleware adds
// a single field: "success", which is "true" if no error is returned, and
// "false" otherwise.
func EndpointInstrumentingMiddleware(duration metrics.Histogram) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {

			defer func(begin time.Time) {
				duration.With("success", fmt.Sprint(err == nil)).Observe(time.Since(begin).Seconds())
			}(time.Now())
			return next(ctx, request)

		}
	}
}

// EndpointLoggingMiddleware returns an endpoint middleware that logs the
// duration of each invocation, and the resulting error, if any.
func EndpointLoggingMiddleware(logger log.Logger) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {

			defer func(begin time.Time) {
				logger.Log("error", err, "took", time.Since(begin))
			}(time.Now())
			return next(ctx, request)

		}
	}
}

// These types are unexported because they only exist to serve the endpoint
// domain, which is totally encapsulated in this package. They are otherwise
// opaque to all callers.

type initStatusRequest struct{}

type initStatusResponse struct {
	V   bool
	Err error
}
