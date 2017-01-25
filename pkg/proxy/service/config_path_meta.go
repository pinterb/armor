package service

import (
//"fmt"
//"io/ioutil"
//"os"
//"path/filepath"
)

// ConfigPathMeta is used to categorize individual configuration files (e.g.
// json).
type ConfigPathMeta struct {
	FullPath      string           `json:"full_path"`
	BasePath      string           `json:"base_path"`
	VaultEndPoint string           `json:"vault_base_path"`
	ConfigPath    string           `json:"config_path"`
	Action        ConfigActionType `json:"action"`
	File          string           `json:"file"`
}
