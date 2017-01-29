package container

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"

	types "github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	dockerapi "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
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
	RemoveContainer(Container, bool, bool, bool, bool) error
	NetemContainer(Container, string, []string, net.IP, time.Duration, string, bool) error
	StopNetemContainer(Container, string, string, bool) error
	PauseContainer(Container, bool) error
	UnpauseContainer(Container, bool) error
}

// NewClient returns a new Client instance which can be used to interact with
// the Docker API.
func NewClient(dockerHost string, tlsConfig *tls.Config) Client {
	httpClient, err := HTTPClient(dockerHost, tlsConfig)
	if err != nil {
		log.Fatalf("Error instantiating Docker client: %s", err)
	}

	apiClient, err := dockerapi.NewClient(dockerHost, "", httpClient, nil)
	if err != nil {
		log.Fatalf("Error instantiating Docker engine-api: %s", err)
	}

	return dockerClient{containerAPI: apiClient, imageAPI: apiClient}
}

type dockerClient struct {
	containerAPI dockerapi.ContainerAPIClient
	imageAPI     dockerapi.ImageAPIClient
}

func (client dockerClient) ListContainers(fn Filter) ([]Container, error) {
	cs := []Container{}

	log.Debug("Retrieving running containers")

	runningContainers, err := client.containerAPI.ContainerList(apiContext(), types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}
	for _, runningContainer := range runningContainers {
		containerInfo, err := client.containerAPI.ContainerInspect(apiContext(), runningContainer.ID)
		if err != nil {
			return nil, err
		}
		log.Debugf("Running container: %s - (%s)", containerInfo.Name, containerInfo.ID)

		imageInfo, _, err := client.imageAPI.ImageInspectWithRaw(apiContext(), containerInfo.Image)
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
		if err := client.containerAPI.ContainerKill(apiContext(), c.ID(), signal); err != nil {
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
		if err := client.containerAPI.ContainerKill(apiContext(), c.ID(), signal); err != nil {
			return err
		}

		// Wait for container to exit, but proceed anyway after the timeout elapses
		if err := client.waitForStop(c, timeout); err != nil {
			log.Debugf("Error waiting for container %s (%s) to stop: ''%s'", c.Name(), c.ID(), err.Error())
		}

		log.Debugf("Killing container %s with %s", c.ID(), defaultKillSignal)
		if err := client.containerAPI.ContainerKill(apiContext(), c.ID(), defaultKillSignal); err != nil {
			return err
		}

		// Wait for container to be removed. In this case an error is a good thing
		if err := client.waitForStop(c, timeout); err == nil {
			return fmt.Errorf("Container %s (%s) could not be stopped", c.Name(), c.ID())
		}
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
		removeOpts := types.ContainerRemoveOptions{
			RemoveVolumes: links,
			RemoveLinks:   volumes,
			Force:         force,
		}
		return client.containerAPI.ContainerRemove(apiContext(), c.ID(), removeOpts)
	}
	return nil
}

func (client dockerClient) NetemContainer(c Container, netInterface string, netemCmd []string, targetIP net.IP, duration time.Duration, tcimage string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	var err error
	if targetIP == nil {
		log.Infof("%sRunning netem command '%s' on container %s for %s", prefix, netemCmd, c.ID(), duration)
		err = client.startNetemContainer(c, netInterface, netemCmd, tcimage, dryrun)
	} else {
		log.Infof("%sRunning netem command '%s' on container %s with filter %s for %s", prefix, netemCmd, c.ID(), targetIP.String(), duration)
		err = client.startNetemContainerIPFilter(c, netInterface, netemCmd, targetIP.String(), tcimage, dryrun)
	}
	if err != nil {
		log.Error(err)
	}
	return err
}

func (client dockerClient) StopNetemContainer(c Container, netInterface string, tcimage string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sStopping netem on container %s", prefix, c.ID())
	err := client.stopNetemContainer(c, netInterface, tcimage, dryrun)
	if err != nil {
		log.Error(err)
	}
	return err
}

func (client dockerClient) PauseContainer(c Container, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sPausing container %s", prefix, c.ID())
	if !dryrun {
		if err := client.containerAPI.ContainerPause(context.Background(), c.ID()); err != nil {
			return err
		}
		log.Debugf("Container %s paused", c.ID())
	}
	return nil
}

func (client dockerClient) UnpauseContainer(c Container, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sUnpausing container %s", prefix, c.ID())
	if !dryrun {
		if err := client.containerAPI.ContainerUnpause(context.Background(), c.ID()); err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

func (client dockerClient) startNetemContainer(c Container, netInterface string, netemCmd []string, tcimage string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sStart netem for container %s on '%s' with command '%s'", prefix, c.ID(), netInterface, netemCmd)
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control:
		// 'tc qdisc add dev eth0 root netem delay 100ms'
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		netemCommand := append([]string{"qdisc", "add", "dev", netInterface, "root", "netem"}, netemCmd...)
		// stop disruption command
		// netemStopCommand := "tc qdisc del dev eth0 root netem"
		log.Debugf("netem command '%s'", strings.Join(netemCommand, " "))
		return client.tcCommand(c, netemCommand, tcimage)
	}
	return nil
}

func (client dockerClient) stopNetemContainer(c Container, netInterface string, tcimage string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sStop netem for container %s on '%s'", prefix, c.ID(), netInterface)
	if !dryrun {
		// stop netem command
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		netemCommand := []string{"qdisc", "del", "dev", netInterface, "root", "netem"}
		log.Debugf("netem command '%s'", strings.Join(netemCommand, " "))
		return client.tcCommand(c, netemCommand, tcimage)
	}
	return nil
}

func (client dockerClient) startNetemContainerIPFilter(c Container, netInterface string, netemCmd []string,
	targetIP string, tcimage string, dryrun bool) error {
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
		handleCommand := []string{"qdisc", "add", "dev", netInterface, "root", "handle", "1:", "prio"}
		log.Debugf("handleCommand %s", handleCommand)
		err := client.tcCommand(c, handleCommand, tcimage)
		if err != nil {
			return err
		}

		//  Delay everything in band 3
		// 'tc qdisc add dev <netInterface> parent 1:3 netem <netemCmd>'
		// See more: http://stuff.onse.fi/man?program=tc
		netemCommand := append([]string{"qdisc", "add", "dev", netInterface, "parent", "1:3", "netem"}, netemCmd...)
		log.Debugf("netemCommand %s", netemCommand)
		err = client.tcCommand(c, netemCommand, tcimage)
		if err != nil {
			return err
		}

		// # say traffic to $PORT is band 3
		// 'tc filter add dev <netInterface> protocol ip parent 1:0 prio 3 u32 match ip dst <targetIP> flowid 1:3'
		// See more: http://stuff.onse.fi/man?program=tc-u32
		filterCommand := []string{"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "3",
			"u32", "match", "ip", "dport", strings.ToLower(targetIP), "flowid", "1:3"}
		log.Debugf("filterCommand %s", filterCommand)
		return client.tcCommand(c, filterCommand, tcimage)
	}
	return nil
}

func (client dockerClient) tcCommand(c Container, args []string, tcimage string) error {
	if tcimage == "" {
		return client.execOnContainer(c, "tc", args, true)
	}
	return client.tcContainerCommand(c, args, tcimage)
}

// execute tc command using other container (with iproute2 package installed), using target container network stack
// try to use `gaiadocker\iproute2` image (Alpine + iproute2 package)
func (client dockerClient) tcContainerCommand(target Container, args []string, tcimage string) error {
	log.Debugf("target tc image: %s", tcimage)
	// container config
	config := ctypes.Config{
		Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Entrypoint: []string{"tc"},
		Cmd:        args,
		Image:      tcimage,
	}
	log.Debugf("Container Config: %s", config)
	// host config
	hconfig := ctypes.HostConfig{
		// auto remove container on tc command exit
		AutoRemove: true,
		// NET_ADMIN is required for "tc netem"
		CapAdd: []string{"NET_ADMIN"},
		// use target container network stack
		NetworkMode: ctypes.NetworkMode("container:" + target.ID()),
		// others
		PortBindings: nat.PortMap{},
		DNS:          []string{},
		DNSOptions:   []string{},
		DNSSearch:    []string{},
	}
	log.Debugf("Host Config: %s", hconfig)
	createResponse, err := client.containerAPI.ContainerCreate(context.Background(), &config, &hconfig, nil, "")
	if err != nil {
		return err
	}
	log.Debugf("tc container id: %s", createResponse.ID)
	return client.containerAPI.ContainerStart(context.Background(), createResponse.ID, types.ContainerStartOptions{})
}

func (client dockerClient) execOnContainer(c Container, execCmd string, execArgs []string, privileged bool) error {
	// trim all spaces from cmd
	execCmd = strings.Replace(execCmd, " ", "", -1)

	// check if command exists inside target container
	checkExists := types.ExecConfig{
		Cmd: []string{"which", execCmd},
	}
	exec, err := client.containerAPI.ContainerExecCreate(apiContext(), c.ID(), checkExists)
	if err != nil {
		return err
	}
	log.Debugf("checking if command %s exists", execCmd)
	err = client.containerAPI.ContainerExecStart(apiContext(), exec.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}
	checkInspect, err := client.containerAPI.ContainerExecInspect(apiContext(), exec.ID)
	if err != nil {
		return err
	}
	if checkInspect.ExitCode != 0 {
		return fmt.Errorf("command '%s' not found inside the %s (%s) container", execCmd, c.Name(), c.ID())
	}
	log.Debugf("command %s found: continue...", execCmd)

	// prepare exec config
	config := types.ExecConfig{
		Privileged: privileged,
		Cmd:        append([]string{execCmd}, execArgs...),
	}
	// execute the command
	exec, err = client.containerAPI.ContainerExecCreate(context.Background(), c.ID(), config)
	if err != nil {
		return err
	}
	log.Debugf("Starting Exec %s %s (%s)", execCmd, execArgs, exec.ID)
	err = client.containerAPI.ContainerExecStart(context.Background(), exec.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}
	exitInspect, err := client.containerAPI.ContainerExecInspect(context.Background(), exec.ID)
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
			if ci, err := client.containerAPI.ContainerInspect(apiContext(), c.ID()); err != nil {
				return err
			} else if !ci.State.Running {
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func apiContext() context.Context {
	return context.Background()
}
