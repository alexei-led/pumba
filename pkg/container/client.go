package container

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	dockerapi "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	defaultStopSignal = "SIGTERM"
	defaultKillSignal = "SIGKILL"
)

// A FilterFunc is a prototype for a function that can be used to filter the
// results from a call to the ListContainers() method on the Client.
type FilterFunc func(*Container) bool

// Client interface
type Client interface {
	ListContainers(context.Context, FilterFunc, ListOpts) ([]*Container, error)
	StopContainer(context.Context, *Container, int, bool) error
	KillContainer(context.Context, *Container, string, bool) error
	ExecContainer(context.Context, *Container, string, bool) error
	RestartContainer(context.Context, *Container, time.Duration, bool) error
	RemoveContainer(context.Context, *Container, bool, bool, bool, bool) error
	NetemContainer(context.Context, *Container, string, []string, []*net.IPNet, []string, []string, time.Duration, string, bool, bool) error
	StopNetemContainer(context.Context, *Container, string, []*net.IPNet, []string, []string, string, bool, bool) error
	PauseContainer(context.Context, *Container, bool) error
	UnpauseContainer(context.Context, *Container, bool) error
	StartContainer(context.Context, *Container, bool) error
	StressContainer(context.Context, *Container, []string, string, bool, time.Duration, bool) (string, <-chan string, <-chan error, error)
	StopContainerWithID(context.Context, string, time.Duration, bool) error
}

type imagePullResponse struct {
	Status         string `json:"status"`
	Error          string `json:"error"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"progressDetail"`
}

// NewClient returns a new Client instance which can be used to interact with the Docker API.
func NewClient(dockerHost string, tlsConfig *tls.Config) (Client, error) {
	httpClient, err := HTTPClient(dockerHost, tlsConfig)
	if err != nil {
		return nil, err
	}

	apiClient, err := dockerapi.NewClientWithOpts(dockerapi.WithHost(dockerHost), dockerapi.WithHTTPClient(httpClient), dockerapi.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create docker client")
	}

	return dockerClient{containerAPI: apiClient, imageAPI: apiClient}, nil
}

type dockerClient struct {
	containerAPI dockerapi.ContainerAPIClient
	imageAPI     dockerapi.ImageAPIClient
}

// ListContainers returns a list of containers that match the given filter
func (client dockerClient) ListContainers(ctx context.Context, fn FilterFunc, opts ListOpts) ([]*Container, error) {
	filterArgs := filters.NewArgs()
	for _, label := range opts.Labels {
		filterArgs.Add("label", label)
	}
	return client.listContainers(ctx, fn, types.ContainerListOptions{All: opts.All, Filters: filterArgs})
}

func (client dockerClient) listContainers(ctx context.Context, fn FilterFunc, opts types.ContainerListOptions) ([]*Container, error) {
	log.Debug("listing containers")
	containers, err := client.containerAPI.ContainerList(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list containers")
	}
	var cs []*Container
	for _, container := range containers {
		containerInfo, err := client.containerAPI.ContainerInspect(ctx, container.ID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to inspect container")
		}
		log.WithFields(log.Fields{
			"name": containerInfo.Name,
			"id":   containerInfo.ID,
		}).Debug("found container")

		imageInfo, _, err := client.imageAPI.ImageInspectWithRaw(ctx, containerInfo.Image)
		if err != nil {
			return nil, errors.Wrap(err, "failed to inspect container image")
		}
		c := &Container{ContainerInfo: containerInfo, ImageInfo: imageInfo}
		if fn(c) {
			cs = append(cs, c)
		}
	}
	return cs, nil
}

// KillContainer kills a container with the given signal
func (client dockerClient) KillContainer(ctx context.Context, c *Container, signal string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"signal": signal,
		"dryrun": dryrun,
	}).Info("killing container")
	if !dryrun {
		err := client.containerAPI.ContainerKill(ctx, c.ID(), signal)
		if err != nil {
			return errors.Wrap(err, "failed to kill container")
		}
	}
	return nil
}

