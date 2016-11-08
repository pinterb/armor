package grpc

// This file provides server-side bindings for the gRPC transport. It utilizes
// the transport/grpc.Server.

import (
	"github.com/cdwlabs/armor/pb"
	"github.com/cdwlabs/armor/pkg/proxy/endpoints"
	"github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/tracing/opentracing"
	grpctransport "github.com/go-kit/kit/transport/grpc"
	stdopentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
)

// NewHandler makes a set of endpoints available as a gRPC AddServer.
// NOTE: At some point, request tracing support needs to be added. And since
// much of this service used example code from go-kit, the place to start for
// tracing support is probably here:
// https://github.com/go-kit/kit/blob/master/examples/addsvc/transport_grpc.go
func NewHandler(ctx context.Context, endpoints endpoints.Endpoints, tracer stdopentracing.Tracer, logger log.Logger) pb.VaultServer {
	options := []grpctransport.ServerOption{
		grpctransport.ServerErrorLogger(logger),
	}
	return &grpcServer{
		initstatus: grpctransport.NewServer(
			ctx,
			endpoints.InitStatusEndpoint,
			DecodeInitStatusRequest,
			EncodeInitStatusResponse,
			append(options, grpctransport.ServerBefore(opentracing.FromGRPCRequest(tracer, "InitStatus", logger)))...,
		),
	}
}

type grpcServer struct {
	initstatus grpctransport.Handler
}

func (s *grpcServer) InitStatus(ctx context.Context, req *pb.InitStatusRequest) (*pb.InitStatusResponse, error) {
	_, rep, err := s.initstatus.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*pb.InitStatusResponse), nil
}

// DecodeInitStatusRequest is a transport/grpc.DecodeRequestFunc that
// converts a gRPC initstatus request to a user-domain initstatus request. Primarily useful
// in a server.
func DecodeInitStatusRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	return endpoints.InitStatusRequest{}, nil
}

// DecodeInitStatusResponse is a transport/grpc.DecodeResponseFunc that
// converts a gRPC initstatus reply to a user-domain initstatus response. Primarily useful in
// a client.
func DecodeInitStatusResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*pb.InitStatusResponse)
	return endpoints.InitStatusResponse{V: bool(reply.Status.Initialized), Err: service.String2Error(reply.Err)}, nil
}

// EncodeInitStatusResponse is a transport/grpc.EncodeResponseFunc that
// converts a user-domain initstatus response to a gRPC initstatus reply. Primarily useful in
// a server.
func EncodeInitStatusResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoints.InitStatusResponse)
	status := &pb.Status{Initialized: bool(resp.V)}
	return &pb.InitStatusResponse{Status: status, Err: service.Error2String(resp.Err)}, nil
}

// EncodeInitStatusRequest is a transport/grpc.EncodeRequestFunc that
// converts a user-domain initstatus request to a gRPC sum request. Primarily useful
// in a client.
func EncodeInitStatusRequest(_ context.Context, request interface{}) (interface{}, error) {
	return &pb.InitStatusRequest{}, nil
}
