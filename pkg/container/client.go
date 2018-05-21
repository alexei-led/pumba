package container

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

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
	ListContainers(context.Context, Filter) ([]Container, error)
	ListAllContainers(context.Context, Filter) ([]Container, error)
	StopContainer(context.Context, Container, int, bool) error
	KillContainer(context.Context, Container, string, bool) error
	RemoveContainer(context.Context, Container, bool, bool, bool, bool) error
	NetemContainer(context.Context, Container, string, []string, []net.IP, time.Duration, string, bool) error
	StopNetemContainer(context.Context, Container, string, []net.IP, string, bool) error
	PauseContainer(context.Context, Container, bool) error
	UnpauseContainer(context.Context, Container, bool) error
	StartContainer(context.Context, Container, bool) error
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

func (client dockerClient) ListContainers(ctx context.Context, fn Filter) ([]Container, error) {
	return client.listContainers(ctx, fn, types.ContainerListOptions{})
}

func (client dockerClient) ListAllContainers(ctx context.Context, fn Filter) ([]Container, error) {
	return client.listContainers(ctx, fn, types.ContainerListOptions{All: true})
}

func (client dockerClient) listContainers(ctx context.Context, fn Filter, opts types.ContainerListOptions) ([]Container, error) {
	log.Debug("listing containers")
	containers, err := client.containerAPI.ContainerList(ctx, opts)
	if err != nil {
		log.WithError(err).Error("failed to list containers")
		return nil, err
	}
	cs := []Container{}
	for _, container := range containers {
		containerInfo, err := client.containerAPI.ContainerInspect(ctx, container.ID)
		if err != nil {
			log.WithError(err).Error("failed to inspect container")
			return nil, err
		}
		log.WithFields(log.Fields{
			"name": containerInfo.Name,
			"id":   containerInfo.ID,
		}).Debug("found container")

		imageInfo, _, err := client.imageAPI.ImageInspectWithRaw(ctx, containerInfo.Image)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"name":  containerInfo.Name,
				"id":    containerInfo.ID,
				"image": containerInfo.Image,
			}).Error("failed to inspect container image")
			return nil, err
		}
		c := Container{containerInfo: containerInfo, imageInfo: imageInfo}
		if fn(c) {
			cs = append(cs, c)
		}
	}
	return cs, nil
}

func (client dockerClient) KillContainer(ctx context.Context, c Container, signal string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"signal": signal,
		"dryrun": dryrun,
	}).Info("killing container")
	if !dryrun {
		return client.containerAPI.ContainerKill(ctx, c.ID(), signal)
	}
	return nil
}

func (client dockerClient) StopContainer(ctx context.Context, c Container, timeout int, dryrun bool) error {
	signal := c.StopSignal()
	if signal == "" {
		signal = defaultStopSignal
	}
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"timout": timeout,
		"signal": signal,
		"dryrun": dryrun,
	}).Info("stopping container")
	if !dryrun {
		if err := client.containerAPI.ContainerKill(ctx, c.ID(), signal); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"name":   c.Name(),
				"id":     c.ID(),
				"signal": signal,
			}).Warn("failed to kill container")
			return err
		}

		// Wait for container to exit, but proceed anyway after the timeout elapses
		if err := client.waitForStop(ctx, c, timeout); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"name":    c.Name(),
				"id":      c.ID(),
				"timeout": timeout,
			}).Warn("failed waiting for container to stop, going to kill it")

			// failed to stop gracefully - going to kill target container
			log.WithFields(log.Fields{
				"name":   c.Name(),
				"id":     c.ID(),
				"signal": defaultKillSignal,
			}).Debug("killing container")
			if err := client.containerAPI.ContainerKill(ctx, c.ID(), defaultKillSignal); err != nil {
				log.WithError(err).WithFields(log.Fields{
					"name":   c.Name(),
					"id":     c.ID(),
					"signal": defaultKillSignal,
				}).Error("failed to kill container")
				return err
			}
			// Wait for container to be removed
			if err := client.waitForStop(ctx, c, timeout); err != nil {
				log.WithError(err).WithFields(log.Fields{
					"name":    c.Name(),
					"id":      c.ID(),
					"timeout": timeout,
				}).Error("failed waiting for container to stop")
				return errors.New("failed waiting for container to stop")
			}
		}
	}
	return nil
}

func (client dockerClient) StartContainer(ctx context.Context, c Container, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"dryrun": dryrun,
	}).Info("starting container")
	if !dryrun {
		return client.containerAPI.ContainerStart(ctx, c.ID(), types.ContainerStartOptions{})
	}

	return nil
}