// ExecContainer executes a command in a container
func (client dockerClient) ExecContainer(ctx context.Context, c *Container, command string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"command": command,
		"dryrun":  dryrun,
	}).Info("exec container")
	if !dryrun {
		createRes, err := client.containerAPI.ContainerExecCreate(
			ctx, c.ID(), types.ExecConfig{
				User:         "root",
				AttachStdout: true,
				AttachStderr: true,
				Cmd:          strings.Split(command, " "),
			},
		)
		if err != nil {
			return errors.Wrap(err, "exec create failed")
		}

		attachRes, err := client.containerAPI.ContainerAttach(
			ctx, createRes.ID, types.ContainerAttachOptions{},
		)
		if err != nil {
			return errors.Wrap(err, "exec attach failed")
		}

		if err = client.containerAPI.ContainerExecStart(
			ctx, createRes.ID, types.ExecStartCheck{},
		); err != nil {
			return errors.Wrap(err, "exec start failed")
		}

		output, err := io.ReadAll(attachRes.Reader)
		if err != nil {
			return errors.Wrap(err, "reading output from exec reader failed")
		}
		log.WithFields(log.Fields{
			"name":    c.Name(),
			"id":      c.ID(),
			"command": command,
			"dryrun":  dryrun,
		}).Info(string(output))

		res, err := client.containerAPI.ContainerExecInspect(ctx, createRes.ID)
		if err != nil {
			return errors.Wrap(err, "exec inspect failed")
		}
		if res.ExitCode != 0 {
			return errors.New("exec failed " + command + fmt.Sprintf(" %d", res.ExitCode))
		}
	}
	return nil
}

// RestartContainer restarts a container
func (client dockerClient) RestartContainer(ctx context.Context, c *Container, timeout time.Duration, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"timeout": timeout,
		"dryrun":  dryrun,
	}).Info("restart container")
	if !dryrun {
		if err := client.containerAPI.ContainerRestart(ctx, c.ID(), &timeout); err != nil {
			return errors.Wrap(err, "failed to restart container")
		}
	}
	return nil
}

// StopContainer stops a container
func (client dockerClient) StopContainer(ctx context.Context, c *Container, timeout int, dryrun bool) error {
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
			return errors.Wrap(err, "failed to kill container")
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
				return errors.Wrap(err, "failed to kill container")
			}
			// Wait for container to be removed
			if err := client.waitForStop(ctx, c, timeout); err != nil {
				return errors.New("failed waiting for container to stop")
			}
		}
	}
	return nil
}

// StopContainerWithID stops a container with a timeout
func (client dockerClient) StopContainerWithID(ctx context.Context, containerID string, timeout time.Duration, dryrun bool) error {
	log.WithFields(log.Fields{
		"id":      containerID,
		"timeout": timeout,
		"dryrun":  dryrun,
	}).Info("stopping container")
	if !dryrun {
		err := client.containerAPI.ContainerStop(ctx, containerID, &timeout)
		if err != nil {
			return errors.Wrap(err, "failed to stop container")
		}
	}
	return nil
}

// StartContainer starts a container
func (client dockerClient) StartContainer(ctx context.Context, c *Container, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"dryrun": dryrun,
	}).Info("starting container")
	if !dryrun {
		err := client.containerAPI.ContainerStart(ctx, c.ID(), types.ContainerStartOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to start container")
		}
	}

	return nil
}

// RemoveContainer removes a container
func (client dockerClient) RemoveContainer(ctx context.Context, c *Container, force, links, volumes, dryrun bool) error {
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
		err := client.containerAPI.ContainerRemove(ctx, c.ID(), removeOpts)
		if err != nil {
			return errors.Wrap(err, "failed to remove container")
		}
	}
	return nil
}

// NetemContainer injects sidecar netem container into the given container network namespace
func (client dockerClient) NetemContainer(ctx context.Context, c *Container, netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string, duration time.Duration, tcimage string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":     c.Name(),
		"id":       c.ID(),
		"command":  netemCmd,
		"ips":      ips,
		"sports":   sports,
		"dports":   dports,
		"duration": duration,
		"tc-image": tcimage,
		"pull":     pull,
		"dryrun":   dryrun,
	}).Info("running netem on container")
	if len(ips) == 0 && len(sports) == 0 && len(dports) == 0 {
		return client.startNetemContainer(ctx, c, netInterface, netemCmd, tcimage, pull, dryrun)
	}
	return client.startNetemContainerIPFilter(ctx, c, netInterface, netemCmd, ips, sports, dports, tcimage, pull, dryrun)
}

// StopNetemContainer stops the netem container injected into the given container network namespace
func (client dockerClient) StopNetemContainer(ctx context.Context, c *Container, netInterface string, ip []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":     c.Name(),
		"id":       c.ID(),
		"IPs":      ip,
		"sports":   sports,
		"dports":   dports,
		"iface":    netInterface,
		"tc-image": tcimage,
		"pull":     pull,
		"dryrun":   dryrun,
	}).Info("stopping netem on container")
	return client.stopNetemContainer(ctx, c, netInterface, ip, sports, dports, tcimage, pull, dryrun)
}

