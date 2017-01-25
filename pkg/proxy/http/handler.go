package http

// this file provides server-side bindings for the HTTP transport.
// It utilizes the transport/http.Server.

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"

	stdopentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"

	"github.com/cdwlabs/armor/pkg/proxy/endpoints"
	"github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/tracing/opentracing"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHandler returns a handler that makes a set of endpoints available on
// predefined paths.
func NewHandler(ctx context.Context, endpoints endpoints.Endpoints, tracer stdopentracing.Tracer, logger log.Logger) http.Handler {
	options := []httptransport.ServerOption{
		httptransport.ServerErrorEncoder(errorEncoder),
		httptransport.ServerErrorLogger(logger),
	}

	r := mux.NewRouter()
	r.Methods("GET").Path("/init/status").Handler(httptransport.NewServer(
		ctx,
		endpoints.InitStatusEndpoint,
		DecodeInitStatusRequest,
		EncodeGenericResponse,
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "InitStatus", logger)))...,
	))
	r.Methods("PUT").Path("/init").Handler(httptransport.NewServer(
		ctx,
		endpoints.InitEndpoint,
		DecodeInitRequest,
		EncodeGenericResponse,
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "Init", logger)))...,
	))
	r.Methods("GET").Path("/seal/status").Handler(httptransport.NewServer(
		ctx,
		endpoints.SealStatusEndpoint,
		DecodeSealStatusRequest,
		EncodeGenericResponse,
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "SealStatus", logger)))...,
	))
	r.Methods("PUT").Path("/unseal").Handler(httptransport.NewServer(
		ctx,
		endpoints.UnsealEndpoint,
		DecodeUnsealRequest,
		EncodeGenericResponse,
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "Unseal", logger)))...,
	))
	r.Methods("POST").Path("/configure").Handler(httptransport.NewServer(
		ctx,
		endpoints.ConfigureEndpoint,
		DecodeConfigureRequest,
		EncodeGenericResponse,
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "Configure", logger)))...,
	))
	r.Methods("GET").Path("/metrics").Handler(promhttp.Handler())

	return r
}

func errorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	w.WriteHeader(err2code(err))
	json.NewEncoder(w).Encode(errorWrapper{Error: err.Error()})
}

func err2code(err error) int {
	switch err {
	case service.ErrExample:
		return http.StatusBadRequest
	}
	switch e := err.(type) {
	case httptransport.Error:
		switch e.Domain {
		case httptransport.DomainDecode:
			return http.StatusBadRequest
		case httptransport.DomainDo:
			return err2code(e.Err)
		}
	}
	return http.StatusInternalServerError
}

func errorDecoder(r *http.Response) error {
	var w errorWrapper
	if err := json.NewDecoder(r.Body).Decode(&w); err != nil {
		return err
	}
	return errors.New(w.Error)
}

type errorWrapper struct {
	Error string `json:"error"`
}

// DecodeInitStatusRequest is a transport/http.DecodeRequestFunc that is
// basically a noop.  Normally, this method's default behavior is to decode
// a JSON-encoded request from the HTTP request body. Primarily useful in
// a server.
func DecodeInitStatusRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req = &endpoints.InitStatusRequest{}
	return req, nil
}

// DecodeInitStatusResponse is a transport/http.DecodeResponseFunc that
// decodes a JSON-encoded initStatus response from the HTTP response body. If the
// response has a non-200 status code, we will interpret that as an error and
// attempt to decode the specific error message from the response body.
// Primarily useful in a client.
func DecodeInitStatusResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errorDecoder(r)
	}
	var resp endpoints.InitStatusResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

// EncodeInitRequest is a transport/http.EncodeRequestFunc that
// JSON-encodes the init request to the request body. Primarily useful in a client.
func EncodeInitRequest(_ context.Context, r *http.Request, request interface{}) error {
	req := request.(endpoints.InitRequest)
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req.Opts); err != nil {
		return err
	}
	r.Body = ioutil.NopCloser(&buf)
	return nil
}

// DecodeInitRequest is a transport/http.DecodeRequestFunc that decodes
// a JSON-encoded init request from the HTTP request body. Primarily useful in
// a server.
func DecodeInitRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var opts = service.InitOptions{}
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		return &endpoints.InitRequest{}, err
	}
	return &endpoints.InitRequest{Opts: opts}, nil
}

// DecodeInitResponse is a transport/http.DecodeResponseFunc that
// decodes a JSON-encoded init response from the HTTP response body. If the
// response has a non-200 status code, we will interpret that as an error and
// attempt to decode the specific error message from the response body.
// Primarily useful in a client.
func DecodeInitResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errorDecoder(r)
	}
	var resp endpoints.InitResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

// DecodeSealStatusRequest is a transport/http.DecodeRequestFunc that is
// basically a noop.  Normally, this method's default behavior is to decode
// a JSON-encoded request from the HTTP request body. Primarily useful in
// a server.
func DecodeSealStatusRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req = &endpoints.SealStatusRequest{}
	return req, nil
}

// DecodeSealStatusResponse is a transport/http.DecodeResponseFunc that
// decodes a JSON-encoded sealstatus response from the HTTP response body. If the
// response has a non-200 status code, we will interpret that as an error and
// attempt to decode the specific error message from the response body.
// Primarily useful in a client.
func DecodeSealStatusResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errorDecoder(r)
	}
	var resp endpoints.SealStatusResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

// DecodeUnsealRequest is a transport/http.DecodeRequestFunc that decodes
// a JSON-encoded unseal request from the HTTP request body. Primarily useful in
// a server.
func DecodeUnsealRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var opts = service.UnsealOptions{}
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		return &endpoints.UnsealRequest{}, err
	}

	return &endpoints.UnsealRequest{Key: opts.Key, Reset: opts.Reset}, nil
}

// DecodeUnsealResponse is a transport/http.DecodeResponseFunc that
// decodes a JSON-encoded unseal response from the HTTP response body. If the
// response has a non-200 status code, we will interpret that as an error and
// attempt to decode the specific error message from the response body.
// Primarily useful in a client.
func DecodeUnsealResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errorDecoder(r)
	}
	var resp endpoints.UnsealResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

// DecodeConfigureRequest is a transport/http.DecodeRequestFunc that decodes
// a JSON-encoded configure request from the HTTP request body. Primarily useful in
// a server.
func DecodeConfigureRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var opts = service.ConfigOptions{}
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		return &endpoints.ConfigureRequest{}, err
	}

	return &endpoints.ConfigureRequest{URL: opts.URL, Token: opts.Token}, nil
}

// DecodeConfigureResponse is a transport/http.DecodeResponseFunc that
// decodes a JSON-encoded configure response from the HTTP response body. If the
// response has a non-200 status code, we will interpret that as an error and
// attempt to decode the specific error message from the response body.
// Primarily useful in a client.
func DecodeConfigureResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errorDecoder(r)
	}
	var resp endpoints.ConfigureResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

// EncodeGenericRequest is a transport/http.EncodeRequestFunc that
// JSON-encodes any request to the request body. Primarily useful in a client.
func EncodeGenericRequest(_ context.Context, r *http.Request, request interface{}) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(request); err != nil {
		return err
	}
	r.Body = ioutil.NopCloser(&buf)
	return nil
}

// EncodeGenericResponse is a transport/http.EncodeResponseFunc that
// encodes the response as JSON to the response writer. Primarily useful in
// a server.
func EncodeGenericResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	if f, ok := response.(endpoints.Failer); ok && f.Failed() != nil {
		errorEncoder(ctx, f.Failed(), w)
		return nil
	}
	return json.NewEncoder(w).Encode(response)
}
