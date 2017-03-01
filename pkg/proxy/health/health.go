package health

// This file is intended to provide health/readiness checking for our
// application.  These checks are used by the Kubernetes Liveness and Readiness
// probes (http://kubernetes.io/docs/user-guide/walkthrough/k8s201/).  See also
// the following article from RedHat:
// https://docs.openshift.com/enterprise/3.0/dev_guide/application_health.html
import (
	"fmt"
	"github.com/go-kit/kit/log"
	"net/http"
	"os"
	"sync"
)

var (
	healthzStatus   = http.StatusOK
	readinessStatus = http.StatusOK
	mu              sync.RWMutex
	logger          log.Logger
)

func init() {
	// Logging domain.
	logger = log.NewLogfmtLogger(os.Stdout)
	logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
	logger = log.NewContext(logger).With("caller", log.DefaultCaller)
}

// HealthzStatus returns the current health status of the application.
func HealthzStatus() int {
	mu.RLock()
	defer mu.RUnlock()
	return healthzStatus
}

// ReadinessStatus returns the current readiness status of the application.
func ReadinessStatus() int {
	mu.RLock()
	defer mu.RUnlock()
	return readinessStatus
}

// SetHealthzStatus sets the current health status of the application.
func SetHealthzStatus(status int) {
	mu.Lock()
	logger.Log("msg", fmt.Sprintf("setting health status to %v", status))
	healthzStatus = status
	mu.Unlock()
}

// SetReadinessStatus sets the current readiness status of the application.
func SetReadinessStatus(status int) {
	mu.Lock()
	if status != readinessStatus {
		logger.Log("msg", fmt.Sprintf("setting readiness status to %v", status))
		readinessStatus = status
	}
	mu.Unlock()
}

// HealthzHandler responds to health check requests.
func HealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(HealthzStatus())
}

// ReadinessHandler responds to readiness check requests.
func ReadinessHandler(w http.ResponseWriter, r *http.Request) {

	var err error
	if err = backendDataHealth(logger); err != nil {
		logger.Log("msg", "backend data readiness check failed")
		SetReadinessStatus(http.StatusServiceUnavailable)

	} else if err = policyDirHealth(logger); err != nil {
		logger.Log("msg", "file system readiness check failed")
		SetReadinessStatus(http.StatusServiceUnavailable)

	} else if err = vaulthealth(); err != nil { // Really just testing the connection to our Vault instance
		logger.Log("msg", "vault readiness check failed")
		SetReadinessStatus(http.StatusServiceUnavailable)

	} else {
		SetReadinessStatus(http.StatusOK)
	}

	w.WriteHeader(ReadinessStatus())
}

// HealthzStatusHandler is an HTTP handler for toggling the health status of
// the application. In other words, if the current status is Okay then hitting
// this endpoint will change the status to Unavailable...and vice versa.
func HealthzStatusHandler(w http.ResponseWriter, r *http.Request) {
	switch HealthzStatus() {
	case http.StatusOK:
		SetHealthzStatus(http.StatusServiceUnavailable)
	case http.StatusServiceUnavailable:
		SetHealthzStatus(http.StatusOK)
	}
	w.WriteHeader(http.StatusOK)
}

//// ReadinessStatusHandler is an HTTP handler for toggling the readiness status
//// of the application. In other words, if the current status is Okay then
//// hitting this endpoint will change the status to Unavailable...and vice versa.
//func ReadinessStatusHandler(w http.ResponseWriter, r *http.Request) {
//	switch ReadinessStatus() {
//	case http.StatusOK:
//		SetReadinessStatus(http.StatusServiceUnavailable)
//	case http.StatusServiceUnavailable:
//		SetReadinessStatus(http.StatusOK)
//	}
//	w.WriteHeader(http.StatusOK)
//}
