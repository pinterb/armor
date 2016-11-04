package docker

import (
	"io"
	"os"
	"strings"

	"github.com/arschles/godo/log"
	dockutil "github.com/arschles/godo/util/docker"
	docker "github.com/fsouza/go-dockerclient"
)

func Run(client *docker.Client,
	image *dockutil.Image,
	taskName,
	cwd,
	containerMount,
	cmd string,
	env []string,
) (id string, err error) {

	mounts := []docker.Mount{
		{Name: "pwd", Source: cwd, Destination: containerMount, Mode: "rxw"},
	}
	cmdSpl := strings.Split(cmd, " ")

	containerName := dockutil.NewContainerName(taskName, cwd)
	createContainerOpts, hostConfig := dockutil.CreateAndStartContainerOpts(image.String(), containerName, cmdSpl, env, mounts, containerMount)
	if err := dockutil.EnsureImage(client, image.String(), func() (io.Writer, error) {
		return os.Stdout, nil
	}); err != nil {
		return nil, err
	}

	container, err := client.CreateContainer(createContainerOpts)
	if err != nil {
		return nil, err
	}

	dockutil.log.Debug(dockutil.CmdStr(createContainerOpts, hostConfig))
}
