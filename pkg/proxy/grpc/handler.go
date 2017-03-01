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

type grpcServer struct {
	initstatus grpctransport.Handler
	init       grpctransport.Handler
	sealstatus grpctransport.Handler
	unseal     grpctransport.Handler
	configure  grpctransport.Handler
}

// NewHandler makes a set of endpoints available as a gRPC Server.
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
		init: grpctransport.NewServer(
			ctx,
			endpoints.InitEndpoint,
			DecodeInitRequest,
			EncodeInitResponse,
			append(options, grpctransport.ServerBefore(opentracing.FromGRPCRequest(tracer, "Init", logger)))...,
		),
		sealstatus: grpctransport.NewServer(
			ctx,
			endpoints.SealStatusEndpoint,
			DecodeSealStatusRequest,
			EncodeSealStatusResponse,
			append(options, grpctransport.ServerBefore(opentracing.FromGRPCRequest(tracer, "SealStatus", logger)))...,
		),
		unseal: grpctransport.NewServer(
			ctx,
			endpoints.UnsealEndpoint,
			DecodeUnsealRequest,
			EncodeUnsealResponse,
			append(options, grpctransport.ServerBefore(opentracing.FromGRPCRequest(tracer, "Unseal", logger)))...,
		),
		configure: grpctransport.NewServer(
			ctx,
			endpoints.ConfigureEndpoint,
			DecodeConfigureRequest,
			EncodeConfigureResponse,
			append(options, grpctransport.ServerBefore(opentracing.FromGRPCRequest(tracer, "Configure", logger)))...,
		),
	}
}

func (s *grpcServer) InitStatus(ctx context.Context, req *pb.InitStatusRequest) (*pb.InitStatusResponse, error) {
	_, rep, err := s.initstatus.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*pb.InitStatusResponse), nil
}

func (s *grpcServer) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitResponse, error) {
	_, rep, err := s.init.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*pb.InitResponse), nil
}

func (s *grpcServer) SealStatus(ctx context.Context, req *pb.SealStatusRequest) (*pb.SealStatusResponse, error) {
	_, rep, err := s.sealstatus.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*pb.SealStatusResponse), nil
}

func (s *grpcServer) Unseal(ctx context.Context, req *pb.UnsealRequest) (*pb.UnsealResponse, error) {
	_, rep, err := s.unseal.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*pb.UnsealResponse), nil
}

func (s *grpcServer) Configure(ctx context.Context, req *pb.ConfigureRequest) (*pb.ConfigureResponse, error) {
	_, rep, err := s.configure.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*pb.ConfigureResponse), nil
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
	return endpoints.InitStatusResponse{Initialized: bool(reply.Status.Initialized), Err: service.String2Error(reply.Err)}, nil
}

// EncodeInitStatusResponse is a transport/grpc.EncodeResponseFunc that
// converts a user-domain initstatus response to a gRPC initstatus reply. Primarily useful in
// a server.
func EncodeInitStatusResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoints.InitStatusResponse)
	status := &pb.Status{Initialized: bool(resp.Initialized)}
	return &pb.InitStatusResponse{Status: status, Err: service.Error2String(resp.Err)}, nil
}

// EncodeInitStatusRequest is a transport/grpc.EncodeRequestFunc that
// converts a user-domain initstatus request to a gRPC initstatus request. Primarily useful
// in a client.
func EncodeInitStatusRequest(_ context.Context, request interface{}) (interface{}, error) {
	return &pb.InitStatusRequest{}, nil
}

