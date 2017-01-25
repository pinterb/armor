package endpoints

// This file contains methods to make individual endpoints from services,
// request and response types to serve those endpoints, as well as encoders
// and decoders for those types, for all of our supported transport
// serialization formats. It also includes endpoint middlewares.

import (
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
	"golang.org/x/net/context"
)

// New returns an Endpoints that wraps the provided server, and wires in all of
// the expected endpoint middlewares via the various parameters.
func New(svc service.Service, logger log.Logger, duration metrics.Histogram, trace stdopentracing.Tracer) Endpoints {
	var initStatusEndpoint endpoint.Endpoint
	{
		initStatusEndpoint = MakeInitStatusEndpoint(svc)
		initStatusEndpoint = opentracing.TraceServer(trace, "InitStatus")(initStatusEndpoint)
		initStatusEndpoint = ratelimit.NewTokenBucketLimiter(rl.NewBucketWithRate(100, 100))(initStatusEndpoint)
		initStatusEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(initStatusEndpoint)
		initStatusEndpoint = LoggingMiddleware(log.NewContext(logger).With("method", "InitStatus"))(initStatusEndpoint)
		initStatusEndpoint = InstrumentingMiddleware(duration.With("method", "InitStatus"))(initStatusEndpoint)
	}
	var initEndpoint endpoint.Endpoint
	{
		initEndpoint = MakeInitEndpoint(svc)
		initEndpoint = opentracing.TraceServer(trace, "Init")(initEndpoint)
		initEndpoint = ratelimit.NewTokenBucketLimiter(rl.NewBucketWithRate(100, 100))(initEndpoint)
		initEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(initEndpoint)
		initEndpoint = LoggingMiddleware(log.NewContext(logger).With("method", "Init"))(initEndpoint)
		initEndpoint = InstrumentingMiddleware(duration.With("method", "Init"))(initEndpoint)
	}
	var sealStatusEndpoint endpoint.Endpoint
	{
		sealStatusEndpoint = MakeSealStatusEndpoint(svc)
		sealStatusEndpoint = opentracing.TraceServer(trace, "SealStatus")(sealStatusEndpoint)
		sealStatusEndpoint = ratelimit.NewTokenBucketLimiter(rl.NewBucketWithRate(100, 100))(sealStatusEndpoint)
		sealStatusEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(sealStatusEndpoint)
		sealStatusEndpoint = LoggingMiddleware(log.NewContext(logger).With("method", "SealStatus"))(sealStatusEndpoint)
		sealStatusEndpoint = InstrumentingMiddleware(duration.With("method", "SealStatus"))(sealStatusEndpoint)
	}
	var unsealEndpoint endpoint.Endpoint
	{
		unsealEndpoint = MakeUnsealEndpoint(svc)
		unsealEndpoint = opentracing.TraceServer(trace, "Unseal")(unsealEndpoint)
		unsealEndpoint = ratelimit.NewTokenBucketLimiter(rl.NewBucketWithRate(100, 100))(unsealEndpoint)
		unsealEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(unsealEndpoint)
		unsealEndpoint = LoggingMiddleware(log.NewContext(logger).With("method", "Unseal"))(unsealEndpoint)
		unsealEndpoint = InstrumentingMiddleware(duration.With("method", "Unseal"))(unsealEndpoint)
	}
	var configureEndpoint endpoint.Endpoint
	{
		configureEndpoint = MakeConfigureEndpoint(svc)
		configureEndpoint = opentracing.TraceServer(trace, "Configure")(configureEndpoint)
		configureEndpoint = ratelimit.NewTokenBucketLimiter(rl.NewBucketWithRate(100, 100))(configureEndpoint)
		configureEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(configureEndpoint)
		configureEndpoint = LoggingMiddleware(log.NewContext(logger).With("method", "Configure"))(configureEndpoint)
		configureEndpoint = InstrumentingMiddleware(duration.With("method", "Configure"))(configureEndpoint)
	}

	return Endpoints{
		InitStatusEndpoint: initStatusEndpoint,
		InitEndpoint:       initEndpoint,
		SealStatusEndpoint: sealStatusEndpoint,
		UnsealEndpoint:     unsealEndpoint,
		ConfigureEndpoint:  configureEndpoint,
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
	InitEndpoint       endpoint.Endpoint
	SealStatusEndpoint endpoint.Endpoint
	UnsealEndpoint     endpoint.Endpoint
	ConfigureEndpoint  endpoint.Endpoint
}

// InitStatus implements Service. Primarily useful in a client
func (e Endpoints) InitStatus(ctx context.Context) (bool, error) {
	request := InitStatusRequest{}
	response, err := e.InitStatusEndpoint(ctx, request)
	if err != nil {
		return false, err
	}
	return response.(InitStatusResponse).Initialized, response.(InitStatusResponse).Err
}

// MakeInitStatusEndpoint returns an endpoint that invokes InitStatus on the
// service.  Primarily useful in a server.
func MakeInitStatusEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		initialized, err := s.InitStatus(ctx)
		return InitStatusResponse{
			Initialized: initialized,
			Err:         err,
		}, nil
	}
}

