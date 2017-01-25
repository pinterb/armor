package service

import (
	"errors"
	"fmt"
	"github.com/cdwlabs/armor/pkg/config"
	getter "github.com/hashicorp/go-getter"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/nats-io/nuid"
	"gopkg.in/go-playground/validator.v9"
	"os"
	"path/filepath"
	"regexp"
)

// use a single instance of Validate, it caches struct info
var (
	validate *validator.Validate
)

// validation errors
var (
	ErrDestUnset                 = errors.New("policy download dest not set in config")
	ErrDestDoesNotExist          = errors.New("policy download dest does not exist")
	ErrDestStatFail              = errors.New("policy download dest failed being stat'd")
	ErrSrcURLUnset               = errors.New("policy source url not set in config")
	ErrSrcDoesNotExist           = errors.New("policy source does not exist (download or sync failed")
	ErrSrcStatFail               = errors.New("policy source failed being stat'd")
	ErrSrcMalformed              = errors.New("policy source does not follow prescribed layout")
	ErrSrcMultiJSON              = errors.New("policy source subdirectory contains more than one json file")
	ErrSrcNoValidation           = errors.New("internal state error encountered. no actions determined")
	ErrStateSysMountAddReqEmpty  = errors.New("no valid vault configuration requests submitted for adding /sys/mounts/")
	ErrStateSysMountUpdReqEmpty  = errors.New("no valid vault configuration requests submitted for tuning /sys/mounts/")
	ErrStateMalformed            = errors.New("configuration requests is malformed")
	ErrSrcReqEmpty               = errors.New("no valid vault configuration files were found in source directory submitted")
	ErrStateSysAuthAddReqEmpty   = errors.New("no valid vault configuration requests submitted for adding /sys/auth/")
	ErrStateSysAuthDelReqEmpty   = errors.New("no valid vault configuration requests submitted for deleting /sys/auth/")
	ErrStateSysPolicyAddReqEmpty = errors.New("no valid vault configuration requests submitted for adding /sys/policy/")
	ErrStateSysPolicyDelReqEmpty = errors.New("no valid vault configuration requests submitted for deleting /sys/policy/")
)

// ConfigActionType describes the different types of configuration or policies
// we apply to an unsealed instance of Vault
type ConfigActionType int

const (
	sysMountAdd     ConfigActionType = iota // 0
	sysMountUpdate                          // 1
	sysAuthAdd                              // 2
	sysAuthDelete                           // 3
	sysPolicyAdd                            // 4
	sysPolicyDelete                         // 5
)

// ConfigOptions are used to configure an unsealed Vault instance with system
// mounts, auths and policies. Generally, configuration takes the form of
// a URL.  Initially, this URL will support a local directory. But it is
// designed to support Git/Mercurial repositories, AWS S3, and HTTP endpoints.
type ConfigOptions struct {
	URL   string `json:"url" validate:"required"`
	Token string `json:"token" validate:"required"`
}

// configOptsExp contains the necessary payload for performing the actual
// configuration updates to Vault.  It is strictly internal use only!
type configOptsExp struct {
	ConfigID        string                    `json:"config_id"`
	Token           string                    `json:"token"`
	SourceDir       string                    `json:"source_dir"`
	Actions         []ConfigActionType        `json:"actions"`
	SysMountAddReq  map[string]ConfigPathMeta `json:"sys_mount_add_req"`
	SysMountUpdReq  map[string]ConfigPathMeta `json:"sys_mount_upd_req"`
	SysAuthAddReq   map[string]ConfigPathMeta `json:"sys_auth_add_req"`
	SysAuthDelReq   map[string]ConfigPathMeta `json:"sys_auth_del_req"`
	SysPolicyAddReq map[string]ConfigPathMeta `json:"sys_policy_add_req"`
	SysPolicyDelReq map[string]ConfigPathMeta `json:"sys_policy_del_req"`
}

// ConfigState represents the current state of Vault after performing
// a config operation.
type ConfigState struct {
	ConfigID string                     `json:"config_id"`
	Mounts   map[string]MountOutput     `json:"mounts"`
	Auths    map[string]AuthMountOutput `json:"auths"`
	Policies []string                   `json:"policies"`
}

