package main

import (
	grpcclient "github.com/cdwlabs/armor/pkg/client/grpc"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cdwlabs/armor/pb"
	"github.com/cdwlabs/armor/pkg/proxy/endpoints"
	vaultgrpc "github.com/cdwlabs/armor/pkg/proxy/grpc"
	vaulthttp "github.com/cdwlabs/armor/pkg/proxy/http"
	"github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/discard"
	"github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	// start a vault container
	containers, err := NewTestContainers()
	if err != nil {
		panic(err)
	}

	ret := m.Run()

	// tests completed, stop the vault container
	err = containers.CleanUp()
	if err != nil {
		panic(err)
	}
	os.Exit(ret)
}

func TestHTTPWiring(t *testing.T) {
	// Assemble the service endpoints
	svc := service.New(log.NewNopLogger(), discard.NewCounter(), discard.NewHistogram())
	eps := endpoints.New(svc, log.NewNopLogger(), discard.NewHistogram(), opentracing.GlobalTracer())
	mux := vaulthttp.NewHandler(context.Background(), eps, opentracing.GlobalTracer(), log.NewNopLogger())

	// Start the HTTP version of our Vault proxy service
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, testcase := range []struct {
		method, url, body, want string
	}{
		{"GET", srv.URL + "/init/status", "", `{"v":false}`},
	} {
		req, _ := http.NewRequest(testcase.method, testcase.url, strings.NewReader(testcase.body))
		resp, _ := http.DefaultClient.Do(req)
		body, _ := ioutil.ReadAll(resp.Body)
		if want, have := testcase.want, strings.TrimSpace(string(body)); want != have {
			t.Errorf("%s %s %s: want %q, have %q", testcase.method, testcase.url, testcase.body, want, have)
		}
	}
}

func TestGRPCWiring(t *testing.T) {
	// Assemble the service endpoints
	grpcAddr := ":9082"
	svc := service.New(log.NewNopLogger(), discard.NewCounter(), discard.NewHistogram())
	eps := endpoints.New(svc, log.NewNopLogger(), discard.NewHistogram(), opentracing.GlobalTracer())

	// Start the gRPC version of our Vault proxy service
	ln, err := net.Listen("tcp", grpcAddr)
	ok(t, err)

	srv := vaultgrpc.NewHandler(context.Background(), eps, opentracing.GlobalTracer(), log.NewNopLogger())
	s := grpc.NewServer()
	pb.RegisterVaultServer(s, srv)
	go s.Serve(ln)
	defer s.GracefulStop()

	// Create gRPC client connection
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure(), grpc.WithTimeout(time.Second))
	ok(t, err)
	defer conn.Close()

	// Create client to gRPC version of our Vault proxy service
	client := grpcclient.New(conn, opentracing.GlobalTracer(), log.NewNopLogger())
	status, err := client.InitStatus(context.Background())
	ok(t, err)
	assert(t, status == false, "expecting InitStatus to return false")
}
