package service

import (
	"encoding/json"
	vaultapi "github.com/hashicorp/vault/api"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// PolicyInput describes the request details for managing policies in a Vault
// instance.
type PolicyInput struct {
	Rules string `json:"rules"`
}

func (opts *configOptsExp) hasSysPolicyRequests() bool {
	return (len(opts.SysPolicyAddReq) > 0) || (len(opts.SysPolicyDelReq) > 0)
}

// searches the source for /sys/policy/xxx
func (opts *configOptsExp) findSysPolicies(metaset []ConfigPathMeta) error {
	if len(metaset) == 0 {
		return nil
	}

	syspolicylit := "/sys/policy/"

	// literals describing type of action
	tunelit := "/tune/"
	disablelit := "/disable/"
	addlit := "/"

	sysPolicyAddReq := make(map[string]ConfigPathMeta)
	sysPolicyDelReq := make(map[string]ConfigPathMeta)

	for _, meta := range metaset {
		// substring one - split file from full path
		subsone, _ := filepath.Split(meta.FullPath)

		// substring two - strip base from beginning of string
		substwo := subsone[len(meta.BasePath):]

		// substring three - determine what type of vault config is being submitted
		// (e.g. /sys/mounts/).  And substring - determine type of update (e.g.
		// tune or disable)
		substhree := ""
		subsfour := ""

		if strings.HasPrefix(substwo, syspolicylit) {
			substhree = substwo[len(syspolicylit):]
			meta.VaultEndPoint = syspolicylit

			if strings.HasSuffix(substhree, tunelit) {
				subsfour = substhree[:len(substhree)-len(tunelit)]
				// tuning auths is currently unsupported
				continue

			} else if strings.HasSuffix(substhree, disablelit) {
				subsfour = substhree[:len(substhree)-len(disablelit)]
				meta.ConfigPath = subsfour
				meta.Action = sysPolicyDelete
				sysPolicyDelReq[subsfour] = meta

			} else if strings.HasSuffix(substhree, addlit) {
				subsfour = substhree[:len(substhree)-len(addlit)]
				meta.ConfigPath = subsfour
				meta.Action = sysPolicyAdd
				sysPolicyAddReq[subsfour] = meta

			} else {
				// not sure we will ever get here...just defensive programming
				continue
			}

		} else {
			// string prefix is not /sys/policy/
			continue
		}
	} // for loop

	// Populate our configOptsExp with the categorization results
	opts.SysPolicyAddReq = sysPolicyAddReq
	if len(opts.SysPolicyAddReq) > 0 {
		opts.Actions = append(opts.Actions, sysPolicyAdd)
	}

	opts.SysPolicyDelReq = sysPolicyDelReq
	if len(opts.SysPolicyDelReq) > 0 {
		opts.Actions = append(opts.Actions, sysPolicyDelete)
	}

	return nil
}

// Perform any updates to /sys/policy/ in Vault.
func (opts *configOptsExp) handleSysPolicies(client *vaultapi.Client) ([]string, error) {

	madechgs := false
	var err error
	if len(opts.SysPolicyAddReq) > 0 {
		err = opts.addSysPolicies(client)
		if err != nil {
			return nil, err
		}
		madechgs = true
	}

	if len(opts.SysPolicyDelReq) > 0 {
		err = opts.deleteSysPolicies(client)
		if err != nil {
			return nil, err
		}
		madechgs = true
	}

	// if there were policy changes, then get list of policies from Vault
	var policies []string
	if madechgs {
		policies, err = listPolicies(client)
		if err != nil {
			return nil, err
		}
	}

	return policies, nil
}

// Get all the current policies in Vault.
func listPolicies(client *vaultapi.Client) ([]string, error) {

	result, err := client.Sys().ListPolicies()
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Add new policies to Vault
func (opts *configOptsExp) addSysPolicies(client *vaultapi.Client) error {
	if len(opts.SysPolicyAddReq) == 0 {
		return ErrStateSysPolicyAddReqEmpty
	}

	for path, meta := range opts.SysPolicyAddReq {
		policyInput, err := deserializePolicyInput(meta.FullPath)
		if err != nil {
			return err
		}

		err = client.Sys().PutPolicy(path, policyInput.Rules)
		if err != nil {
			return err
		}
	}

	return nil
}

// Delete existing policies in Vault
func (opts *configOptsExp) deleteSysPolicies(client *vaultapi.Client) error {
	if len(opts.SysPolicyDelReq) == 0 {
		return ErrStateSysPolicyDelReqEmpty
	}

	// NOTE: deleting policies in Vault doesn't require any real json file.
	// Disable by path name only.
	for path := range opts.SysPolicyDelReq {
		err := client.Sys().DeletePolicy(path)
		if err != nil {
			return err
		}
	}

	return nil
}

// Unmarshal a json file into a PolicyInput. A convenience func to help with
// testing.
func deserializePolicyInput(path string) (PolicyInput, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return PolicyInput{}, err
	}

	// unmarshal json file into generic map
	var policyIn PolicyInput
	err = json.Unmarshal(raw, &policyIn)
	if err != nil {
		return PolicyInput{}, err
	}

	return policyIn, nil
}
