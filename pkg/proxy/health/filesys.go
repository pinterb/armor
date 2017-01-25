package health

import (
	"errors"
	"fmt"
	"github.com/go-kit/kit/log"
	"os"

	"github.com/spf13/viper"
)

func policyDirHealth(logger log.Logger) error {

	if viper.IsSet("policy_config_dir") && viper.GetString("policy_config_dir") != "" {
		policydir := viper.GetString("policy_config_dir")
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
