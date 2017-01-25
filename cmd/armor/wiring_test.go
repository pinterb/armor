package main

import (
	"flag"
	"fmt"
	grpcclient "github.com/cdwlabs/armor/pkg/client/grpc"
	httpclient "github.com/cdwlabs/armor/pkg/client/http"
	"github.com/cdwlabs/armor/pkg/config"
	"net"
	"net/http/httptest"
	"os"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/cdwlabs/armor/pb"
	"github.com/cdwlabs/armor/pkg/proxy/endpoints"
	vaultgrpc "github.com/cdwlabs/armor/pkg/proxy/grpc"
	vaulthttp "github.com/cdwlabs/armor/pkg/proxy/http"
	"github.com/cdwlabs/armor/pkg/proxy/service"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/discard"
	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	integration = flag.Bool("integration", false, "run integration tests against external systems")
)

func TestMain(m *testing.M) {

	flag.Parse()
	var ret int

	// The implication of running integration tests is
	// that Vault is an external, unmodified dependency.
	// So we don't have to start our embedded, test container.
	if *integration {
		fmt.Println("Running as an integration test...")
		fmt.Println("Skipping...Until our jenkins pipeline is set up for true integration testing.")
		//ret = m.Run()
	} else {
		//		// start a vault container
		//		containers, err := NewTestContainers()
		//		if err != nil {
		//			panic(err)
		//		}

		// once vault is started, run tests...
		ret = m.Run()

		//		// tests completed, stop the vault container
		//		err = containers.CleanUp()
		//		if err != nil {
		//			panic(err)
		//		}
	}

	os.Exit(ret)
}

func setUp(t *testing.T) {
	_, err := os.Stat(config.PolicyConfigPathDefault)
	assert.Error(t, err, "expecting an error when stat'n policy config dir")
	assert.True(t, os.IsNotExist(err), "expecting error to be os.IsNotExist")
	err = os.MkdirAll(config.PolicyConfigPathDefault, 0755)
	assert.NoError(t, err, "not expecting an error when making policy config dir")
}

func tearDown(t *testing.T) {
	_, err := os.Stat(config.PolicyConfigPathDefault)
	assert.NoError(t, err, "not expecting an error when stat'n policy config dir")
	err = os.RemoveAll(config.PolicyConfigPathDefault)
	assert.NoError(t, err, "not expecting an error when removing all under policy config dir")
	_, err = os.Stat(config.PolicyConfigPathDefault)
	assert.Error(t, err, "expecting an error when stat'n policy config dir")
	assert.True(t, os.IsNotExist(err), "expecting error to be os.IsNotExist")
}

