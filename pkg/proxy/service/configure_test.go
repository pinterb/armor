package service

import (
	"github.com/cdwlabs/armor/pkg/config"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func setUp(t *testing.T) {
	_, err := os.Stat(config.PolicyConfigPathDefault)
	assert.Error(t, err, "expecting an error when stat'n policy config dir")
	assert.True(t, os.IsNotExist(err), "expecting error to be os.IsNotExist")
	err = os.MkdirAll(config.PolicyConfigPathDefault, 0755)
	assert.NoError(t, err, "not expecting an error when making policy config dir")
}

func tearDown(t *testing.T) {
	_, err := os.Stat(config.PolicyConfigPathDefault)
	assert.NoError(t, err, "not expecting an error when stat'n policy config dir")
	err = os.RemoveAll(config.PolicyConfigPathDefault)
	assert.NoError(t, err, "not expecting an error when removing all under policy config dir")
	_, err = os.Stat(config.PolicyConfigPathDefault)
	assert.Error(t, err, "expecting an error when stat'n policy config dir")
	assert.True(t, os.IsNotExist(err), "expecting error to be os.IsNotExist")
}

func TestConfigOptions_Validate_DestDir(t *testing.T) {
	opts := &ConfigOptions{
		URL:   "",
		Token: "nbkd193dnakd1ueadf3",
	}

	_, err := opts.validate()
	if assert.Error(t, err, "expecting an error when policy config dir does not exist") {
		assert.Contains(t, err.Error(), "policy download dest does not exist", "expecting error type of 'policy config dir does not exist'")
	}
}

func TestConfigOptions_Validate_SourceDir(t *testing.T) {
	setUp(t)
	defer tearDown(t)
	cwd, _ := os.Getwd()

	// missing source url
	opts := &ConfigOptions{}
	_, err := opts.validate()
	if assert.Error(t, err, "expecting an error when policy config src url is not set") {
		assert.Contains(t, err.Error(), "ConfigOptions.URL validation failed on 'required' check", "expecting URL validation error for missing URL")
	}

	// empty source url
	opts = &ConfigOptions{
		URL: "",
	}
	_, err = opts.validate()
	if assert.Error(t, err, "expecting an error when policy config src url is empty") {
		assert.Contains(t, err.Error(), "ConfigOptions.URL validation failed on 'required' check", "expecting URL validation error for empty URL")
	}

	// missing token
	missingdir := "/test-fixtures/configure/not-here"
	opts = &ConfigOptions{
		URL: cwd + missingdir,
	}

	_, err = opts.validate()
	if assert.Error(t, err, "expecting an error when policy config src token is empty") {
		assert.Contains(t, err.Error(), "ConfigOptions.Token validation failed on 'required' check", "expecting Token validation error for missing token")
	}

	// non-existent source directory
	opts.Token = "nbkd193dnakd1ueadf3"
	_, err = opts.validate()
	if assert.Error(t, err, "expecting an error when policy config src does not exist") {
		assert.Contains(t, err.Error(), missingdir+": no such file or directory", "expecting error type of 'no such file or directory'")
	}

	// this directory is current valid...
	//	// malformed source directory
	//	malformeddir := "/test-fixtures/configure/malformed"
	//	opts = &ConfigOptions{
	//		URL: cwd + malformeddir,
	//	}
	//	_, err = opts.validate()
	//	if assert.Error(t, err, "expecting an error when policy config src directory is malformed") {
	//		assert.Contains(t, err.Error(), "service/configure: policy source does
	//		not follow prescribed layout", "expecting error type of 'policy config
	//		dir does not exi//st'")
	//}

	// initial /sys/mounts
	initialmntsdir := "/test-fixtures/configure/initialmounts"
	opts = &ConfigOptions{
		URL:   cwd + initialmntsdir,
		Token: "nbkd193dnakd1ueadf3",
	}
	state, err := opts.validate()
	assert.NoError(t, err, "not expecting an error when policy config src directory is well formed initial mounts")
	state.dumpMeta()

	assert.True(t, state.hasSysMountRequests(), "expecting mount requests")
	postgresql, ok := state.SysMountAddReq["postgresql"]
	assert.True(t, ok, "expecting to find request")
	assert.Contains(t, postgresql.FullPath, "data/sys/mounts/postgresql/postgresql.json", "expecting to find fullpath")
	assert.Equal(t, sysMountAdd, postgresql.Action, "expecting a match on action")
	assert.Equal(t, "/sys/mounts/", postgresql.VaultEndPoint, "expecting match for vault endpoint")
	assert.Equal(t, "postgresql", postgresql.ConfigPath, "expecting match on config path")
	_, err = deserializeMountInput(postgresql.FullPath)
	assert.NoError(t, err, "not expecting an error when deserializing json config")

	aws, ok := state.SysMountAddReq["aws"]
	assert.True(t, ok, "expecting to find request")
	assert.Contains(t, aws.FullPath, "data/sys/mounts/aws/aws.json", "expecting to find fullpath")
	assert.Equal(t, sysMountAdd, aws.Action, "expecting a match on action")
	assert.Equal(t, "/sys/mounts/", aws.VaultEndPoint, "expecting match for vault endpoint")
	assert.Equal(t, "aws", aws.ConfigPath, "expecting match on config path")
	_, err = deserializeMountInput(aws.FullPath)
	assert.NoError(t, err, "not expecting an error when deserializing json config")

}