func (client dockerClient) RemoveContainer(ctx context.Context, c Container, force bool, links bool, volumes bool, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"force":   force,
		"links":   links,
		"volumes": volumes,
		"dryrun":  dryrun,
	}).Info("removing container")
	if !dryrun {
		removeOpts := types.ContainerRemoveOptions{
			RemoveVolumes: volumes,
			RemoveLinks:   links,
			Force:         force,
		}
		return client.containerAPI.ContainerRemove(ctx, c.ID(), removeOpts)
	}
	return nil
}

func (client dockerClient) NetemContainer(ctx context.Context, c Container, netInterface string, netemCmd []string, ips []net.IP, duration time.Duration, tcimage string, dryrun bool) error {
	prefix := ""
	if dryrun {
		prefix = dryRunPrefix
	}
	var err error
	if len(ips) == 0 {
		log.Infof("%sRunning netem command '%s' on container %s for %s", prefix, netemCmd, c.ID(), duration)
		err = client.startNetemContainer(ctx, c, netInterface, netemCmd, tcimage, dryrun)
	} else {
		log.Infof("%sRunning netem command '%s' on container %s for %s using filter %v", prefix, netemCmd, c.ID(), duration, ips)
		err = client.startNetemContainerIPFilter(ctx, c, netInterface, netemCmd, ips, tcimage, dryrun)
	}
	if err != nil {
		log.Error(err)
	}
	return err
}

func (client dockerClient) StopNetemContainer(ctx context.Context, c Container, netInterface string, ip []net.IP, tcimage string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":     c.Name(),
		"id":       c.ID(),
		"IPs":      ip,
		"iface":    netInterface,
		"tc-image": tcimage,
		"dryrun":   dryrun,
	}).Info("stopping netem on container")
	return client.stopNetemContainer(ctx, c, netInterface, ip, tcimage, dryrun)
}

func (client dockerClient) PauseContainer(ctx context.Context, c Container, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"dryrun": dryrun,
	}).Info("pausing container")
	if !dryrun {
		return client.containerAPI.ContainerPause(ctx, c.ID())
	}
	return nil
}

func (client dockerClient) UnpauseContainer(ctx context.Context, c Container, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"dryrun": dryrun,
	}).Info("stop pausing container")
	if !dryrun {
		return client.containerAPI.ContainerUnpause(ctx, c.ID())
	}
	return nil
}

func (client dockerClient) startNetemContainer(ctx context.Context, c Container, netInterface string, netemCmd []string, tcimage string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"iface":   netInterface,
		"netem":   strings.Join(netemCmd, " "),
		"tcimage": tcimage,
		"dryrun":  dryrun,
	}).Info("start netem for container")
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control:
		// 'tc qdisc add dev eth0 root netem delay 100ms'
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		netemCommand := append([]string{"qdisc", "add", "dev", netInterface, "root", "netem"}, netemCmd...)
		// stop disruption command
		// netemStopCommand := "tc qdisc del dev eth0 root netem"
		log.WithField("netem", strings.Join(netemCommand, " ")).Debug("adding netem qdisc")
		return client.tcCommand(ctx, c, netemCommand, tcimage)
	}
	return nil
}