// DecodeInitRequest is a transport/grpc.DecodeRequestFunc that
// converts a gRPC init request to a user-domain init request. Primarily useful
// in a server.
func DecodeInitRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*pb.InitRequest)
	opts := service.InitOptions{
		SecretShares:          int(req.SecretShares),
		SecretThreshold:       int(req.SecretThreshold),
		StoredShares:          int(req.StoredShares),
		PGPKeys:               req.PgpKeys,
		RecoveryShares:        int(req.RecoveryShares),
		RecoveryThreshold:     int(req.RecoveryThreshold),
		RecoveryPGPKeys:       req.RecoveryPgpKeys,
		RootTokenPGPKey:       req.RootTokenPgpKey,
		RootTokenHolderEmail:  req.RootTokenHolderEmail,
		SecretKeyHolderEmails: req.SecretKeyHolderEmails,
	}
	return &endpoints.InitRequest{Opts: opts}, nil
}

// DecodeInitResponse is a transport/grpc.DecodeResponseFunc that
// converts a gRPC init reply to a user-domain init response. Primarily useful in
// a client.
func DecodeInitResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*pb.InitResponse)
	init := service.InitKeys{
		Keys:            reply.Keys,
		KeysB64:         reply.KeysBase64,
		RecoveryKeys:    reply.RecoveryKeys,
		RecoveryKeysB64: reply.RecoveryKeysBase64,
		RootToken:       reply.RootToken,
	}
	return endpoints.InitResponse{Init: init, Err: service.String2Error(reply.Err)}, nil
}

// EncodeInitResponse is a transport/grpc.EncodeResponseFunc that
// converts a user-domain init response to a gRPC init reply. Primarily useful in
// a server.
func EncodeInitResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoints.InitResponse)

	return &pb.InitResponse{
		Keys:               resp.Init.Keys,
		KeysBase64:         resp.Init.KeysB64,
		RecoveryKeys:       resp.Init.RecoveryKeys,
		RecoveryKeysBase64: resp.Init.RecoveryKeysB64,
		RootToken:          resp.Init.RootToken,
		Err:                service.Error2String(resp.Err),
	}, nil
}

// EncodeInitRequest is a transport/grpc.EncodeRequestFunc that
// converts a user-domain init request to a gRPC init request. Primarily useful
// in a client.
func EncodeInitRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(endpoints.InitRequest)
	return &pb.InitRequest{
		SecretShares:          uint32(req.Opts.SecretShares),
		SecretThreshold:       uint32(req.Opts.SecretThreshold),
		StoredShares:          uint32(req.Opts.StoredShares),
		PgpKeys:               req.Opts.PGPKeys,
		RecoveryShares:        uint32(req.Opts.RecoveryShares),
		RecoveryThreshold:     uint32(req.Opts.RecoveryThreshold),
		RecoveryPgpKeys:       req.Opts.RecoveryPGPKeys,
		RootTokenPgpKey:       req.Opts.RootTokenPGPKey,
		RootTokenHolderEmail:  req.Opts.RootTokenHolderEmail,
		SecretKeyHolderEmails: req.Opts.SecretKeyHolderEmails,
	}, nil
}

// DecodeSealStatusRequest is a transport/grpc.DecodeRequestFunc that
// converts a gRPC sealstatus request to a user-domain sealstatus request. Primarily useful
// in a server.
func DecodeSealStatusRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	return endpoints.SealStatusRequest{}, nil
}

// DecodeSealStatusResponse is a transport/grpc.DecodeResponseFunc that
// converts a gRPC sealstatus reply to a user-domain sealstatus response. Primarily useful in
// a client.
func DecodeSealStatusResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*pb.SealStatusResponse)
	status := endpoints.SealStatusResponse{
		Sealed:      reply.SealStatus.Sealed,
		T:           int(reply.SealStatus.T),
		N:           int(reply.SealStatus.N),
		Progress:    int(reply.SealStatus.Progress),
		Version:     reply.SealStatus.Version,
		ClusterName: reply.SealStatus.ClusterName,
		ClusterID:   reply.SealStatus.ClusterId,
		Err:         service.String2Error(reply.Err),
	}

	return status, nil
}

