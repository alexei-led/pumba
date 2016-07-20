package container

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/samalba/dockerclient"
)

const (
	defaultStopSignal = "SIGTERM"
	defaultKillSignal = "SIGKILL"
	dryRunPrefix      = "DRY: "
)

// A Filter is a prototype for a function that can be used to filter the
// results from a call to the ListContainers() method on the Client.
type Filter func(Container) bool

// A Client is the interface through which Pumba interacts with the Docker API.
type Client interface {
	ListContainers(Filter) ([]Container, error)
	StopContainer(Container, int, bool) error
	KillContainer(Container, string, bool) error
	StartContainer(Container) error
	RenameContainer(Container, string) error
	RemoveImage(Container, bool, bool) error
	RemoveContainer(Container, bool, string, string, bool) error
	DisruptContainer(Container, string, string, bool) error
	PauseContainer(Container, time.Duration, bool) error
}

// NewClient returns a new Client instance which can be used to interact with
// the Docker API.
func NewClient(dockerHost string, tlsConfig *tls.Config) Client {
	docker, err := dockerclient.NewDockerClient(dockerHost, tlsConfig)

	if err != nil {
		log.Fatalf("Error instantiating Docker client: %s", err)
	}

	return dockerClient{api: docker}
}

type dockerClient struct {
	api dockerclient.Client
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

func (client dockerClient) RemoveContainer(c Container, force bool, link string, volumes string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sRemoving container %s", prefix, c.ID())
	if !dryrun {
		return client.api.RemoveContainer(c.ID(), force, len(volumes) > 0)
	}
	return nil
}

func (client dockerClient) DisruptContainer(c Container, netInterface string, netemCmd string, dryrun bool) error {
	// find out if we have command, ip or command:ip
	cmd := strings.Split(netemCmd, ":")
	if len(cmd) == 2 {
		return client.disruptContainerFilterNetwork(c, netInterface, cmd[0], cmd[1], dryrun)
	}
	return client.disruptContainerAllNetwork(c, netInterface, cmd[0], dryrun)
}

func (client dockerClient) PauseContainer(c Container, duration time.Duration, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sPausing container '%s' for '%s'", prefix, c.ID(), duration)
	if !dryrun {
		if err := client.api.PauseContainer(c.ID()); err != nil {
			return err
		}
		// pause the current goroutine for specified duration
		time.Sleep(duration)
		if err := client.api.UnpauseContainer(c.ID()); err != nil {
			return err
		}
	}
	return nil
}

func (client dockerClient) disruptContainerAllNetwork(c Container, netInterface string, netemCmd string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sDisrupting container %s with netem cmd %s", prefix, c.ID(), netemCmd)
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control:
		// 'tc qdisc add dev eth0 root netem delay 100ms'
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		netemCommand := "tc qdisc add dev " + netInterface + " root netem " + strings.ToLower(netemCmd)
		return client.execOnContainer(c, netemCommand)
	}
	return nil
}

func (client dockerClient) disruptContainerFilterNetwork(c Container, netInterface string, netemCmd string,
	targetIP string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sDisrupting container %s with netem cmd %s on traffic to %s",
		prefix, c.ID(), netemCmd, targetIP)
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control
		// to filter network, needs to create a priority scheduling, add a low priority
		//  queue, apply netem command on that queue only, then route IP traffic
		//  to the low priority queue
		// 'tc qdisc add dev eth0 root handle 1: prio'
		// 'tc qdisc add dev eth0 parent 1:3 netem delay 3000ms'
		// 'tc filter add dev eth0 protocol ip parent 1:0 prio 3 /
		//  u32 match ip dst 172.19.0.3 flowid 1:3'
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		handleCommand := "tc qdisc add dev " + netInterface + " root handle 1: prio"
		log.Debugf("Disrupt with handleCommand %s", handleCommand)
		err := client.execOnContainer(c, handleCommand)
		if err != nil {
			return err
		}

		netemCommand := "tc qdisc add dev " + netInterface + " parent 1:3 netem " + strings.ToLower(netemCmd)
		log.Debugf("Disrupt with netemCommand %s", netemCommand)
		err = client.execOnContainer(c, netemCommand)
		if err != nil {
			return err
		}

		filterCommand := "tc filter add dev " + netInterface + " protocol ip parent 1:0 prio 3 " +
			"u32 match ip dst " + strings.ToLower(targetIP) + " flowid 1:3"
		log.Debugf("Disrupt with filterCommand %s", filterCommand)
		return client.execOnContainer(c, filterCommand)
	}
	return nil
}

func (client dockerClient) execOnContainer(c Container, execCmd string) error {
	execConfig := &dockerclient.ExecConfig{
		Cmd:       strings.Split(execCmd, " "),
		Container: c.ID(),
	}
	_id, err := client.api.ExecCreate(execConfig)
	if err != nil {
		return err
	}

	log.Debugf("Starting Exec %s (%s)", execCmd, _id)
	return client.api.ExecStart(_id, execConfig)
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