func TestHTTPWiring(t *testing.T) {
	// basic set up
	setUp(t)
	defer tearDown(t)

	// start a vault container
	containers, err := NewTestContainers()
	if err != nil {
		panic(err)
	}
	defer containers.CleanUp()

	// Assemble the service endpoints
	ctx := context.Background()
	svc := service.New(log.NewNopLogger(), discard.NewCounter(), discard.NewHistogram())
	eps := endpoints.New(svc, log.NewNopLogger(), discard.NewHistogram(), opentracing.GlobalTracer())
	mux := vaulthttp.NewHandler(ctx, eps, opentracing.GlobalTracer(), log.NewNopLogger())

	// Start the HTTP version of our Vault proxy service
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := httpclient.New(srv.URL, opentracing.GlobalTracer(), log.NewNopLogger())
	assert.NoError(t, err, "not expecting an error when creating http client")

	// Get init status for uninitialized vault
	status, err := client.InitStatus(ctx)
	assert.NoError(t, err, "not expecting an error when calling http initstatus")
	assert.False(t, status, "expecting InitStatus to return false")

	// Perform a valid init of uninitialized vault
	initOptions := service.InitOptions{
		SecretShares:    5,
		SecretThreshold: 3,
	}
	initValues, err := client.Init(ctx, initOptions)
	assert.NoError(t, err, "not expecting an error when calling http init")

	keylen := len(initValues.Keys)
	key64len := len(initValues.KeysB64)
	recovkeylen := len(initValues.RecoveryKeys)
	recovkey64len := len(initValues.RecoveryKeysB64)
	roottokenlen := utf8.RuneCountInString(initValues.RootToken)

	assert.True(t, 5 == keylen, "expecting 5 keys to be returned")
	assert.True(t, 5 == key64len, "expecting 5 base 64 keys to be returned")
	assert.True(t, 36 == roottokenlen, "expecting a root token to be returned")
	assert.True(t, 0 == recovkeylen, "not expecting any recovery keys to be returned")
	assert.True(t, 0 == recovkey64len, "not expecting any base 64 recovery keys to be returned")

	// Save these keys for unsealing
	keyone := initValues.Keys[0]
	keytwo := initValues.Keys[1]
	keythree := initValues.Keys[2]

	// Get init status for initialized vault
	status, err = client.InitStatus(ctx)
	assert.NoError(t, err, "not expecting an error when calling http initstatus")
	assert.True(t, status, "expecting InitStatus to return true")

	// Get seal status for initialized, but unsealed vault
	state, err := client.SealStatus(ctx)
	assert.NoError(t, err, "not expecting an error when calling http sealstatus")
	assert.True(t, state.Sealed, "expecting sealed state of true")
	assert.True(t, 3 == state.T, "expecting threshold value of 3")
	assert.True(t, 5 == state.N, "expecting shares value of 5")
	assert.True(t, 0 == state.Progress, "expecting progress value of 0")

	// Create unseal w/first key
	unsealonereq := service.UnsealOptions{
		Key:   keyone,
		Reset: false,
	}

	// Unseal with first key
	sealstatone, err := client.Unseal(ctx, unsealonereq)
	assert.NoError(t, err, "not expecting an error when calling http first unseal")
	assert.True(t, sealstatone.Sealed, "expecting vault to still be sealed")
	assert.True(t, 1 == sealstatone.Progress, "expecting vault unseal progress to be set to one")
	assert.True(t, 0 == utf8.RuneCountInString(sealstatone.ClusterID), "expecting vault cluster id to be unset")
	assert.True(t, 0 == utf8.RuneCountInString(sealstatone.ClusterName), "expecting vault cluster name to be unset")

	// Create unseal w/second key
	unsealtworeq := service.UnsealOptions{
		Key:   keytwo,
		Reset: false,
	}

	// Unseal with second key
	sealstattwo, err := client.Unseal(ctx, unsealtworeq)
	assert.NoError(t, err, "not expecting an error when calling http 2nd unseal")
	assert.True(t, sealstattwo.Sealed, "expecting vault to still be sealed")
	assert.True(t, 2 == sealstattwo.Progress, "expecting vault unseal progress to be set to two")
	assert.True(t, 0 == utf8.RuneCountInString(sealstatone.ClusterID), "expecting vault cluster id to be unset")
	assert.True(t, 0 == utf8.RuneCountInString(sealstatone.ClusterName), "expecting vault cluster name to be unset")

	// Create unseal w/third key
	unsealthreereq := service.UnsealOptions{
		Key:   keythree,
		Reset: false,
	}

	// Unseal with third key
	sealstatthree, err := client.Unseal(ctx, unsealthreereq)
	assert.NoError(t, err, "not expecting an error when calling http 3rd unseal")
	assert.True(t, !sealstatthree.Sealed, "expecting vault to be unsealed")
	assert.True(t, 0 == sealstatthree.Progress, "expecting vault unseal progress reset to zero")
	assert.True(t, 0 < utf8.RuneCountInString(sealstatthree.ClusterID), "expecting vault cluster id to now be set")
	assert.True(t, 0 < utf8.RuneCountInString(sealstatthree.ClusterName), "expecting vault cluster name to now be set")

	cwd, _ := os.Getwd()

	// Non-existent configuration directory
	mounturl := cwd + "/test-fixtures/configure/blah"
	cfgreq := service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	_, err = client.Configure(ctx, cfgreq)
	assert.Error(t, err, "expecting an error with configure request containing non-existent url")

	// Malformed configuration directory
	mounturl = cwd + "/test-fixtures/configure/malformed"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	_, err = client.Configure(ctx, cfgreq)
	assert.Error(t, err, "expecting an error with configure request containing malformed directory")

	// Missing configuration token
	mounturl = cwd + "/test-fixtures/configure/malformed"
	cfgreq = service.ConfigOptions{
		URL: mounturl,
	}
	_, err = client.Configure(ctx, cfgreq)
	assert.Error(t, err, "expecting an error with configure request missing a token")

	// Configure initial mounts
	mounturl = cwd + "/test-fixtures/configure/initialmounts"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err := client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure initial mounts")
	assert.True(t, len(configstate.Mounts) == 6, "expecting six initial mounts")
	awscfg, ok := configstate.Mounts["aws/"]
	assert.True(t, ok, "expecting aws mount to exist")
	_, ok = configstate.Mounts["postgresql/"]
	assert.True(t, ok, "expecting postgresql mount to exist")
	assert.True(t, awscfg.Config.DefaultLeaseTTL == 8, "expecting initial aws mount default lease ttl of 8")
	assert.True(t, awscfg.Config.MaxLeaseTTL == 24, "expecting initial aws mount max lease ttl of 24")
	_, ok = configstate.Mounts["cdw/mans/xyzinc/app1/prod/db/"]
	assert.True(t, ok, "expecting cdw/mans/xyzinc/app1/prod/db mount to exist")

	// Tune postgresql mount
	mounturl = cwd + "/test-fixtures/configure/postgresqlmounttune"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure tuning of postgresql mount")
	assert.True(t, len(configstate.Mounts) == 6, "expecting six initial mounts")
	postgrescfg, ok := configstate.Mounts["postgresql/"]
	assert.True(t, ok, "expecting postgresql mount to exist")
	assert.True(t, postgrescfg.Config.DefaultLeaseTTL == 7, "expecting postgresql mount default lease ttl of 7")
	assert.True(t, postgrescfg.Config.MaxLeaseTTL == 21, "expecting postgresql mount max lease ttl of 21")

	// Enable initial auth backends
	mounturl = cwd + "/test-fixtures/configure/initialauths"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure initial auth backends")
	assert.True(t, len(configstate.Auths) == 6, "expecting six initial auth backends")
	githubcfg, ok := configstate.Auths["github/"]
	assert.True(t, ok, "expecting github auth backend to exist")
	assert.Equal(t, "Authentication to the cdwlabs GitHub organization", githubcfg.Description, "expecting github auth description")
	_, ok = configstate.Auths["userpass/"]
	assert.True(t, ok, "expecting userpass auth backend to exist")
	appadmincfg, ok := configstate.Auths["cdw/mans/xyzinc/app1/prod/admin/"]
	assert.True(t, ok, "expecting cdw/mans/xyzinc/app1/prod/admin auth backend to exist")
	assert.Equal(t, "approle", appadmincfg.Type, "expecting approle")

	// Disable userpass auth backend
	mounturl = cwd + "/test-fixtures/configure/disableuserpass"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure disable userpass backends")
	assert.True(t, len(configstate.Auths) == 5, "expecting five auth backends")
	_, ok = configstate.Auths["userpass/"]
	assert.False(t, ok, "expecting userpass auth backend to no longer exist")

	// Enable initial policies
	mounturl = cwd + "/test-fixtures/configure/initialpolicies"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure initial policies")
	assert.True(t, len(configstate.Policies) == 4, "expecting four initial policies")
	assert.True(t, containsString(configstate.Policies, "postgresql/readonly"), "expecting a specific policy")
	assert.True(t, containsString(configstate.Policies, "cdw/mans/xyzinc/app1/prod/db/readonly"), "expecting a specific policy")

	// Delete/disable postgresql/readonly policy
	mounturl = cwd + "/test-fixtures/configure/disablepostgresqlpolicy"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure disable policy")
	fmt.Printf("polcies: %q\n", configstate.Policies)
	assert.True(t, len(configstate.Policies) == 3, "expecting three policies")
	assert.False(t, containsString(configstate.Policies, "postgresql/readonly"), "expecting a specific policy")
	assert.True(t, containsString(configstate.Policies, "cdw/mans/xyzinc/app1/prod/db/readonly"), "expecting a specific policy")
}

