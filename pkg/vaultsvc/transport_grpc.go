package vaultsvc

// This file provides server-side bindings for the gRPC transport. It utilizes
// the transport/grpc.Server.

import (
	stdopentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"

	"github.com/cdwlabs/armor/pb"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/tracing/opentracing"
	grpctransport "github.com/go-kit/kit/transport/grpc"
)

// MakeGRPCServer makes a set of endpoints available as a gRPC AddServer.
// NOTE: At some point, request tracing support needs to be added. And since
// much of this service used example code from go-kit, the place to start for
// tracing support is probably here:
// https://github.com/go-kit/kit/blob/master/examples/addsvc/transport_grpc.go
func MakeGRPCServer(ctx context.Context, endpoints Endpoints, tracer stdopentracing.Tracer, logger log.Logger) pb.VaultServer {
	options := []grpctransport.ServerOption{
		grpctransport.ServerErrorLogger(logger),
	}
	return &grpcServer{
		initstatus: grpctransport.NewServer(
			ctx,
			endpoints.InitStatusEndpoint,
			DecodeGRPCInitStatusRequest,
			EncodeGRPCInitStatusResponse,
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

// DecodeGRPCInitStatusRequest is a transport/grpc.DecodeRequestFunc that
// converts a gRPC initstatus request to a user-domain initstatus request. Primarily useful
// in a server.
func DecodeGRPCInitStatusRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	return initStatusRequest{}, nil
}

// DecodeGRPCInitStatusResponse is a transport/grpc.DecodeResponseFunc that
// converts a gRPC initstatus reply to a user-domain initstatus response. Primarily useful in
// a client.
func DecodeGRPCInitStatusResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*pb.InitStatusResponse)
	return initStatusResponse{V: bool(reply.Status.Initialized), Err: str2err(reply.Err)}, nil
}

// EncodeGRPCInitStatusResponse is a transport/grpc.EncodeResponseFunc that
// converts a user-domain initstatus response to a gRPC initstatus reply. Primarily useful in
// a server.
func EncodeGRPCInitStatusResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(initStatusResponse)
	status := &pb.Status{Initialized: bool(resp.V)}
	return &pb.InitStatusResponse{Status: status, Err: err2str(resp.Err)}, nil
}

// EncodeGRPCInitStatusRequest is a transport/grpc.EncodeRequestFunc that
// converts a user-domain initstatus request to a gRPC sum request. Primarily useful
// in a client.
func EncodeGRPCInitStatusRequest(_ context.Context, request interface{}) (interface{}, error) {
	return &pb.InitStatusRequest{}, nil
}
