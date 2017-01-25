package service

import (
	"encoding/json"
	vaultapi "github.com/hashicorp/vault/api"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// MountInput maps directly to Vault's own MountInput.
type MountInput struct {
	Type        string           `json:"type"`
	Description string           `json:"description"`
	Config      MountConfigInput `json:"config,omitempty"`
}

// MountConfigInput describes the lease details of requested mount.
type MountConfigInput struct {
	DefaultLeaseTTL string `json:"default_lease_ttl,omitempty"`
	MaxLeaseTTL     string `json:"max_lease_ttl,omitempty"`
}

// MountOutput maps directly to Vault's own MountOutput. Used by ConfigState to
// describe the mounts currently defined in a Vault instance.
type MountOutput struct {
	Path        string            `json:"path"`
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Config      MountConfigOutput `json:"config,omitempty"`
}

// MountConfigOutput describes the lease details of an individual mount.
type MountConfigOutput struct {
	DefaultLeaseTTL int `json:"default_lease_ttl,omitempty"`
	MaxLeaseTTL     int `json:"max_lease_ttl,omitempty"`
}

func (opts *configOptsExp) hasSysMountRequests() bool {
	return (len(opts.SysMountAddReq) > 0) || (len(opts.SysMountUpdReq) > 0)
}

// searches the source for /sys/mounts/xxx
func (opts *configOptsExp) findSysMounts(metaset []ConfigPathMeta) error {
	if len(metaset) == 0 {
		return nil
	}

	sysmountslit := "/sys/mounts/"

	// literals describing type of action
	tunelit := "/tune/"
	disablelit := "/disable/"
	addlit := "/"

	sysMountAddReq := make(map[string]ConfigPathMeta)
	sysMountUpdReq := make(map[string]ConfigPathMeta)

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

		if strings.HasPrefix(substwo, sysmountslit) {
			substhree = substwo[len(sysmountslit):]
			meta.VaultEndPoint = sysmountslit

			if strings.HasSuffix(substhree, tunelit) {
				subsfour = substhree[:len(substhree)-len(tunelit)]
				meta.ConfigPath = subsfour
				meta.Action = sysMountUpdate
				sysMountUpdReq[subsfour] = meta

			} else if strings.HasSuffix(substhree, disablelit) {
				subsfour = substhree[:len(substhree)-len(disablelit)]
				// unmounting is currently unsupported
				continue

			} else if strings.HasSuffix(substhree, addlit) {
				subsfour = substhree[:len(substhree)-len(addlit)]
				meta.ConfigPath = subsfour
				meta.Action = sysMountAdd
				sysMountAddReq[subsfour] = meta

			} else {
				// not sure we will ever get here...just defensive programming
				continue
			}

		} else {
			// string prefix is not /sys/mounts/
			continue
		}
	} // for loop

	// Populate our configOptsExp with the categorization results
	opts.SysMountAddReq = sysMountAddReq
	if len(opts.SysMountAddReq) > 0 {
		opts.Actions = append(opts.Actions, sysMountAdd)
	}

	opts.SysMountUpdReq = sysMountUpdReq
	if len(opts.SysMountUpdReq) > 0 {
		opts.Actions = append(opts.Actions, sysMountUpdate)
	}

	return nil
}

// Perform any updates to /sys/mounts/ in Vault.
func (opts *configOptsExp) handleSysMounts(client *vaultapi.Client) (map[string]MountOutput, error) {

	madechgs := false
	var err error
	if len(opts.SysMountAddReq) > 0 {
		err = opts.addSysMounts(client)
		if err != nil {
			return nil, err
		}
		madechgs = true
	}

	if len(opts.SysMountUpdReq) > 0 {
		err = opts.tuneSysMounts(client)
		if err != nil {
			return nil, err
		}
		madechgs = true
	}

	// get list of mounts from Vault
	var mounts map[string]MountOutput
	if madechgs {
		// get list of mounts from Vault
		mounts, err = listMounts(client)
		if err != nil {
			return nil, err
		}
	}

	return mounts, nil
}

// Get all the current mounts in Vault.
func listMounts(client *vaultapi.Client) (map[string]MountOutput, error) {

	result, err := client.Sys().ListMounts()
	if err != nil {
		return nil, err
	}

	out := make(map[string]MountOutput)
	for k, v := range result {
		mountCfgOut := MountConfigOutput{
			DefaultLeaseTTL: v.Config.DefaultLeaseTTL,
			MaxLeaseTTL:     v.Config.MaxLeaseTTL,
		}

		mountOut := MountOutput{
			Type:        v.Type,
			Description: v.Description,
			Config:      mountCfgOut,
		}

		out[k] = mountOut
	}

	return out, nil
}

// Add new mounts to Vault
func (opts *configOptsExp) addSysMounts(client *vaultapi.Client) error {
	if len(opts.SysMountAddReq) == 0 {
		return ErrStateSysMountAddReqEmpty
	}

	for path, meta := range opts.SysMountAddReq {
		mountInput, err := deserializeMountInput(meta.FullPath)
		if err != nil {
			return err
		}

		mountCfgIn := vaultapi.MountConfigInput{
			DefaultLeaseTTL: mountInput.Config.DefaultLeaseTTL,
			MaxLeaseTTL:     mountInput.Config.MaxLeaseTTL,
		}

		mountIn := &vaultapi.MountInput{
			Type:        mountInput.Type,
			Description: mountInput.Description,
			Config:      mountCfgIn,
		}

		err = client.Sys().Mount(path, mountIn)
		if err != nil {
			return err
		}
	}

	return nil
}

// Update existing mounts to Vault
func (opts *configOptsExp) tuneSysMounts(client *vaultapi.Client) error {
	if len(opts.SysMountUpdReq) == 0 {
		return ErrStateSysMountUpdReqEmpty
	}

	for path, meta := range opts.SysMountUpdReq {
		mountInput, err := deserializeMountConfigInput(meta.FullPath)
		if err != nil {
			return err
		}

		mountCfgIn := vaultapi.MountConfigInput{
			DefaultLeaseTTL: mountInput.DefaultLeaseTTL,
			MaxLeaseTTL:     mountInput.MaxLeaseTTL,
		}

		err = client.Sys().TuneMount(path, mountCfgIn)
		if err != nil {
			return err
		}
	}

	return nil
}

// Unmarshal a json file into a MountInput. A convenience func to help with
// testing.
func deserializeMountInput(path string) (MountInput, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return MountInput{}, err
	}

	// unmarshal json file into generic map
	var mountIn MountInput
	err = json.Unmarshal(raw, &mountIn)
	if err != nil {
		return MountInput{}, err
	}

	return mountIn, nil
}

// Unmarshal a json file into a MountConfigInput. A convenience func to help with
// testing.
func deserializeMountConfigInput(path string) (MountConfigInput, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return MountConfigInput{}, err
	}

	// unmarshal json file into generic map
	var mountCfgIn MountConfigInput
	err = json.Unmarshal(raw, &mountCfgIn)
	if err != nil {
		return MountConfigInput{}, err
	}

	return mountCfgIn, nil
}