func TestGRPCWiring(t *testing.T) {
	// basic set up
	setUp(t)
	defer tearDown(t)

	// start a vault container
	containers, err := NewTestContainers()
	if err != nil {
		panic(err)
	}
	defer containers.CleanUp()

	// Assemble the service endpoints
	grpcAddr := ":9082"
	svc := service.New(log.NewNopLogger(), discard.NewCounter(), discard.NewHistogram())
	eps := endpoints.New(svc, log.NewNopLogger(), discard.NewHistogram(), opentracing.GlobalTracer())

	// Start the gRPC version of our Vault proxy service
	ln, err := net.Listen("tcp", grpcAddr)
	assert.NoError(t, err, "not expecting an error when starting grpc listener")

	srv := vaultgrpc.NewHandler(context.Background(), eps, opentracing.GlobalTracer(), log.NewNopLogger())
	s := grpc.NewServer()
	pb.RegisterVaultServer(s, srv)
	go s.Serve(ln)
	defer s.GracefulStop()

	// Create gRPC client connection
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure(), grpc.WithTimeout(time.Second))
	assert.NoError(t, err, "not expecting an error when connecting to grpc server")
	defer conn.Close()

	// Create client to gRPC version of our Vault proxy service
	ctx := context.Background()
	client := grpcclient.New(conn, opentracing.GlobalTracer(), log.NewNopLogger())

	// Get init status for uninitialized vault
	status, err := client.InitStatus(ctx)
	assert.NoError(t, err, "not expecting an error when calling grpc initstatus")
	assert.False(t, status, "expecting InitStatus to return false")

	// Perform a valid init of uninitialized vault
	initOptions := service.InitOptions{
		SecretShares:    5,
		SecretThreshold: 3,
	}

	initValues, err := client.Init(ctx, initOptions)
	assert.NoError(t, err, "not expecting an error when calling grpc init")

	keylen := len(initValues.Keys)
	key64len := len(initValues.KeysB64)
	recovkeylen := len(initValues.RecoveryKeys)
	recovkey64len := len(initValues.RecoveryKeysB64)
	roottokenlen := utf8.RuneCountInString(initValues.RootToken)

	assert.True(t, 5 == keylen, "expecting 5 keys to be returned")
	assert.True(t, 5 == key64len, "expecting 5 base 64 keys to be returned")
	assert.True(t, 36 == roottokenlen, "expecting a root token to be returned")
	assert.True(t, 0 == recovkeylen, "not expecting any recovery keys to be returned")
	assert.True(t, 0 == recovkey64len, "not expecting any base 64 recovery keys to be returned")

	// Save these keys for unsealing
	keyone := initValues.Keys[0]
	keytwo := initValues.Keys[1]
	keythree := initValues.Keys[2]

	// Get init status for initialized vault
	status, err = client.InitStatus(ctx)
	assert.NoError(t, err, "not expecting an error when calling grpc initstatus")
	assert.True(t, status, "expecting InitStatus to return true")

	// Get seal status for initialized, but unsealed vault
	state, err := client.SealStatus(ctx)
	assert.NoError(t, err, "not expecting an error when calling grpc sealstatus")
	assert.True(t, state.Sealed, "expecting sealed state of true")
	assert.True(t, 3 == state.T, "expecting threshold value of 3")
	assert.True(t, 5 == state.N, "expecting shares value of 5")
	assert.True(t, 0 == state.Progress, "expecting progress value of 0")

	// Create unseal w/first key
	unsealonereq := service.UnsealOptions{
		Key:   keyone,
		Reset: false,
	}

	// Unseal with first key
	sealstatone, err := client.Unseal(ctx, unsealonereq)
	assert.NoError(t, err, "not expecting an error when calling grpc first unseal")
	assert.True(t, sealstatone.Sealed, "expecting vault to still be sealed")
	assert.True(t, 1 == sealstatone.Progress, "expecting vault unseal progress to be set to one")
	assert.True(t, 0 == utf8.RuneCountInString(sealstatone.ClusterID), "expecting vault cluster id to be unset")
	assert.True(t, 0 == utf8.RuneCountInString(sealstatone.ClusterName), "expecting vault cluster name to be unset")

	// Create unseal w/second key
	unsealtworeq := service.UnsealOptions{
		Key:   keytwo,
		Reset: false,
	}

	// Unseal with second key
	sealstattwo, err := client.Unseal(ctx, unsealtworeq)
	assert.NoError(t, err, "not expecting an error when calling grpc 2nd unseal")
	assert.True(t, sealstattwo.Sealed, "expecting vault to still be sealed")
	assert.True(t, 2 == sealstattwo.Progress, "expecting vault unseal progress to be set to two")
	assert.True(t, 0 == utf8.RuneCountInString(sealstatone.ClusterID), "expecting vault cluster id to be unset")
	assert.True(t, 0 == utf8.RuneCountInString(sealstatone.ClusterName), "expecting vault cluster name to be unset")

	// Create unseal w/third key
	unsealthreereq := service.UnsealOptions{
		Key:   keythree,
		Reset: false,
	}

	// Unseal with third key
	sealstatthree, err := client.Unseal(ctx, unsealthreereq)
	assert.NoError(t, err, "not expecting an error when calling grpc 3rd unseal")
	assert.True(t, !sealstatthree.Sealed, "expecting vault to be unsealed")
	assert.True(t, 0 == sealstatthree.Progress, "expecting vault unseal progress reset to zero")
	assert.True(t, 0 < utf8.RuneCountInString(sealstatthree.ClusterID), "expecting vault cluster id to now be set")
	assert.True(t, 0 < utf8.RuneCountInString(sealstatthree.ClusterName), "expecting vault cluster name to now be set")

	cwd, _ := os.Getwd()

	// Non-existent configuration directory
	mounturl := cwd + "/test-fixtures/configure/blah"
	cfgreq := service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	_, err = client.Configure(ctx, cfgreq)
	assert.Error(t, err, "expecting an error with configure request containing non-existent url")

	// Malformed configuration directory
	mounturl = cwd + "/test-fixtures/configure/malformed"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	_, err = client.Configure(ctx, cfgreq)
	assert.Error(t, err, "expecting an error with configure request containing malformed directory")

	// Missing configuration token
	mounturl = cwd + "/test-fixtures/configure/malformed"
	cfgreq = service.ConfigOptions{
		URL: mounturl,
	}
	_, err = client.Configure(ctx, cfgreq)
	assert.Error(t, err, "expecting an error with configure request missing a token")

	// Configure initial mounts
	mounturl = cwd + "/test-fixtures/configure/initialmounts"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err := client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure initial mounts")
	assert.True(t, len(configstate.Mounts) == 6, "expecting six initial mounts")
	awscfg, ok := configstate.Mounts["aws/"]
	assert.True(t, ok, "expecting aws mount to exist")
	_, ok = configstate.Mounts["postgresql/"]
	assert.True(t, ok, "expecting postgresql mount to exist")
	assert.True(t, awscfg.Config.DefaultLeaseTTL == 8, "expecting initial aws mount default lease ttl of 8")
	assert.True(t, awscfg.Config.MaxLeaseTTL == 24, "expecting initial aws mount max lease ttl of 24")

	// Tune postgresql mount
	mounturl = cwd + "/test-fixtures/configure/postgresqlmounttune"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure tuning of postgresql mount")
	assert.True(t, len(configstate.Mounts) == 6, "expecting six initial mounts")
	postgrescfg, ok := configstate.Mounts["postgresql/"]
	assert.True(t, ok, "expecting postgresql mount to exist")
	assert.True(t, postgrescfg.Config.DefaultLeaseTTL == 7, "expecting postgresql mount default lease ttl of 7")
	assert.True(t, postgrescfg.Config.MaxLeaseTTL == 21, "expecting postgresql mount max lease ttl of 21")

	// Enable initial auth backends
	mounturl = cwd + "/test-fixtures/configure/initialauths"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure initial auth backends")
	assert.True(t, len(configstate.Auths) == 6, "expecting six initial auth backends")
	githubcfg, ok := configstate.Auths["github/"]
	assert.True(t, ok, "expecting github auth backend to exist")
	assert.Equal(t, "Authentication to the cdwlabs GitHub organization", githubcfg.Description, "expecting github auth description")
	_, ok = configstate.Auths["userpass/"]
	assert.True(t, ok, "expecting userpass auth backend to exist")
	appadmincfg, ok := configstate.Auths["cdw/mans/xyzinc/app1/prod/admin/"]
	assert.True(t, ok, "expecting cdw/mans/xyzinc/app1/prod/admin auth backend to exist")
	assert.Equal(t, "approle", appadmincfg.Type, "expecting approle")

	// Disable userpass auth backend
	mounturl = cwd + "/test-fixtures/configure/disableuserpass"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure disable userpass backends")
	assert.True(t, len(configstate.Auths) == 5, "expecting five auth backends")
	_, ok = configstate.Auths["userpass/"]
	assert.False(t, ok, "expecting userpass auth backend to no longer exist")

	// Enable initial policies
	mounturl = cwd + "/test-fixtures/configure/initialpolicies"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure initial policies")
	assert.True(t, len(configstate.Policies) == 4, "expecting four initial policies")
	assert.True(t, containsString(configstate.Policies, "postgresql/readonly"), "expecting a specific policy")
	assert.True(t, containsString(configstate.Policies, "cdw/mans/xyzinc/app1/prod/db/readonly"), "expecting a specific policy")

	// Delete/disable postgresql/readonly policy
	mounturl = cwd + "/test-fixtures/configure/disablepostgresqlpolicy"
	cfgreq = service.ConfigOptions{
		URL:   mounturl,
		Token: initValues.RootToken,
	}
	configstate, err = client.Configure(ctx, cfgreq)
	assert.NoError(t, err, "not expecting an error from /configure disable policy")
	fmt.Printf("polcies: %q\n", configstate.Policies)
	assert.True(t, len(configstate.Policies) == 3, "expecting three policies")
	assert.False(t, containsString(configstate.Policies, "postgresql/readonly"), "expecting a specific policy")
	assert.True(t, containsString(configstate.Policies, "cdw/mans/xyzinc/app1/prod/db/readonly"), "expecting a specific policy")
}