// Init implements Service. Primarily useful in a client
func (e Endpoints) Init(ctx context.Context, opts service.InitOptions) (service.InitKeys, error) {
	request := InitRequest{Opts: opts}
	response, err := e.InitEndpoint(ctx, request)
	if err != nil {
		return service.InitKeys{}, err
	}
	return response.(InitResponse).Init, response.(InitResponse).Err
}

// MakeInitEndpoint returns an endpoint that invokes Init on the
// service.  Primarily useful in a server.
func MakeInitEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		var req = *request.(*InitRequest)
		initResp, err := s.Init(ctx, req.Opts)
		return InitResponse{
			Init: initResp,
			Err:  err,
		}, nil
	}
}

// SealStatus implements Service. Primarily useful in a client
func (e Endpoints) SealStatus(ctx context.Context) (service.SealState, error) {
	request := SealStatusRequest{}
	response, err := e.SealStatusEndpoint(ctx, request)
	if err != nil {
		return service.SealState{}, err
	}

	state := service.SealState{
		Sealed:      response.(SealStatusResponse).Sealed,
		T:           response.(SealStatusResponse).T,
		N:           response.(SealStatusResponse).N,
		Progress:    response.(SealStatusResponse).Progress,
		ClusterName: response.(SealStatusResponse).ClusterName,
		ClusterID:   response.(SealStatusResponse).ClusterID,
	}
	return state, response.(SealStatusResponse).Err
}

// MakeSealStatusEndpoint returns an endpoint that invokes SealStatus on the
// service.  Primarily useful in a server.
func MakeSealStatusEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		state, err := s.SealStatus(ctx)
		return SealStatusResponse{
			Sealed:      state.Sealed,
			T:           state.T,
			N:           state.N,
			Progress:    state.Progress,
			Version:     state.Version,
			ClusterName: state.ClusterName,
			ClusterID:   state.ClusterID,
			Err:         err,
		}, nil
	}
}

// Unseal implements Service. Primarily useful in a client
func (e Endpoints) Unseal(ctx context.Context, opts service.UnsealOptions) (service.SealState, error) {
	request := UnsealRequest{Key: opts.Key, Reset: opts.Reset}
	response, err := e.UnsealEndpoint(ctx, request)
	if err != nil {
		return service.SealState{}, err
	}

	state := service.SealState{
		Sealed:      response.(UnsealResponse).Sealed,
		T:           response.(UnsealResponse).T,
		N:           response.(UnsealResponse).N,
		Progress:    response.(UnsealResponse).Progress,
		ClusterName: response.(UnsealResponse).ClusterName,
		ClusterID:   response.(UnsealResponse).ClusterID,
	}
	return state, response.(UnsealResponse).Err
}

// MakeUnsealEndpoint returns an endpoint that invokes Unseal on the
// service.  Primarily useful in a server.
func MakeUnsealEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		var req = *request.(*UnsealRequest)
		opts := service.UnsealOptions{
			Key:   req.Key,
			Reset: req.Reset,
		}

		state, err := s.Unseal(ctx, opts)
		return UnsealResponse{
			Sealed:      state.Sealed,
			T:           state.T,
			N:           state.N,
			Progress:    state.Progress,
			Version:     state.Version,
			ClusterName: state.ClusterName,
			ClusterID:   state.ClusterID,
			Err:         err,
		}, nil
	}
}

