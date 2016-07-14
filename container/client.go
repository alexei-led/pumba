package container

import (
	"crypto/tls"
	"fmt"
	"time"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/samalba/dockerclient"
)

const (
	defaultStopSignal = "SIGTERM"
	dryRunPrefix      = "DRY: "
)

// A Filter is a prototype for a function that can be used to filter the
// results from a call to the ListContainers() method on the Client.
type Filter func(Container) bool

// A Client is the interface through which Pumba interacts with the Docker API.
type Client interface {
	ListContainers(Filter) ([]Container, error)
	StopContainer(Container, time.Duration, bool) error
	KillContainer(Container, string, bool) error
	StartContainer(Container) error
	RenameContainer(Container, string) error
	RemoveImage(Container, bool, bool) error
	RemoveContainer(Container, bool, bool) error
	DisruptContainer(Container, string, bool) error
}

// NewClient returns a new Client instance which can be used to interact with
// the Docker API.
func NewClient(dockerHost string, tlsConfig *tls.Config, pullImages bool) Client {
	docker, err := dockerclient.NewDockerClient(dockerHost, tlsConfig)

	if err != nil {
		log.Fatalf("Error instantiating Docker client: %s", err)
	}

	return dockerClient{api: docker, pullImages: pullImages}
}

type dockerClient struct {
	api        dockerclient.Client
	pullImages bool
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

func (client dockerClient) StopContainer(c Container, timeout time.Duration, dryrun bool) error {
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

		log.Debugf("Removing container %s", c.ID())

		if err := client.api.RemoveContainer(c.ID(), true, false); err != nil {
			return err
		}

		// Wait for container to be removed. In this case an error is a good thing
		if err := client.waitForStop(c, timeout); err == nil {
			return fmt.Errorf("Container %s (%s) could not be removed", c.Name(), c.ID())
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

func (client dockerClient) RemoveContainer(c Container, force bool, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sRemoving container %s", prefix, c.ID())
	if !dryrun {
		return client.api.RemoveContainer(c.ID(), force, true)
	}
	return nil
}

func (client dockerClient) DisruptContainer(c Container, netemCmd string, dryrun bool) error {
	// find out if we have command, ip or command:ip
	cmd = strings.Split(netemCmd, ":")
	if len(cmd) == 2 {
		return disruptContainerFilterNetwork(c, cmd[0], cmd[1], dryrun)
	}
	else {// all network
		return disruptContainerAllNetwork(c, cmd[0], dryrun)
	}
}

func (client dockerClient) disruptContainerAllNetwork(c Container, netemCmd string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	log.Infof("%sDisrupting container %s with netem cmd %s", prefix, c.ID(), netemCmd)
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control:
		// 'tc qdisc add dev eth0 root netem delay 100ms'
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		netemCommand := "tc qdisc add dev eth0 root netem " + strings.ToLower(netemCmd)
		return execOnContainer(c, netemCommand)
	}
	return nil
}

func (client dockerClient) disruptContainerFilterNetwork(c Container, netemCmd string, 
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
		handleCommand := "tc qdisc add dev eth0 root handle 1: prio"
		err_handle := execOnContainer(c, handleCommand)
		if err_handle != nil {
				return err_handle
			}

		netemCommand := "tc qdisc add dev eth0 parent 1:3 netem " + strings.ToLower(netemCmd)
		err_netem := execOnContainer(c, netemCommand)
		if err_netem != nil {
				return err_netem
			}

		filterCommand := "tc filter add dev eth0 protocol ip parent 1:0 prio 3 "+
			"u32 match ip dst " + strings.ToLower(targetIP) + " flowid 1:3"
		return execOnContainer(c, filterCommand)
	}
	return nil
}

func (client dockerClient) execOnContainer(c Container, execCmd string) error {
	execConfig := &dockerclient.ExecConfig{
		Cmd: strings.Split(execCmd, " "),
		Container: c.ID(),
	}
	_id, err := client.api.ExecCreate(execConfig)
	if err != nil {
			return err
		}

	log.Debugf("Starting Exec %s (%s)", execCmd, _id)
	return client.api.ExecStart(_id, execConfig)
}

func (client dockerClient) waitForStop(c Container, waitTime time.Duration) error {
	timeout := time.After(waitTime)

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