// ensures that the Configure request payload is valid and for valid
// payloads then retrieves the requested URL resource.
func (opts *ConfigOptions) validate() (configOptsExp, error) {
	cfg := config.Config()
	var policyConfigDir string
	if cfg.IsSet("policy_config_dir") && cfg.GetString("policy_config_dir") != "" {
		policyConfigDir = cfg.GetString("policy_config_dir")
		_, err := os.Stat(policyConfigDir)
		if os.IsNotExist(err) {
			return configOptsExp{}, ErrDestDoesNotExist
		} else if err != nil {
			return configOptsExp{}, ErrDestStatFail
		}
	} else {
		return configOptsExp{}, ErrDestUnset
	}

	validate = validator.New()
	err := validate.Struct(opts)
	if err != nil {
		validationerr := ""
		for _, err := range err.(validator.ValidationErrors) {
			//fmt.Printf("Error: ActualTag: %s", err.ActualTag())
			//fmt.Printf("Error: Field: %s", err.Field())
			//fmt.Printf("Error: Kind: %s", err.Kind())
			//fmt.Printf("Error: Namespace: %s", err.Namespace())
			//fmt.Printf("Error: Param: %s", err.Param())
			//fmt.Printf("Error: StructNamespace: %s", err.StructNamespace())
			//fmt.Printf("Error: StructField: %s", err.StructField())
			//fmt.Printf("Error: Tag: %s", err.Tag())
			validationerr = fmt.Sprintf("%s validation failed on '%s' check", err.Namespace(), err.Tag())
			break
		}

		if validationerr != "" {
			return configOptsExp{}, fmt.Errorf(validationerr)
		}
		return configOptsExp{}, fmt.Errorf("Invalid configuration option(s)")
	}

	requestid := nuid.Next()
	srcdest := policyConfigDir + "/" + requestid
	err = getter.Get(srcdest, opts.URL)
	if err != nil {
		return configOptsExp{}, err
	}

	// validate source directory for correctness
	srcdata := srcdest + "/data"
	_, err = os.Stat(srcdata)
	if os.IsNotExist(err) {
		return configOptsExp{}, ErrSrcMalformed
	} else if err != nil {
		return configOptsExp{}, err
	}

	// To continue with validation, create our internal, expanded configuration
	// options. It's here that we determine what kind of updates are being
	// requested.  And if no valid updates are requested, we treat that as an
	// error condition as well.
	cfgState := configOptsExp{
		ConfigID:  requestid,
		Token:     opts.Token,
		SourceDir: srcdata,
		Actions:   make([]ConfigActionType, 0, 25),
	}

	// categorize individual configuration files
	err = cfgState.categorize()
	if err != nil {
		return cfgState, err
	}

	// no valid requests found, that's a problem.
	if len(cfgState.Actions) == 0 {
		return cfgState, ErrSrcReqEmpty
	}

	return cfgState, nil
}

// Perform any configuration updates to Vault.
func (opts *configOptsExp) categorize() error {
	metaset, err := filesByExt(opts.SourceDir, ".json")
	if err != nil {
		return err
	}

	// find /sys/mounts/
	err = opts.findSysMounts(metaset)
	if err != nil {
		return err
	}

	// find /sys/auth/
	err = opts.findSysAuths(metaset)
	if err != nil {
		return err
	}

	// find /sys/policy/
	err = opts.findSysPolicies(metaset)
	return err
}

// Perform any configuration updates to Vault.
func (opts *configOptsExp) handleRequests(client *vaultapi.Client) (ConfigState, error) {

	state := ConfigState{
		ConfigID: opts.ConfigID,
	}

	if opts.hasSysMountRequests() {
		mounts, err := opts.handleSysMounts(client)
		if err != nil {
			return state, err
		}
		state.Mounts = mounts
	}

	if opts.hasSysAuthRequests() {
		auths, err := opts.handleSysAuths(client)
		if err != nil {
			return state, err
		}
		state.Auths = auths
	}

	if opts.hasSysPolicyRequests() {
		policies, err := opts.handleSysPolicies(client)
		if err != nil {
			return state, err
		}
		state.Policies = policies
	}

	return state, nil
}