// Configure implements Service. Primarily useful in a client
func (e Endpoints) Configure(ctx context.Context, opts service.ConfigOptions) (service.ConfigState, error) {
	request := ConfigureRequest{URL: opts.URL, Token: opts.Token}
	response, err := e.ConfigureEndpoint(ctx, request)
	if err != nil {
		return service.ConfigState{}, err
	}

	// mounts
	var mounts map[string]service.MountOutput
	if (response.(ConfigureResponse).Mounts != nil) && (len(response.(ConfigureResponse).Mounts) > 0) {
		mounts = make(map[string]service.MountOutput)
		for k, v := range response.(ConfigureResponse).Mounts {
			mountCfgOut := service.MountConfigOutput{
				DefaultLeaseTTL: v.Config.DefaultLeaseTTL,
				MaxLeaseTTL:     v.Config.MaxLeaseTTL,
			}

			mountOut := service.MountOutput{
				Type:        v.Type,
				Description: v.Description,
				Config:      mountCfgOut,
			}

			mounts[k] = mountOut
		}
	}

	// auths
	var auths map[string]service.AuthMountOutput
	if (response.(ConfigureResponse).Auths != nil) && (len(response.(ConfigureResponse).Auths) > 0) {
		auths = make(map[string]service.AuthMountOutput)
		for k, v := range response.(ConfigureResponse).Auths {
			cfgOut := service.AuthConfigOutput{
				DefaultLeaseTTL: v.Config.DefaultLeaseTTL,
				MaxLeaseTTL:     v.Config.MaxLeaseTTL,
			}

			authMountOut := service.AuthMountOutput{
				Type:        v.Type,
				Description: v.Description,
				Config:      cfgOut,
			}

			auths[k] = authMountOut
		}
	}

	// policies
	var policies []string
	if (response.(ConfigureResponse).Policies != nil) && (len(response.(ConfigureResponse).Policies) > 0) {
		policies = response.(ConfigureResponse).Policies
	}

	state := service.ConfigState{
		ConfigID: response.(ConfigureResponse).ConfigID,
		Mounts:   mounts,
		Auths:    auths,
		Policies: policies,
	}
	return state, response.(ConfigureResponse).Err
}