func (client dockerClient) stopNetemContainer(ctx context.Context, c Container, netInterface string, ips []net.IP, tcimage string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"iface":   netInterface,
		"IPs":     ips,
		"tcimage": tcimage,
		"dryrun":  dryrun,
	}).Info("stop netem for container")
	if !dryrun {
		if len(ips) != 0 {
			// delete qdisc 'parent 1:1 handle 10:'
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand := []string{"qdisc", "del", "dev", netInterface, "parent", "1:1", "handle", "10:"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err := client.tcCommand(ctx, c, netemCommand, tcimage)
			if err != nil {
				log.WithError(err).Error("failed to execute tc command")
				return err
			}
			// delete qdisc 'parent 1:2 handle 20:'
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand = []string{"qdisc", "del", "dev", netInterface, "parent", "1:2", "handle", "20:"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err = client.tcCommand(ctx, c, netemCommand, tcimage)
			if err != nil {
				log.WithError(err).Error("failed to execute tc command")
				return err
			}
			// delete qdisc 'parent 1:3 handle 30:'
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand = []string{"qdisc", "del", "dev", netInterface, "parent", "1:3", "handle", "30:"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err = client.tcCommand(ctx, c, netemCommand, tcimage)
			if err != nil {
				log.WithError(err).Error("failed to execute tc command")
				return err
			}
			// delete qdisc 'root handle 1: prio'
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand = []string{"qdisc", "del", "dev", netInterface, "root", "handle", "1:", "prio"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err = client.tcCommand(ctx, c, netemCommand, tcimage)
			if err != nil {
				log.WithError(err).Error("failed to execute tc command")
				return err
			}
		} else {
			// stop netem command
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand := []string{"qdisc", "del", "dev", netInterface, "root", "netem"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err := client.tcCommand(ctx, c, netemCommand, tcimage)
			if err != nil {
				log.WithError(err).Error("failed to execute tc command")
				return err
			}
		}
	}
	return nil
}

func (client dockerClient) startNetemContainerIPFilter(ctx context.Context, c Container, netInterface string, netemCmd []string,
	ips []net.IP, tcimage string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"iface":   netInterface,
		"IPs":     ips,
		"tcimage": tcimage,
		"dryrun":  dryrun,
	}).Info("start netem for container with IP(s) filter")
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control
		// to filter network, needs to create a priority scheduling, add a low priority
		// queue, apply netem command on that queue only, then route IP traffic to the low priority queue
		// See more: http://www.linuxfoundation.org/collaborate/workgroups/networking/netem

		//            1:   root qdisc
		//           / | \
		//          /  |  \
		//         /   |   \
		//       1:1  1:2  1:3    classes
		//        |    |    |
		//       10:  20:  30:    qdiscs    qdiscs
		//      sfq  sfq  netem
		// band  0    1     2

		// Create a priority-based queue. This *instantly* creates classes 1:1, 1:2, 1:3
		// 'tc qdisc add dev <netInterface> root handle 1: prio'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		handleCommand := []string{"qdisc", "add", "dev", netInterface, "root", "handle", "1:", "prio"}
		log.WithField("netem", strings.Join(handleCommand, " ")).Debug("adding netem qdisc")
		err := client.tcCommand(ctx, c, handleCommand, tcimage)
		if err != nil {
			log.WithError(err).Error("failed to execute tc command")
			return err
		}

		// Create Stochastic Fairness Queueing (sfq) queueing discipline for 1:1 class.
		// 'tc qdisc add dev <netInterface> parent 1:1 handle 10: sfq'
		// See more: https://linux.die.net/man/8/tc-sfq
		netemCommand := []string{"qdisc", "add", "dev", netInterface, "parent", "1:1", "handle", "10:", "sfq"}
		log.WithField("netem", strings.Join(netemCommand, " ")).Debug("adding netem qdisc")
		err = client.tcCommand(ctx, c, netemCommand, tcimage)
		if err != nil {
			log.WithError(err).Error("failed to execute tc command")
			return err
		}

		// Create Stochastic Fairness Queueing (sfq) queueing discipline for 1:2 class
		// 'tc qdisc add dev <netInterface> parent 1:2 handle 20: sfq'
		// See more: https://linux.die.net/man/8/tc-sfq
		netemCommand = []string{"qdisc", "add", "dev", netInterface, "parent", "1:2", "handle", "20:", "sfq"}
		log.WithField("netem", strings.Join(netemCommand, " ")).Debug("adding netem qdisc")
		err = client.tcCommand(ctx, c, netemCommand, tcimage)
		if err != nil {
			log.WithError(err).Error("failed to execute tc command")
			return err
		}

		// Add queueing discipline for 1:3 class. No traffic is going through 1:3 yet
		// 'tc qdisc add dev <netInterface> parent 1:3 handle 30: netem <netemCmd>'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		netemCommand = append([]string{"qdisc", "add", "dev", netInterface, "parent", "1:3", "handle", "30:", "netem"}, netemCmd...)
		log.WithField("netem", strings.Join(netemCommand, " ")).Debug("adding netem qdisc")
		err = client.tcCommand(ctx, c, netemCommand, tcimage)
		if err != nil {
			log.WithError(err).Error("failed to execute tc command")
			return err
		}

		// # redirect traffic to specific IP through band 3
		// 'tc filter add dev <netInterface> protocol ip parent 1:0 prio 1 u32 match ip dst <targetIP> flowid 1:3'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		for _, ip := range ips {
			filterCommand := []string{"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "dst", ip.String(), "flowid", "1:3"}
			log.WithField("netem", strings.Join(filterCommand, " ")).Debug("adding netem filter")
			err = client.tcCommand(ctx, c, filterCommand, tcimage)
			if err != nil {
				log.WithError(err).Error("failed to execute tc command")
				return err
			}
		}
	}
	return nil
}

func (client dockerClient) tcCommand(ctx context.Context, c Container, args []string, tcimage string) error {
	if tcimage == "" {
		return client.execOnContainer(ctx, c, "tc", args, true)
	}
	return client.tcContainerCommand(ctx, c, args, tcimage)
}

// execute tc command using other container (with iproute2 package installed), using target container network stack
// try to use `gaiadocker\iproute2` image (Alpine + iproute2 package)
func (client dockerClient) tcContainerCommand(ctx context.Context, target Container, args []string, tcimage string) error {
	log.WithFields(log.Fields{
		"tcimage": tcimage,
		"tc args": args,
	}).Debug("executing tc command")
	// container config
	config := ctypes.Config{
		Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Entrypoint: []string{"tc"},
		Cmd:        args,
		Image:      tcimage,
	}
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
	log.WithField("network", hconfig.NetworkMode).Debug("network mode")
	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, "")
	if err != nil {
		log.WithError(err).Error("failed to create tc container")
		return err
	}
	log.WithField("id", createResponse.ID).Debug("tc container created, starting it")
	err = client.containerAPI.ContainerStart(ctx, createResponse.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithError(err).Error("failed to start tc container")
		return err
	}
	return nil
}

