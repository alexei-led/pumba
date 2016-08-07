package container

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/samalba/dockerclient"

	engineapi "github.com/docker/engine-api/client"
	enginetypes "github.com/docker/engine-api/types"
)

const (
	defaultStopSignal = "SIGTERM"
	defaultKillSignal = "SIGKILL"
	dryRunPrefix      = "DRY: "
)

// A Filter is a prototype for a function that can be used to filter the
// results from a call to the ListContainers() method on the Client.
type Filter func(Container) bool

// Client interface
type Client interface {
	ListContainers(Filter) ([]Container, error)
	StopContainer(Container, int, bool) error
	KillContainer(Container, string, bool) error
	StartContainer(Container) error
	RenameContainer(Container, string) error
	RemoveImage(Container, bool, bool) error
	RemoveContainer(Container, bool, bool, bool, bool) error
	NetemContainer(Container, string, string, net.IP, time.Duration, bool) error
	StopNetemContainer(Container, string, bool) error
	PauseContainer(Container, time.Duration, bool) error
}

// NewClient returns a new Client instance which can be used to interact with
// the Docker API.
func NewClient(dockerHost string, tlsConfig *tls.Config) Client {
	docker, err := dockerclient.NewDockerClient(dockerHost, tlsConfig)
	if err != nil {
		log.Fatalf("Error instantiating Docker client: %s", err)
	}

	// Use HTTP Client used by dockerclient to create engine-api client
	apiClient, err := engineapi.NewClient(dockerHost, "", docker.HTTPClient, nil)
	if err != nil {
		log.Fatalf("Error instantiating Docker engine-api: %s", err)
	}

	return dockerClient{api: docker, apiClient: apiClient}
}

type dockerClient struct {
	api dockerclient.Client
	// NOTE: use official docker/engine-api instead of samalba/dockerclient; lazy refactoring
	apiClient engineapi.ContainerAPIClient
}

func (client dockerClient) ListContainers(fn Filter) ([]Container, error) {
	cs := []Container{}

	log.Debug("Retrieving running containers")

	runningContainers, err := client.api.ListContainers(false, false, "")
	if err != nil {
		return nil, err
	}
	for _, runningContainer := range runningContainers {
		containerInfo, err := client.api.InspectContainer(runningContainer.Id)
		if err != nil {
			return nil, err
		}
		log.Debugf("Running container: %s - (%s)", containerInfo.Name, containerInfo.Id)

		imageInfo, err := client.api.InspectImage(containerInfo.Image)
		if err != nil {
			return nil, err
		}

		c := Container{containerInfo: containerInfo, imageInfo: imageInfo}
		if fn(c) {
			cs = append(cs, c)
		}
	}

	return cs, nil
}

func (client dockerClient) KillContainer(c Container, signal string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sKilling %s (%s) with signal %s", prefix, c.Name(), c.ID(), signal)
	if !dryrun {
		if err := client.api.KillContainer(c.ID(), signal); err != nil {
			return err
		}
	}
	return nil
}

func (client dockerClient) StopContainer(c Container, timeout int, dryrun bool) error {
	signal := c.StopSignal()
	if signal == "" {
		signal = defaultStopSignal
	}
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sStopping %s (%s) with %s", prefix, c.Name(), c.ID(), signal)
	if !dryrun {
		if err := client.api.KillContainer(c.ID(), signal); err != nil {
			return err
		}

		// Wait for container to exit, but proceed anyway after the timeout elapses
		if err := client.waitForStop(c, timeout); err != nil {
			log.Debugf("Error waiting for container %s (%s) to stop: ''%s'", c.Name(), c.ID(), err.Error())
		}

		log.Debugf("Killing container %s with %s", c.ID(), defaultKillSignal)
		if err := client.api.KillContainer(c.ID(), defaultKillSignal); err != nil {
			return err
		}

		// Wait for container to be removed. In this case an error is a good thing
		if err := client.waitForStop(c, timeout); err == nil {
			return fmt.Errorf("Container %s (%s) could not be stopped", c.Name(), c.ID())
		}
	}

	return nil
}