// PauseContainer pauses a container main process
func (client dockerClient) PauseContainer(ctx context.Context, c *Container, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"dryrun": dryrun,
	}).Info("pausing container")
	if !dryrun {
		err := client.containerAPI.ContainerPause(ctx, c.ID())
		if err != nil {
			return errors.Wrap(err, "failed to pause container")
		}
	}
	return nil
}

// UnpauseContainer unpauses a container main process
func (client dockerClient) UnpauseContainer(ctx context.Context, c *Container, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"dryrun": dryrun,
	}).Info("stop pausing container")
	if !dryrun {
		err := client.containerAPI.ContainerUnpause(ctx, c.ID())
		if err != nil {
			return errors.Wrap(err, "failed to unpause container")
		}
	}
	return nil
}

// StressContainer starts stress test on a container (CPU, memory, network, io)
func (client dockerClient) StressContainer(ctx context.Context, c *Container, stressors []string, image string, pull bool, duration time.Duration, dryrun bool) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"name":      c.Name(),
		"id":        c.ID(),
		"stressors": stressors,
		"image":     image,
		"pull":      pull,
		"duration":  duration,
		"dryrun":    dryrun,
	}).Info("stress testing container")
	if !dryrun {
		return client.stressContainerCommand(ctx, c.ID(), stressors, image, pull)
	}
	return "", nil, nil, nil
}

func (client dockerClient) startNetemContainer(ctx context.Context, c *Container, netInterface string, netemCmd []string, tcimage string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"iface":   netInterface,
		"netem":   strings.Join(netemCmd, " "),
		"tcimage": tcimage,
		"pull":    pull,
		"dryrun":  dryrun,
	}).Debug("start netem for container")
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control:
		// 'tc qdisc add dev eth0 root netem delay 100ms'
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		netemCommand := append([]string{"qdisc", "add", "dev", netInterface, "root", "netem"}, netemCmd...)
		// stop disruption command
		// netemStopCommand := "tc qdisc del dev eth0 root netem"
		log.WithField("netem", strings.Join(netemCommand, " ")).Debug("adding netem qdisc")
		return client.tcCommand(ctx, c, netemCommand, tcimage, pull)
	}
	return nil
}