// EncodeSealStatusResponse is a transport/grpc.EncodeResponseFunc that
// converts a user-domain sealstatus response to a gRPC sealstatus reply. Primarily useful in
// a server.
func EncodeSealStatusResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoints.SealStatusResponse)

	status := &pb.SealStatus{
		Sealed:      resp.Sealed,
		T:           uint32(resp.T),
		N:           uint32(resp.N),
		Progress:    uint32(resp.Progress),
		Version:     resp.Version,
		ClusterName: resp.ClusterName,
		ClusterId:   resp.ClusterID,
	}
	return &pb.SealStatusResponse{
		SealStatus: status,
		Err:        service.Error2String(resp.Err),
	}, nil
}

// EncodeSealStatusRequest is a transport/grpc.EncodeRequestFunc that
// converts a user-domain sealstatus request to a gRPC sealstatus request. Primarily useful
// in a client.
func EncodeSealStatusRequest(_ context.Context, request interface{}) (interface{}, error) {
	return &pb.SealStatusRequest{}, nil
}

// DecodeUnsealRequest is a transport/grpc.DecodeRequestFunc that
// converts a gRPC unseal request to a user-domain unseal request. Primarily useful
// in a server.
func DecodeUnsealRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*pb.UnsealRequest)
	return &endpoints.UnsealRequest{Key: req.Key, Reset: req.Reset_}, nil
}

// DecodeUnsealResponse is a transport/grpc.DecodeResponseFunc that
// converts a gRPC unseal reply to a user-domain unseal response. Primarily useful in
// a client.
func DecodeUnsealResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*pb.UnsealResponse)
	status := endpoints.UnsealResponse{
		Sealed:      reply.SealStatus.Sealed,
		T:           int(reply.SealStatus.T),
		N:           int(reply.SealStatus.N),
		Progress:    int(reply.SealStatus.Progress),
		Version:     reply.SealStatus.Version,
		ClusterName: reply.SealStatus.ClusterName,
		ClusterID:   reply.SealStatus.ClusterId,
		Err:         service.String2Error(reply.Err),
	}

	return status, nil
}

// EncodeUnsealResponse is a transport/grpc.EncodeResponseFunc that
// converts a user-domain unseal response to a gRPC sealstatus reply. Primarily useful in
// a server.
func EncodeUnsealResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoints.UnsealResponse)

	status := &pb.SealStatus{
		Sealed:      resp.Sealed,
		T:           uint32(resp.T),
		N:           uint32(resp.N),
		Progress:    uint32(resp.Progress),
		Version:     resp.Version,
		ClusterName: resp.ClusterName,
		ClusterId:   resp.ClusterID,
	}
	return &pb.UnsealResponse{
		SealStatus: status,
		Err:        service.Error2String(resp.Err),
	}, nil
}

// EncodeUnsealRequest is a transport/grpc.EncodeRequestFunc that
// converts a user-domain unseal request to a gRPC unseal request. Primarily useful
// in a client.
func EncodeUnsealRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(endpoints.UnsealRequest)
	return &pb.UnsealRequest{
		Key:    req.Key,
		Reset_: req.Reset,
	}, nil
}

// DecodeConfigureRequest is a transport/grpc.DecodeRequestFunc that
// converts a gRPC configure request to a user-domain configure request. Primarily useful
// in a server.
func DecodeConfigureRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*pb.ConfigureRequest)
	return &endpoints.ConfigureRequest{URL: req.Url, Token: req.Token}, nil
}

