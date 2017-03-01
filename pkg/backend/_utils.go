package backend

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/cdwlabs/armor/pkg/config"
)

type Backend struct {
}

var (
	defaultBackend *Backend
	defaultSession *session.Session
)

//// Config returns the default configuration which is bound to defined
//// environment variables.
//func Config() Provider {
//	return defaultConfig
//}

func init() {
	cfg := config.Config()
	//defaultConfig = viperConfig("armor")
	cfg.Get("")
}