func (client dockerClient) StartContainer(c Container) error {
	config := c.runtimeConfig()
	hostConfig := c.hostConfig()
	name := c.Name()

	log.Infof("Starting %s", name)

	newContainerID, err := client.api.CreateContainer(config, name, nil)
	if err != nil {
		return err
	}

	log.Debugf("Starting container %s (%s)", name, newContainerID)

	return client.api.StartContainer(newContainerID, hostConfig)
}

func (client dockerClient) RenameContainer(c Container, newName string) error {
	log.Debugf("Renaming container %s (%s) to %s", c.Name(), c.ID(), newName)
	return client.api.RenameContainer(c.ID(), newName)
}

func (client dockerClient) RemoveImage(c Container, force bool, dryrun bool) error {
	imageID := c.ImageID()
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sRemoving image %s", prefix, imageID)
	if !dryrun {
		_, err := client.api.RemoveImage(imageID, force)
		return err
	}
	return nil
}

func (client dockerClient) RemoveContainer(c Container, force bool, links bool, volumes bool, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sRemoving container %s", prefix, c.ID())
	if !dryrun {
		removeOpts := enginetypes.ContainerRemoveOptions{
			RemoveVolumes: links,
			RemoveLinks:   volumes,
			Force:         force,
		}
		return client.apiClient.ContainerRemove(context.Background(), c.ID(), removeOpts)
	}
	return nil
}

func (client dockerClient) NetemContainer(c Container, netInterface string, netemCmd string, targetIP net.IP, duration time.Duration, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	var err error
	if targetIP == nil {
		log.Infof("%sRunning netem command '%s' on container %s for %s", prefix, netemCmd, c.ID(), duration)
		err = client.startNetemContainer(c, netInterface, netemCmd, dryrun)
	} else {
		log.Infof("%sRunning netem command '%s' on container %s with filter %s for %s", prefix, netemCmd, c.ID(), targetIP.String(), duration)
		err = client.startNetemContainerIPFilter(c, netInterface, netemCmd, targetIP.String(), dryrun)
	}
	return err
}

func (client dockerClient) StopNetemContainer(c Container, netInterface string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sStopping netem on container %s", prefix, c.ID())
	return client.stopNetemContainer(c, netInterface, dryrun)
}

func (client dockerClient) PauseContainer(c Container, duration time.Duration, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sPausing container %s for %s", prefix, c.ID(), duration)
	if !dryrun {
		if err := client.api.PauseContainer(c.ID()); err != nil {
			return err
		}
		log.Debugf("Container %s paused for %s", c.ID(), duration)
		// TODO: FIXME: pause the current goroutine for specified duration
		time.Sleep(duration)
		if err := client.api.UnpauseContainer(c.ID()); err != nil {
			return err
		}
		log.Debugf("Container upaused %s after %s", c.ID(), duration)
	}
	return nil
}

func (client dockerClient) startNetemContainer(c Container, netInterface string, netemCmd string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sStart netem for container %s on '%s' with command '%s'", prefix, c.ID(), netInterface, netemCmd)
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control:
		// 'tc qdisc add dev eth0 root netem delay 100ms'
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		netemCommand := "qdisc add dev " + netInterface + " root netem " + strings.ToLower(netemCmd)
		// stop disruption command
		// netemStopCommand := "tc qdisc del dev eth0 root netem"
		log.Debugf("netem command '%s'", netemCommand)
		return client.execOnContainer(c, "tc", netemCommand, true)
	}
	return nil
}

func (client dockerClient) stopNetemContainer(c Container, netInterface string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sStop netem for container %s on '%s'", prefix, c.ID(), netInterface)
	if !dryrun {
		// stop netem command
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		netemCommand := "qdisc del dev " + netInterface + " root netem"
		log.Debugf("netem command '%s'", netemCommand)
		return client.execOnContainer(c, "tc", netemCommand, true)
	}
	return nil
}