func (client dockerClient) stopNetemContainer(ctx context.Context, c *Container, netInterface string, ips []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"iface":   netInterface,
		"IPs":     ips,
		"tcimage": tcimage,
		"pull":    pull,
		"dryrun":  dryrun,
	}).Debug("stop netem for container")
	if !dryrun {
		if len(ips) != 0 || len(sports) != 0 || len(dports) != 0 {
			// delete qdisc 'parent 1:1 handle 10:'
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand := []string{"qdisc", "del", "dev", netInterface, "parent", "1:1", "handle", "10:"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err := client.tcCommand(ctx, c, netemCommand, tcimage, pull)
			if err != nil {
				return errors.Wrap(err, "failed to delete qdisc 'parent 1:1 handle 10:'")
			}
			// delete qdisc 'parent 1:2 handle 20:'
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand = []string{"qdisc", "del", "dev", netInterface, "parent", "1:2", "handle", "20:"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err = client.tcCommand(ctx, c, netemCommand, tcimage, pull)
			if err != nil {
				return errors.Wrap(err, "failed to delete qdisc 'parent 1:2 handle 20:'")
			}
			// delete qdisc 'parent 1:3 handle 30:'
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand = []string{"qdisc", "del", "dev", netInterface, "parent", "1:3", "handle", "30:"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err = client.tcCommand(ctx, c, netemCommand, tcimage, pull)
			if err != nil {
				return errors.Wrap(err, "failed to delete qdisc 'parent 1:3 handle 30:'")
			}
			// delete qdisc 'root handle 1: prio'
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand = []string{"qdisc", "del", "dev", netInterface, "root", "handle", "1:", "prio"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err = client.tcCommand(ctx, c, netemCommand, tcimage, pull)
			if err != nil {
				return errors.Wrap(err, "failed to delete qdisc 'root handle 1: prio'")
			}
		} else {
			// stop netem command
			// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
			netemCommand := []string{"qdisc", "del", "dev", netInterface, "root", "netem"}
			log.WithField("netem", strings.Join(netemCommand, " ")).Debug("deleting netem qdisc")
			err := client.tcCommand(ctx, c, netemCommand, tcimage, pull)
			if err != nil {
				return errors.Wrap(err, "failed to stop netem")
			}
		}
	}
	return nil
}

//nolint:funlen
func (client dockerClient) startNetemContainerIPFilter(ctx context.Context, c *Container, netInterface string, netemCmd []string,
	ips []*net.IPNet, sports []string, dports []string, tcimage string, pull bool, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"iface":   netInterface,
		"IPs":     ips,
		"Sports":  sports,
		"Dports":  dports,
		"tcimage": tcimage,
		"pull":    pull,
		"dryrun":  dryrun,
	}).Debug("start netem for container with IP(s) filter")
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
		err := client.tcCommand(ctx, c, handleCommand, tcimage, pull)
		if err != nil {
			return errors.Wrap(err, "failed to create a priority-based queue")
		}

		// Create Stochastic Fairness Queueing (sfq) queueing discipline for 1:1 class.
		// 'tc qdisc add dev <netInterface> parent 1:1 handle 10: sfq'
		// See more: https://linux.die.net/man/8/tc-sfq
		netemCommand := []string{"qdisc", "add", "dev", netInterface, "parent", "1:1", "handle", "10:", "sfq"}
		log.WithField("netem", strings.Join(netemCommand, " ")).Debug("adding netem qdisc")
		err = client.tcCommand(ctx, c, netemCommand, tcimage, pull)
		if err != nil {
			return errors.Wrap(err, "failed to create Stochastic Fairness Queueing (sfq) queueing discipline for 1:1 class")
		}

		// Create Stochastic Fairness Queueing (sfq) queueing discipline for 1:2 class
		// 'tc qdisc add dev <netInterface> parent 1:2 handle 20: sfq'
		// See more: https://linux.die.net/man/8/tc-sfq
		netemCommand = []string{"qdisc", "add", "dev", netInterface, "parent", "1:2", "handle", "20:", "sfq"}
		log.WithField("netem", strings.Join(netemCommand, " ")).Debug("adding netem qdisc")
		err = client.tcCommand(ctx, c, netemCommand, tcimage, pull)
		if err != nil {
			return errors.Wrap(err, "failed to create Stochastic Fairness Queueing (sfq) queueing discipline for 1:2 class")
		}

		// Add queueing discipline for 1:3 class. No traffic is going through 1:3 yet
		// 'tc qdisc add dev <netInterface> parent 1:3 handle 30: netem <netemCmd>'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		netemCommand = append([]string{"qdisc", "add", "dev", netInterface, "parent", "1:3", "handle", "30:", "netem"}, netemCmd...)
		log.WithField("netem", strings.Join(netemCommand, " ")).Debug("adding netem qdisc")
		err = client.tcCommand(ctx, c, netemCommand, tcimage, pull)
		if err != nil {
			return errors.Wrap(err, "failed to add queueing discipline for 1:3 class")
		}

		// # redirect traffic to specific IP through band 3
		// 'tc filter add dev <netInterface> protocol ip parent 1:0 prio 1 u32 match ip dst <targetIP> flowid 1:3'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		for _, ip := range ips {
			filterCommand := []string{"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "dst", ip.String(), "flowid", "1:3"}
			log.WithField("netem", strings.Join(filterCommand, " ")).Debug("adding netem IP filter")
			err = client.tcCommand(ctx, c, filterCommand, tcimage, pull)
			if err != nil {
				return errors.Wrap(err, "failed to redirect traffic to specific IP through band 3")
			}
		}
		// # redirect traffic to specific sport through band 3
		// 'tc filter add dev <netInterface> protocol ip parent 1:0 prio 1 u32 match ip <s/d>port <targetPort> 0xffff flowid 1:3'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		for _, sport := range sports {
			filterPortCommand := []string{"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "sport", sport, "0xffff", "flowid", "1:3"}
			log.WithField("netem", strings.Join(filterPortCommand, " ")).Debug("adding netem port filter")
			err = client.tcCommand(ctx, c, filterPortCommand, tcimage, pull)
			if err != nil {
				return errors.Wrap(err, "failed to redirect traffic from port "+sport+" through band 3")
			}
		}
		// # redirect traffic to specific dport through band 3
		// 'tc filter add dev <netInterface> protocol ip parent 1:0 prio 1 u32 match ip <s/d>port <targetPort> 0xffff flowid 1:3'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		for _, dport := range dports {
			filterPortCommand := []string{"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "dport", dport, "0xffff", "flowid", "1:3"}
			log.WithField("netem", strings.Join(filterPortCommand, " ")).Debug("adding netem port filter")
			err = client.tcCommand(ctx, c, filterPortCommand, tcimage, pull)
			if err != nil {
				return errors.Wrap(err, "failed to redirect traffic to port "+dport+" through band 3")
			}
		}
	}
	return nil
}