func (opts *configOptsExp) dumpMeta() error {

	fmt.Println("")
	fmt.Println("/sys/mounts/ adds:")
	if len(opts.SysMountAddReq) == 0 {
		fmt.Println("no requests found")
	} else {
		for path, meta := range opts.SysMountAddReq {
			fmt.Printf("path: %q\n\tfullpath: %q\n\tbase: %q\n\tvault endpoint: %q\n\taction: %d\n\tpath: %q\n\tfile: %q\n", path, meta.FullPath, meta.BasePath, meta.VaultEndPoint, meta.Action, meta.ConfigPath, meta.File)
		}
	}

	fmt.Println("")
	fmt.Println("/sys/mounts/ tune:")
	if len(opts.SysMountUpdReq) == 0 {
		fmt.Println("no requests found")
	} else {
		for path, meta := range opts.SysMountUpdReq {
			fmt.Printf("path: %q\n\tfullpath: %q\n\tbase: %q\n\tvault endpoint: %q\n\taction: %d\n\tpath: %q\n\tfile: %q\n", path, meta.FullPath, meta.BasePath, meta.VaultEndPoint, meta.Action, meta.ConfigPath, meta.File)
		}
	}

	fmt.Println("")
	fmt.Println("/sys/auth/ adds:")
	if len(opts.SysAuthAddReq) == 0 {
		fmt.Println("no requests found")
	} else {
		for path, meta := range opts.SysAuthAddReq {
			fmt.Printf("path: %q\n\tfullpath: %q\n\tbase: %q\n\tvault endpoint: %q\n\taction: %d\n\tpath: %q\n\tfile: %q\n", path, meta.FullPath, meta.BasePath, meta.VaultEndPoint, meta.Action, meta.ConfigPath, meta.File)
		}
	}

	fmt.Println("")
	fmt.Println("/sys/auth/ disable:")
	if len(opts.SysAuthDelReq) == 0 {
		fmt.Println("no requests found")
	} else {
		for path, meta := range opts.SysAuthDelReq {
			fmt.Printf("path: %q\n\tfullpath: %q\n\tbase: %q\n\tvault endpoint: %q\n\taction: %d\n\tpath: %q\n\tfile: %q\n", path, meta.FullPath, meta.BasePath, meta.VaultEndPoint, meta.Action, meta.ConfigPath, meta.File)
		}
	}

	fmt.Println("")
	fmt.Println("/sys/policy/ adds:")
	if len(opts.SysPolicyAddReq) == 0 {
		fmt.Println("no requests found")
	} else {
		for path, meta := range opts.SysPolicyAddReq {
			fmt.Printf("path: %q\n\tfullpath: %q\n\tbase: %q\n\tvault endpoint: %q\n\taction: %d\n\tpath: %q\n\tfile: %q\n", path, meta.FullPath, meta.BasePath, meta.VaultEndPoint, meta.Action, meta.ConfigPath, meta.File)
		}
	}

	fmt.Println("")
	fmt.Println("/sys/policy/ disable:")
	if len(opts.SysPolicyDelReq) == 0 {
		fmt.Println("no requests found")
	} else {
		for path, meta := range opts.SysPolicyDelReq {
			fmt.Printf("path: %q\n\tfullpath: %q\n\tbase: %q\n\tvault endpoint: %q\n\taction: %d\n\tpath: %q\n\tfile: %q\n", path, meta.FullPath, meta.BasePath, meta.VaultEndPoint, meta.Action, meta.ConfigPath, meta.File)
		}
	}

	return nil
}

// Given a search path, find all files with a given extension. This does
// perform a recursive search into sub-directories.
func filesByExt(searchpath, ext string) ([]ConfigPathMeta, error) {
	var files []ConfigPathMeta
	_, err := os.Stat(searchpath)
	if os.IsNotExist(err) {
		return files, nil
	} else if err != nil {
		return nil, err
	}

	filepath.Walk(searchpath, func(path string, f os.FileInfo, _ error) error {
		if !f.IsDir() {
			r, err := regexp.MatchString(ext, f.Name())
			if err == nil && r {
				files = append(files, ConfigPathMeta{
					FullPath: path,
					BasePath: searchpath,
					File:     f.Name(),
				})
			}
		}
		return nil
	})

	return files, nil
}