// MakeConfigureEndpoint returns an endpoint that invokes Configure on the
// service.  Primarily useful in a server.
func MakeConfigureEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		var req = *request.(*ConfigureRequest)
		opts := service.ConfigOptions{
			URL:   req.URL,
			Token: req.Token,
		}

		state, err := s.Configure(ctx, opts)

		// mounts
		var mounts map[string]MountOutput
		if (state.Mounts != nil) && (len(state.Mounts) > 0) {
			mounts = make(map[string]MountOutput)
			for k, v := range state.Mounts {
				mountCfgOut := MountConfigOutput{
					DefaultLeaseTTL: v.Config.DefaultLeaseTTL,
					MaxLeaseTTL:     v.Config.MaxLeaseTTL,
				}

				mountOut := MountOutput{
					Type:        v.Type,
					Description: v.Description,
					Config:      mountCfgOut,
				}

				mounts[k] = mountOut
			}
		}

		// auths
		var auths map[string]AuthMountOutput
		if (state.Auths != nil) && (len(state.Auths) > 0) {
			auths = make(map[string]AuthMountOutput)
			for k, v := range state.Auths {
				cfgOut := AuthConfigOutput{
					DefaultLeaseTTL: v.Config.DefaultLeaseTTL,
					MaxLeaseTTL:     v.Config.MaxLeaseTTL,
				}

				authMountOut := AuthMountOutput{
					Type:        v.Type,
					Description: v.Description,
					Config:      cfgOut,
				}

				auths[k] = authMountOut
			}
		}

		// policies
		var policies []string
		if (state.Policies != nil) && (len(state.Policies) > 0) {
			policies = state.Policies
		}

		return ConfigureResponse{
			ConfigID: state.ConfigID,
			Mounts:   mounts,
			Auths:    auths,
			Policies: policies,
			Err:      err,
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

// InitStatusResponse collects the response values for the InitStatus method.
type InitStatusResponse struct {
	Initialized bool  `json:"initialized"`
	Err         error `json:"-"` // should be intercepted by Failed/errorEncoder
}

// Failed implements Failer.
func (r InitStatusResponse) Failed() error { return r.Err }

// InitRequest collects the request parameters (if any) for the
// Init method.
type InitRequest struct {
	Opts service.InitOptions
}

// InitResponse collects the response values for the Init method.
type InitResponse struct {
	Init service.InitKeys `json:"init"`
	Err  error            `json:"-"` // should be intercepted by Failed/errorEncoder
}

// Failed implements Failer.
func (r InitResponse) Failed() error { return r.Err }

// SealStatusRequest collects the request parameters (if any) for the
// SealStatus method.
type SealStatusRequest struct{}

// SealStatusResponse collects the response values for the SealStatus method.
type SealStatusResponse struct {
	Sealed      bool   `json:"sealed"`
	T           int    `json:"t"`
	N           int    `json:"n"`
	Progress    int    `json:"progress"`
	Version     string `json:"version"`
	ClusterName string `json:"cluster_name,omitempty"`
	ClusterID   string `json:"cluster_id,omitempty"`
	Err         error  `json:"-"` // should be intercepted by Failed/errorEncoder
}

// Failed implements Failer.
func (r SealStatusResponse) Failed() error { return r.Err }

// UnsealRequest collects the request parameters (if any) for the
// Unseal method.
type UnsealRequest struct {
	Key   string
	Reset bool
}

// UnsealResponse collects the response values for the Unseal method.
type UnsealResponse struct {
	Sealed      bool   `json:"sealed"`
	T           int    `json:"t"`
	N           int    `json:"n"`
	Progress    int    `json:"progress"`
	Version     string `json:"version"`
	ClusterName string `json:"cluster_name,omitempty"`
	ClusterID   string `json:"cluster_id,omitempty"`
	Err         error  `json:"-"` // should be intercepted by Failed/errorEncoder
}

// Failed implements Failer.
func (r UnsealResponse) Failed() error { return r.Err }

// ConfigureRequest collects the request parameters (if any) for the Configure
// method.
type ConfigureRequest struct {
	URL   string
	Token string
}

// ConfigureResponse collects the response values for the Configure method.
type ConfigureResponse struct {
	ConfigID string                     `json:"config_id,omitempty"`
	Mounts   map[string]MountOutput     `json:"mounts,omitempty"`
	Auths    map[string]AuthMountOutput `json:"auths,omitempty"`
	Policies []string                   `json:"policies,omitempty"`
	Err      error                      `json:"-"` // should be intercepted by Failed/errorEncoder
}

// Failed implements Failer.
func (r ConfigureResponse) Failed() error { return r.Err }

// MountOutput maps directly to Vault's own MountOutput. Used by ConfigState to
// describe the mounts currently defined in a Vault instance.
type MountOutput struct {
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Config      MountConfigOutput `json:"config,omitempty"`
}

// MountConfigOutput describes the lease details of an individual mount.
type MountConfigOutput struct {
	DefaultLeaseTTL int `json:"default_lease_ttl,omitempty"`
	MaxLeaseTTL     int `json:"max_lease_ttl,omitempty"`
}

// AuthMountOutput maps directly to Vault's own AuthMount. Used by ConfigState to
// describe the auth backends currently defined in a Vault instance.
type AuthMountOutput struct {
	Type        string           `json:"type"`
	Description string           `json:"description,omitempty"`
	Config      AuthConfigOutput `json:"config,omitempty"`
}

// AuthConfigOutput describes the lease details of an auth backend.
type AuthConfigOutput struct {
	DefaultLeaseTTL int `json:"default_lease_ttl,omitempty"`
	MaxLeaseTTL     int `json:"max_lease_ttl,omitempty"`
}