func (client dockerClient) tcCommand(ctx context.Context, c *Container, args []string, tcimage string, pull bool) error {
	if tcimage == "" {
		return client.execOnContainer(ctx, c, "tc", args, true)
	}
	return client.tcContainerCommand(ctx, c, args, tcimage, pull)
}

// execute tc command using other container (with iproute2 package installed), using target container network stack
// try to use `gaiadocker\iproute2` image (Alpine + iproute2 package)
func (client dockerClient) tcContainerCommand(ctx context.Context, target *Container, args []string, tcimage string, pull bool) error {
	log.WithFields(log.Fields{
		"container": target.ID(),
		"tc-image":  tcimage,
		"pull":      pull,
		"args":      args,
	}).Debug("executing tc command in a separate container joining target container network namespace")
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
	// pull docker image if required: can pull only public images
	if pull {
		log.WithField("image", config.Image).Debug("pulling tc-image")
		events, err := client.imageAPI.ImagePull(ctx, config.Image, types.ImagePullOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to pull tc-image")
		}
		defer events.Close()
		d := json.NewDecoder(events)
		var pullResponse *imagePullResponse
		for {
			if err = d.Decode(&pullResponse); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return errors.Wrap(err, "failed to decode docker pull response for tc-image")
			}
			log.Debug(pullResponse)
		}
	}
	log.WithField("image", config.Image).Debug("creating tc-container")
	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")
	if err != nil {
		return errors.Wrap(err, "failed to create tc-container from tc-image")
	}
	log.WithField("id", createResponse.ID).Debug("tc container created, starting it")
	err = client.containerAPI.ContainerStart(ctx, createResponse.ID, types.ContainerStartOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to start tc-container")
	}
	return nil
}

//nolint:funlen
// execute a stress-ng command in stress-ng Docker container in target container cgroup
func (client dockerClient) stressContainerCommand(ctx context.Context, targetID string, stressors []string, image string, pull bool) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"target":    targetID,
		"stressors": stressors,
		"image":     image,
		"pull":      pull,
	}).Debug("executing stress-ng command")
	dockhackArgs := append([]string{targetID, "stress-ng"}, stressors...)
	// container config
	config := ctypes.Config{
		Labels: map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Image:  image,
		// dockhack script is required as entrypoint https://github.com/tavisrudd/dockhack
		Entrypoint: []string{"dockhack", "cg_exec"},
		// target container ID plus stress-ng command and it's arguments
		Cmd: dockhackArgs,
	}
	// docker socket mount
	dockerSocket := mount.Mount{
		Type:   mount.TypeBind,
		Source: "/var/run/docker.sock",
		Target: "/var/run/docker.sock",
	}
	fsCgroup := mount.Mount{
		Type:   mount.TypeBind,
		Source: "/sys/fs/cgroup",
		Target: "/sys/fs/cgroup",
	}
	// host config
	hconfig := ctypes.HostConfig{
		// auto remove container on command exit
		AutoRemove: true,
		// SYS_ADMIN is required for "dockhack" script to access cgroup
		CapAdd: []string{"SYS_ADMIN"},
		// apparmor:unconfined required to work with cgroup from container
		SecurityOpt: []string{"apparmor:unconfined"},
		// mount docker.sock and host cgroups fs as bind mount (equal to --volume /path:/path flag in docker run)
		Mounts: []mount.Mount{dockerSocket, fsCgroup},
	}
	// make output and error channel
	output := make(chan string)
	outerr := make(chan error)
	// pull docker image if required: can pull only public images
	if pull {
		log.WithField("image", config.Image).Debug("pulling stress-ng image")
		events, err := client.imageAPI.ImagePull(ctx, config.Image, types.ImagePullOptions{})
		if err != nil {
			close(outerr)
			close(output)
			return "", output, outerr, errors.Wrap(err, "failed to pull stress-ng image")
		}
		defer events.Close()
		d := json.NewDecoder(events)
		var pullResponse *imagePullResponse
		for {
			if err = d.Decode(&pullResponse); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				close(outerr)
				close(output)
				return "", output, outerr, errors.Wrap(err, "failed to decode docker pull result")
			}
			log.Debug(pullResponse)
		}
	}
	// create stress-ng container
	log.WithField("image", config.Image).Debug("creating stress-ng container")
	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")
	if err != nil {
		close(outerr)
		close(output)
		return "", output, outerr, errors.Wrap(err, "failed to create stress-ng container")
	}
	// attach to stress-ng container, capturing stdout and stderr
	opts := types.ContainerAttachOptions{
		Stdin:  false,
		Stdout: true,
		Stderr: true,
		Stream: true,
	}
	attach, err := client.containerAPI.ContainerAttach(ctx, createResponse.ID, opts)
	if err != nil {
		close(outerr)
		close(output)
		return "", output, outerr, errors.Wrap(err, "failed to attach to stress-ng container")
	}
	// copy stderr and stdout from attached reader
	go func() {
		defer close(output)
		defer close(outerr)
		defer attach.Close()
		var stdout bytes.Buffer
		_, e := io.Copy(&stdout, attach.Reader)
		if e != nil {
			outerr <- e
			return
		}
		// inspect stress-ng container
		inspect, e := client.containerAPI.ContainerInspect(ctx, createResponse.ID)
		if e != nil {
			outerr <- errors.Wrap(e, "failed to inspect stress-ng container")
			return
		}
		// get status of stress-ng command
		if inspect.State.ExitCode != 0 {
			outerr <- errors.Errorf("stress-ng exited with error: %v", stdout.String())
			return
		}
		output <- stdout.String()
	}()
	// start stress-ng container running stress-ng in target container cgroup
	log.WithField("id", createResponse.ID).Debug("stress-ng container created, starting it")
	err = client.containerAPI.ContainerStart(ctx, createResponse.ID, types.ContainerStartOptions{})
	if err != nil {
		return createResponse.ID, output, outerr, errors.Wrap(err, "failed to start stress-ng container")
	}
	return createResponse.ID, output, outerr, nil
}

