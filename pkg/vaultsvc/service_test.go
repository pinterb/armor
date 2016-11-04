package vaultsvc

import (
	"bytes"
	"encoding/json"
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

type TestContainers struct {
	client       *docker.Client
	vaultID      string
	vaultDataDir string
}

var (
	// Set to false if you want to use test Vault container for debugging
	// purposes (e.g. failing test cases?)
	testRemoveContainer = true

	// Set to true if you want to see logs from Vault container.  NOTE: these
	// logs are printed after tests have completed (either successfully or
	// unsuccessfully).
	testDumpContainerLogs = true

	ctx            = context.Background()
	dockerEndpoint string

	VaultImageName = "pinterb/vault"
	VaultImageTag  = "0.6.2"

	// VaultDisableMlock if true, this will disable the server from executing the
	// mlock syscall to prevent memory from being swapped to disk. This is not
	// recommended for production!
	VaultDisableMlock = false
	VaultDisableCache = false

	VaultMaxLeaseTTL    = 32 * 24 * time.Hour
	VaultMaxLeaseTTLRaw = "720h"

	VaultDefaultLeaseTTL    = 32 * 24 * time.Hour
	VaultDefaultLeaseTTLRaw = "168h"

	// A function with no arguments that outputs a valid JSON string to be used
	// as the value of the environment variable VAULT_LOCAL_CONFIG.
	VaultLocalConfigGen = DefaultVaultLocalConfig
)

// assert fails the test if the condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
}

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}

func DefaultVaultLocalConfig() (string, error) {
	type Backend struct {
		Type              string            `json:"type,omitempty"`
		RedirectAddr      string            `json:"redirect_addr,omitempty"`
		ClusterAddr       string            `json:"cluster_addr,omitempty"`
		DisableClustering bool              `json:"disable_clustering,omitempty"`
		Config            map[string]string `json:"config,omitempty"`
	}

	type FileBackend struct {
		Config map[string]string `json:"file,omitempty"`
	}

	type Listener struct {
		Type   string            `json:"type,omitempty"`
		Config map[string]string `json:"config,omitempty"`
	}

	type TCPListener struct {
		Config map[string]string `json:"tcp,omitempty"`
	}

	type Telemetry struct {
		StatsiteAddr                       string `json:"statsite_address,omitempty"`
		StatsdAddr                         string `json:"statsd_address,omitempty"`
		DisableHostname                    bool   `json:"disable_hostname,omitempty"`
		CirconusAPIToken                   string `json:"circonus_api_token,omitempty"`
		CirconusAPIApp                     string `json:"circonus_api_app,omitempty"`
		CirconusAPIURL                     string `json:"circonus_api_url,omitempty"`
		CirconusSubmissionInterval         string `json:"circonus_submission_interval,omitempty"`
		CirconusCheckSubmissionURL         string `json:"circonus_submission_url,omitempty"`
		CirconusCheckID                    string `json:"circonus_check_id,omitempty"`
		CirconusCheckForceMetricActivation string `json:"circonus_check_force_metric_activation,omitempty"`
		CirconusCheckInstanceID            string `json:"circonus_check_instance_id,omitempty"`
		CirconusCheckSearchTag             string `json:"circonus_check_search_tag,omitempty"`
		CirconusBrokerID                   string `json:"circonus_broker_id,omitempty"`
		CirconusBrokerSelectTag            string `json:"circonus_broker_select_tag,omitempty"`
	}

	type Config struct {
		Listeners          *TCPListener  `json:"listener,omitempty"`
		Backend            *FileBackend  `json:"backend,omitempty"`
		HABackend          *Backend      `json:"ha_backend,omitempty"`
		CacheSize          int           `json:"cache_size,omitempty"`
		DisableCache       bool          `json:"disable_cache,omitempty"`
		DisableMlock       bool          `json:"disable_mlock,omitempty"`
		Telemetry          *Telemetry    `json:"telemetry,omitempty"`
		MaxLeaseTTL        time.Duration `json:"-,omitempty"`
		MaxLeaseTTLRaw     string        `json:"max_lease_ttl,omitempty"`
		DefaultLeaseTTL    time.Duration `json:"-,omitempty"`
		DefaultLeaseTTLRaw string        `json:"default_lease_ttl,omitempty"`
		ClusterName        string        `json:"cluster_name,omitempty"`
	}

	vals := &Config{
		DisableCache: VaultDisableCache,
		DisableMlock: VaultDisableMlock,

		Backend: &FileBackend{
			Config: map[string]string{
				"path": "/vault/file",
			},
		},

		Listeners: &TCPListener{
			Config: map[string]string{
				"address":       "0.0.0.0:8200",
				"tls_disable":   "false",
				"tls_cert_file": "/vault/tls/cert.pem",
				"tls_key_file":  "/vault/tls/cert-key.pem",
			},
		},

		Telemetry: &Telemetry{},

		MaxLeaseTTL:        VaultMaxLeaseTTL,
		MaxLeaseTTLRaw:     VaultMaxLeaseTTLRaw,
		DefaultLeaseTTL:    VaultDefaultLeaseTTL,
		DefaultLeaseTTLRaw: VaultDefaultLeaseTTLRaw,
	}

	ret, err := json.Marshal(vals)
	if err != nil {
		return "", err
	}

	return string(ret), nil
}

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

func Test_InitStatus(t *testing.T) {
	service := NewVaultService()
	status, err := service.InitStatus(ctx)
	if err != nil {
		t.Fatalf("resp is error: %v", err)
	}
	assert(t, status == false, "expecting init status to be false")
}