// DecodeConfigureResponse is a transport/grpc.DecodeResponseFunc that
// converts a gRPC configure reply to a user-domain configure response. Primarily useful in
// a client.
func DecodeConfigureResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*pb.ConfigureResponse)

	var mounts map[string]endpoints.MountOutput
	var auths map[string]endpoints.AuthMountOutput
	var policies []string

	if reply.ConfigStatus != nil {
		// mounts
		if (reply.ConfigStatus.Mounts != nil) && (len(reply.ConfigStatus.Mounts) > 0) {
			mounts = make(map[string]endpoints.MountOutput)
			for k, v := range reply.ConfigStatus.Mounts {

				mountCfgOut := endpoints.MountConfigOutput{
					DefaultLeaseTTL: int(v.Config.DefaultLeaseTtl),
					MaxLeaseTTL:     int(v.Config.MaxLeaseTtl),
				}

				mountOut := endpoints.MountOutput{
					Type:        v.Type,
					Description: v.Description,
					Config:      mountCfgOut,
				}

				mounts[k] = mountOut
			}
		}

		// auths
		if (reply.ConfigStatus.Auths != nil) && (len(reply.ConfigStatus.Auths) > 0) {
			auths = make(map[string]endpoints.AuthMountOutput)
			for k, v := range reply.ConfigStatus.Auths {

				authCfgOut := endpoints.AuthConfigOutput{
					DefaultLeaseTTL: int(v.Config.DefaultLeaseTtl),
					MaxLeaseTTL:     int(v.Config.MaxLeaseTtl),
				}

				authMountOut := endpoints.AuthMountOutput{
					Type:        v.Type,
					Description: v.Description,
					Config:      authCfgOut,
				}

				auths[k] = authMountOut
			}
		}
	}

	// policies
	if (reply.ConfigStatus.Policies != nil) && (len(reply.ConfigStatus.Policies) > 0) {
		policies = reply.ConfigStatus.Policies
	}

	status := endpoints.ConfigureResponse{
		ConfigID: reply.ConfigStatus.ConfigId,
		Mounts:   mounts,
		Auths:    auths,
		Policies: policies,
		Err:      service.String2Error(reply.Err),
	}

	return status, nil
}

// EncodeConfigureResponse is a transport/grpc.EncodeResponseFunc that
// converts a user-domain configure response to a gRPC configstatus reply. Primarily useful in
// a server.
func EncodeConfigureResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoints.ConfigureResponse)

	// mounts
	var mounts map[string]*pb.MountOutput
	if (resp.Mounts != nil) && (len(resp.Mounts) > 0) {
		mounts = make(map[string]*pb.MountOutput)
		for k, v := range resp.Mounts {
			mountCfgOut := &pb.MountConfigOutput{
				DefaultLeaseTtl: uint32(v.Config.DefaultLeaseTTL),
				MaxLeaseTtl:     uint32(v.Config.MaxLeaseTTL),
			}

			mountOut := &pb.MountOutput{
				Type:        v.Type,
				Description: v.Description,
				Config:      mountCfgOut,
			}

			mounts[k] = mountOut
		}
	}

	// auths
	var auths map[string]*pb.AuthMountOutput
	if (resp.Auths != nil) && (len(resp.Auths) > 0) {
		auths = make(map[string]*pb.AuthMountOutput)
		for k, v := range resp.Auths {
			authCfgOut := &pb.AuthConfigOutput{
				DefaultLeaseTtl: uint32(v.Config.DefaultLeaseTTL),
				MaxLeaseTtl:     uint32(v.Config.MaxLeaseTTL),
			}

			authMountOut := &pb.AuthMountOutput{
				Type:        v.Type,
				Description: v.Description,
				Config:      authCfgOut,
			}

			auths[k] = authMountOut
		}
	}

	// policies
	var policies []string
	if (resp.Policies != nil) && (len(resp.Policies) > 0) {
		policies = resp.Policies
	}

	status := &pb.ConfigStatus{
		ConfigId: resp.ConfigID,
		Mounts:   mounts,
		Auths:    auths,
		Policies: policies,
	}
	return &pb.ConfigureResponse{
		ConfigStatus: status,
		Err:          service.Error2String(resp.Err),
	}, nil
}

// EncodeConfigureRequest is a transport/grpc.EncodeRequestFunc that
// converts a user-domain configure request to a gRPC configure request. Primarily useful
// in a client.
func EncodeConfigureRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(endpoints.ConfigureRequest)
	return &pb.ConfigureRequest{
		Url:   req.URL,
		Token: req.Token,
	}, nil
}