// execute command on container
func (client dockerClient) execOnContainer(ctx context.Context, c *Container, execCmd string, execArgs []string, privileged bool) error {
	log.WithFields(log.Fields{
		"id":         c.ID(),
		"name":       c.Name(),
		"command":    execCmd,
		"args":       execArgs,
		"privileged": privileged,
	}).Debug("executing command in container")
	// trim all spaces from cmd
	execCmd = strings.ReplaceAll(execCmd, " ", "")

	// check if command exists inside target container
	checkExists := types.ExecConfig{
		Cmd: []string{"which", execCmd},
	}
	exec, err := client.containerAPI.ContainerExecCreate(ctx, c.ID(), checkExists)
	if err != nil {
		return errors.Wrap(err, "failed to create exec configuration to check if command exists")
	}
	log.WithField("command", execCmd).Debugf("checking if command exists")
	err = client.containerAPI.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})
	if err != nil {
		return errors.Wrap(err, "failed to check if command exists in a container")
	}
	checkInspect, err := client.containerAPI.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return errors.Wrap(err, "failed to inspect check execution")
	}
	if checkInspect.ExitCode != 0 {
		return errors.Errorf("command '%s' not found inside the %s container", execCmd, c.ID())
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
		return errors.Wrap(err, "failed to create exec configuration for a command")
	}
	log.Debugf("starting exec %s %s (%s)", execCmd, execArgs, exec.ID)
	err = client.containerAPI.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})
	if err != nil {
		return errors.Wrap(err, "failed to start command execution")
	}
	exitInspect, err := client.containerAPI.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return errors.Wrap(err, "failed to inspect command execution")
	}
	if exitInspect.ExitCode != 0 {
		return errors.Errorf("command '%s' failed in %s container; run it in manually to debug", execCmd, c.ID())
	}
	return nil
}

func (client dockerClient) waitForStop(ctx context.Context, c *Container, waitTime int) error {
	// check status every 100 ms
	const checkInterval = 100 * time.Microsecond
	// timeout after waitTime seconds
	timeout := time.After(time.Duration(waitTime) * time.Second)
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"timeout": timeout,
	}).Debug("waiting for container to stop")
	for {
		select {
		case <-timeout:
			return errors.New("timeout on waiting to stop")
		case <-ctx.Done():
			return errors.New("aborted waiting to stop")
		default:
			if ci, err := client.containerAPI.ContainerInspect(ctx, c.ID()); err != nil {
				return errors.Wrap(err, "failed to inspect container, while waiting to stop")
			} else if !ci.State.Running {
				return nil
			}
		}
		time.Sleep(checkInterval)
	}
}