// NewTestContainers sets up our test containers.
func NewTestContainers() (*TestContainers, error) {
	client, err := docker.NewClient(getDockerEndpoint())
	if err != nil {
		fmt.Errorf("Failed to create docker client: %v", err)
		return nil, err
	}

	err = client.Ping()
	if err != nil {
		fmt.Errorf("Failed to ping docker w/client: %v", err)
		return nil, err
	}

	// Create a temporary directory for vault data
	dataDir, err := ioutil.TempDir("", "vaultdata")
	if err != nil {
		fmt.Errorf("Failed to temp directory for vault's data directory: %v", err)
		return nil, err
	}

	cwd, _ := os.Getwd()
	os.Setenv(vaultapi.EnvVaultClientCert, cwd+"/test-fixtures/keys/client.pem")
	os.Setenv(vaultapi.EnvVaultClientKey, cwd+"/test-fixtures/keys/client-key.pem")
	os.Setenv(vaultapi.EnvVaultCACert, cwd+"/test-fixtures/keys/ca-cert.pem")
	//os.Setenv(vaultapi.EnvVaultCAPath, cwd+"/test-fixtures/keys")
	//os.Setenv(vaultapi.EnvVaultInsecure, "true")
	os.Setenv(vaultapi.EnvVaultMaxRetries, "5")

	// Define our Vault container host config...
	mounts := []docker.Mount{
		{Name: "data", Source: dataDir, Destination: "/vault/file", Mode: "rxw"},
		{Name: "tls", Source: cwd + "/test-fixtures/keys", Destination: "/vault/tls", Mode: "rxw"},
	}

	vols := make(map[string]struct{})
	for _, mount := range mounts {
		vols[mount.Source] = struct{}{}
	}

	binds := make([]string, len(mounts))
	for i, mount := range mounts {
		binds[i] = fmt.Sprintf("%s:%s", mount.Source, mount.Destination)
	}
	capadd := make([]string, 1)
	capadd[0] = "IPC_LOCK"

	portBindings := map[docker.Port][]docker.PortBinding{
		"8200/tcp": {{HostIP: "0.0.0.0", HostPort: "8200"}}}

	hostConfig := docker.HostConfig{
		Binds:           binds,
		CapAdd:          capadd,
		PortBindings:    portBindings,
		PublishAllPorts: false,
		Privileged:      false,
	}

	// Define our Vault create container options...
	containerName := fmt.Sprintf("vault-test-%s", uuid.New())
	exposedVaultPort := map[docker.Port]struct{}{
		"8200/tcp": {}}

	genVaultConfig, err := VaultLocalConfigGen()

	createOpts := docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image:        fmt.Sprintf("%s:%s", VaultImageName, VaultImageTag),
			Labels:       map[string]string{"com.cdw.cdwlabs": "true"},
			Hostname:     "feh.cdw.com",
			Volumes:      vols,
			Mounts:       mounts,
			ExposedPorts: exposedVaultPort,
			Env:          []string{fmt.Sprintf("VAULT_LOCAL_CONFIG=%s", genVaultConfig), "VAULT_CACERT=/vault/tls/intermediate_ca.pem"},
			Cmd:          []string{"server", "-log-level=debug"},
		},
		HostConfig: &hostConfig,
	}

	cont, err := client.CreateContainer(createOpts)
	if err != nil {
		fmt.Errorf("Failed to create Vault test container: %v", err)
		return nil, err
	}

	err = client.StartContainer(cont.ID, nil)
	if err != nil {
		fmt.Errorf("Failed to start Vault test container: %v", err)
		return nil, err
	}

	return &TestContainers{
		client:       client,
		vaultID:      cont.ID,
		vaultDataDir: dataDir,
	}, nil
}

// CleanUp removes our test containers.
func (containers *TestContainers) CleanUp() error {
	//	defer containers.writer.Flush()
	defer os.RemoveAll(containers.vaultDataDir)
	err := containers.client.StopContainer(containers.vaultID, 10)
	if err != nil {
		fmt.Errorf("Failed to stop container: %v", err)
		return err
	}

	// Reading logs from container and sending them to buf.
	if testDumpContainerLogs {
		fmt.Println("")
		fmt.Println("##############################################")
		fmt.Println("           Vault Container Logs")
		fmt.Println("")
		var buf bytes.Buffer
		err = containers.client.AttachToContainer(docker.AttachToContainerOptions{
			Container:    containers.vaultID,
			OutputStream: &buf,
			Logs:         true,
			Stdout:       true,
			Stderr:       true,
		})
		if err != nil {
			fmt.Errorf("Failed to attach to stopped container: %v", err)
			return err
		}
		fmt.Println(buf.String())
		fmt.Println("")
		fmt.Println("##############################################")
		fmt.Println("")
	}

	if testRemoveContainer {
		opts := docker.RemoveContainerOptions{ID: containers.vaultID}
		err = containers.client.RemoveContainer(opts)
		if err != nil {
			fmt.Errorf("Failed to remove container: %v", err)
			return err
		}
	}
	return nil
}

func getDockerEndpoint() string {
	var endpoint string
	if len(os.Getenv("DOCKER_HOST")) > 0 {
		endpoint = os.Getenv("DOCKER_HOST")
	} else {
		endpoint = "unix:///var/run/docker.sock"
	}
	fmt.Printf("Connecting to docker on %s", endpoint)

	return endpoint
}