func (client dockerClient) startNetemContainerIPFilter(c Container, netInterface string, netemCmd string,
	targetIP string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sStart netem for container %s on '%s' with command '%s', filter by IP '%s'",
		prefix, c.ID(), netInterface, netemCmd, targetIP)
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control
		// to filter network, needs to create a priority scheduling, add a low priority
		// queue, apply netem command on that queue only, then route IP traffic to the low priority queue
		// See more: http://www.linuxfoundation.org/collaborate/workgroups/networking/netem

		//  Create a priority-based queue.
		// 'tc qdisc add dev <netInterface> root handle 1: prio'
		// See more: http://stuff.onse.fi/man?program=tc
		handleCommand := "qdisc add dev " + netInterface + " root handle 1: prio"
		log.Debugf("handleCommand %s", handleCommand)
		err := client.execOnContainer(c, "tc", handleCommand, true)
		if err != nil {
			return err
		}

		//  Delay everything in band 3
		// 'tc qdisc add dev <netInterface> parent 1:3 netem <netemCmd>'
		// See more: http://stuff.onse.fi/man?program=tc
		netemCommand := "qdisc add dev " + netInterface + " parent 1:3 netem " + strings.ToLower(netemCmd)
		log.Debugf("netemCommand %s", netemCommand)
		err = client.execOnContainer(c, "tc", netemCommand, true)
		if err != nil {
			return err
		}

		// # say traffic to $PORT is band 3
		// 'tc filter add dev <netInterface> protocol ip parent 1:0 prio 3 u32 match ip dst <targetIP> flowid 1:3'
		// See more: http://stuff.onse.fi/man?program=tc-u32
		filterCommand := "filter add dev " + netInterface + " protocol ip parent 1:0 prio 3 " +
			"u32 match ip dport " + strings.ToLower(targetIP) + " flowid 1:3"
		log.Debugf("filterCommand %s", filterCommand)
		return client.execOnContainer(c, "tc", filterCommand, true)
	}
	return nil
}

func (client dockerClient) execOnContainer(c Container, execCmd string, execArgs string, privileged bool) error {
	// trim all spaces from cmd
	execCmd = strings.Replace(execCmd, " ", "", -1)

	// check if command exists inside target container
	checkExists := enginetypes.ExecConfig{
		Cmd: []string{"which", execCmd},
	}
	exec, err := client.apiClient.ContainerExecCreate(context.Background(), c.ID(), checkExists)
	if err != nil {
		return err
	}
	log.Debugf("checking if command %s exists", execCmd)
	err = client.apiClient.ContainerExecStart(context.Background(), exec.ID, enginetypes.ExecStartCheck{})
	if err != nil {
		return err
	}
	checkInspect, err := client.apiClient.ContainerExecInspect(context.Background(), exec.ID)
	if err != nil {
		return err
	}
	if checkInspect.ExitCode != 0 {
		return fmt.Errorf("command '%s' not found inside the %s (%s) container", execCmd, c.Name(), c.ID())
	}
	log.Debugf("command %s found: continue...", execCmd)

	// prepare exec config
	config := enginetypes.ExecConfig{
		Privileged: privileged,
		Cmd:        strings.Split(execCmd+" "+execArgs, " "),
	}
	// execute the command
	exec, err = client.apiClient.ContainerExecCreate(context.Background(), c.ID(), config)
	if err != nil {
		return err
	}
	log.Debugf("Starting Exec %s %s (%s)", execCmd, execArgs, exec.ID)
	err = client.apiClient.ContainerExecStart(context.Background(), exec.ID, enginetypes.ExecStartCheck{})
	if err != nil {
		return err
	}
	exitInspect, err := client.apiClient.ContainerExecInspect(context.Background(), exec.ID)
	if err != nil {
		return err
	}
	if exitInspect.ExitCode != 0 {
		return fmt.Errorf("command '%s' failed in %s (%s) container; run it in manually to debug", execCmd, c.Name(), c.ID())
	}
	return nil
}

func (client dockerClient) waitForStop(c Container, waitTime int) error {
	timeout := time.After(time.Duration(waitTime) * time.Second)

	for {
		select {
		case <-timeout:
			return nil
		default:
			if ci, err := client.api.InspectContainer(c.ID()); err != nil {
				return err
			} else if !ci.State.Running {
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}
}
