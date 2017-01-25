package service

import (
	"encoding/json"
	vaultapi "github.com/hashicorp/vault/api"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// AuthInput describes the request details for adding auth backends to a Vault
// instance.
type AuthInput struct {
	Type        string          `json:"type"`
	Description string          `json:"description"`
	Config      AuthConfigInput `json:"config,omitempty"`
}

// AuthConfigInput describes the lease details of requested mount.
type AuthConfigInput struct {
	DefaultLeaseTTL string `json:"default_lease_ttl,omitempty"`
	MaxLeaseTTL     string `json:"max_lease_ttl,omitempty"`
}

// AuthMountOutput maps directly to Vault's own AuthMount. Used by ConfigState to
// describe the auth mounts currently defined in a Vault instance.
type AuthMountOutput struct {
	Path        string           `json:"path"`
	Type        string           `json:"type"`
	Description string           `json:"description,omitempty"`
	Config      AuthConfigOutput `json:"config,omitempty"`
}

// AuthConfigOutput describes the lease details of an individual mount.
type AuthConfigOutput struct {
	DefaultLeaseTTL int `json:"default_lease_ttl,omitempty"`
	MaxLeaseTTL     int `json:"max_lease_ttl,omitempty"`
}

func (opts *configOptsExp) hasSysAuthRequests() bool {
	return (len(opts.SysAuthAddReq) > 0) || (len(opts.SysAuthDelReq) > 0)
}

// searches the source for /sys/auth/xxx
func (opts *configOptsExp) findSysAuths(metaset []ConfigPathMeta) error {
	if len(metaset) == 0 {
		return nil
	}

	sysauthslit := "/sys/auth/"

	// literals describing type of action
	tunelit := "/tune/"
	disablelit := "/disable/"
	addlit := "/"

	sysAuthAddReq := make(map[string]ConfigPathMeta)
	sysAuthDelReq := make(map[string]ConfigPathMeta)

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

		if strings.HasPrefix(substwo, sysauthslit) {
			substhree = substwo[len(sysauthslit):]
			meta.VaultEndPoint = sysauthslit

			if strings.HasSuffix(substhree, tunelit) {
				subsfour = substhree[:len(substhree)-len(tunelit)]
				// tuning auths is currently unsupported
				continue

			} else if strings.HasSuffix(substhree, disablelit) {
				subsfour = substhree[:len(substhree)-len(disablelit)]
				meta.ConfigPath = subsfour
				meta.Action = sysAuthDelete
				sysAuthDelReq[subsfour] = meta

			} else if strings.HasSuffix(substhree, addlit) {
				subsfour = substhree[:len(substhree)-len(addlit)]
				meta.ConfigPath = subsfour
				meta.Action = sysAuthAdd
				sysAuthAddReq[subsfour] = meta

			} else {
				// not sure we will ever get here...just defensive programming
				continue
			}

		} else {
			// string prefix is not /sys/auth/
			continue
		}
	} // for loop

	// Populate our configOptsExp with the categorization results
	opts.SysAuthAddReq = sysAuthAddReq
	if len(opts.SysAuthAddReq) > 0 {
		opts.Actions = append(opts.Actions, sysAuthAdd)
	}

	opts.SysAuthDelReq = sysAuthDelReq
	if len(opts.SysAuthDelReq) > 0 {
		opts.Actions = append(opts.Actions, sysAuthDelete)
	}

	return nil
}

// Perform any updates to /sys/auth/ in Vault.
func (opts *configOptsExp) handleSysAuths(client *vaultapi.Client) (map[string]AuthMountOutput, error) {

	madechgs := false
	var err error
	if len(opts.SysAuthAddReq) > 0 {
		err = opts.addSysAuths(client)
		if err != nil {
			return nil, err
		}
		madechgs = true
	}

	if len(opts.SysAuthDelReq) > 0 {
		err = opts.deleteSysAuths(client)
		if err != nil {
			return nil, err
		}
		madechgs = true
	}

	// if there were auth changes, then get list of auth backends from Vault
	var auths map[string]AuthMountOutput
	if madechgs {
		auths, err = listAuths(client)
		if err != nil {
			return nil, err
		}
	}

	return auths, nil
}

// Get all the current auth backends in Vault.
func listAuths(client *vaultapi.Client) (map[string]AuthMountOutput, error) {

	result, err := client.Sys().ListAuth()
	if err != nil {
		return nil, err
	}

	out := make(map[string]AuthMountOutput)
	for k, v := range result {
		cfgOut := AuthConfigOutput{
			DefaultLeaseTTL: v.Config.DefaultLeaseTTL,
			MaxLeaseTTL:     v.Config.MaxLeaseTTL,
		}

		mountOut := AuthMountOutput{
			Type:        v.Type,
			Description: v.Description,
			Config:      cfgOut,
		}

		out[k] = mountOut
	}

	return out, nil
}

// Add new auth backends to Vault
func (opts *configOptsExp) addSysAuths(client *vaultapi.Client) error {
	if len(opts.SysAuthAddReq) == 0 {
		return ErrStateSysAuthAddReqEmpty
	}

	for path, meta := range opts.SysAuthAddReq {
		authInput, err := deserializeAuthInput(meta.FullPath)
		if err != nil {
			return err
		}

		err = client.Sys().EnableAuth(path, authInput.Type, authInput.Description)
		if err != nil {
			return err
		}
	}

	return nil
}

// Disable existing auth backends in Vault
func (opts *configOptsExp) deleteSysAuths(client *vaultapi.Client) error {
	if len(opts.SysAuthDelReq) == 0 {
		return ErrStateSysAuthDelReqEmpty
	}

	// NOTE: disabling auth backends in Vault doesn't require any real json file.
	// Disable by path name only.
	for path := range opts.SysAuthDelReq {
		err := client.Sys().DisableAuth(path)
		if err != nil {
			return err
		}
	}

	return nil
}

// Unmarshal a json file into a AuthInput. A convenience func to help with
// testing.
func deserializeAuthInput(path string) (AuthInput, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return AuthInput{}, err
	}

	// unmarshal json file into generic map
	var authIn AuthInput
	err = json.Unmarshal(raw, &authIn)
	if err != nil {
		return AuthInput{}, err
	}

	return authIn, nil
}
