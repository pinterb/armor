package health

import (
	"errors"
	"fmt"
	"github.com/cdwlabs/armor/pkg/config"
	"github.com/go-kit/kit/log"
	"os"
)

func policyDirHealth(logger log.Logger) error {
	cfg := config.Config()

	if cfg.IsSet("policy_config_dir") && cfg.GetString("policy_config_dir") != "" {
		policydir := cfg.GetString("policy_config_dir")
		_, err := os.Stat(policydir)
		if os.IsNotExist(err) {
			logger.Log("msg", fmt.Sprintf("creating missing policy download destination directory: %s", policydir))
			err = os.MkdirAll(policydir, 0755)
			return err
		} else if err != nil {
			return err
		}
	} else {
		return errors.New("Download directory for configuring Vault was not specified")
	}

	return nil
}