func (client dockerClient) execOnContainer(ctx context.Context, c Container, execCmd string, execArgs []string, privileged bool) error {
	log.WithFields(log.Fields{
		"id":         c.ID(),
		"name":       c.Name(),
		"command":    execCmd,
		"args":       execArgs,
		"privileged": privileged,
	}).Debug("executing command in container")
	// trim all spaces from cmd
	execCmd = strings.Replace(execCmd, " ", "", -1)

	// check if command exists inside target container
	checkExists := types.ExecConfig{
		Cmd: []string{"which", execCmd},
	}
	exec, err := client.containerAPI.ContainerExecCreate(ctx, c.ID(), checkExists)
	if err != nil {
		log.WithError(err).Error("failed to create exec configuration to check if command exists")
		return err
	}
	log.WithField("command", execCmd).Debugf("checking if command exists")
	err = client.containerAPI.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})
	if err != nil {
		log.WithError(err).Error("failed to check if command exists in a container")
		return err
	}
	checkInspect, err := client.containerAPI.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		log.WithError(err).Error("failed to inspect check execution")
		return err
	}
	if checkInspect.ExitCode != 0 {
		log.Error("command does not exist inside the container")
		return fmt.Errorf("command '%s' not found inside the %s (%s) container", execCmd, c.Name(), c.ID())
	}

	// if command found execute it
	log.WithField("command", execCmd).Debug("command found: continue execution")

	// prepare exec config
	config := types.ExecConfig{
		Privileged: privileged,
		Cmd:        append([]string{execCmd}, execArgs...),
	}
	// execute the command
	exec, err = client.containerAPI.ContainerExecCreate(ctx, c.ID(), config)
	if err != nil {
		log.WithError(err).Error("failed to create exec configuration for a command")
		return err
	}
	log.Debugf("Starting Exec %s %s (%s)", execCmd, execArgs, exec.ID)
	err = client.containerAPI.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})
	if err != nil {
		log.WithError(err).Error("failed to start command execution")
		return err
	}
	exitInspect, err := client.containerAPI.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		log.WithError(err).Error("failed to inspect command execution")
		return err
	}
	if exitInspect.ExitCode != 0 {
		log.WithField("exit", exitInspect.ExitCode).Error("command exited with error")
		return fmt.Errorf("command '%s' failed in %s (%s) container; run it in manually to debug", execCmd, c.Name(), c.ID())
	}
	return nil
}

func (client dockerClient) waitForStop(ctx context.Context, c Container, waitTime int) error {
	timeout := time.After(time.Duration(waitTime) * time.Second)
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"timeout": timeout,
	}).Debug("waiting for container to stop")
	for {
		select {
		case <-timeout:
			log.WithFields(log.Fields{
				"name": c.Name(),
				"id":   c.ID(),
			}).Warn("timout waiting to stop")
			return errors.New("timeout waiting to stop")
		case <-ctx.Done():
			log.WithFields(log.Fields{
				"name": c.Name(),
				"id":   c.ID(),
			}).Warn("waiting aborted")
			return errors.New("aborted waiting to stop")
		default:
			if ci, err := client.containerAPI.ContainerInspect(ctx, c.ID()); err != nil {
				log.WithError(err).Error("failed to inspect container, while waiting to stop")
				return err
			} else if !ci.State.Running {
				return nil
			}
		}
		// check status every 100 ms
		time.Sleep(100 * time.Microsecond)
	}
}
