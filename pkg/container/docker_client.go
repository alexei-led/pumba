package container

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	ctypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	dockerapi "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

// NewClient returns a new Client instance which can be used to interact with the Docker API.
func NewClient(dockerHost string, tlsConfig *tls.Config) (Client, error) {
	httpClient, err := HTTPClient(dockerHost, tlsConfig)
	if err != nil {
		return nil, err
	}

	apiClient, err := dockerapi.NewClientWithOpts(dockerapi.WithHost(dockerHost), dockerapi.WithHTTPClient(httpClient), dockerapi.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
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
	return client.listContainers(ctx, fn, ctypes.ListOptions{All: opts.All, Filters: filterArgs})
}

func (client dockerClient) listContainers(ctx context.Context, fn FilterFunc, opts ctypes.ListOptions) ([]*Container, error) {
	log.Debug("listing containers")
	containers, err := client.containerAPI.ContainerList(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	var cs []*Container
	for _, container := range containers {
		containerInfo, err := client.containerAPI.ContainerInspect(ctx, container.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container: %w", err)
		}
		log.WithFields(log.Fields{
			"name": containerInfo.Name,
			"id":   containerInfo.ID,
		}).Debug("found container")

		imgInfo, err := client.imageAPI.ImageInspect(ctx, containerInfo.Image)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container img: %w", err)
		}
		c := &Container{ContainerInfo: containerInfo, ImageInfo: imgInfo}
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
			return fmt.Errorf("failed to kill container: %w", err)
		}
	}
	return nil
}

// ExecContainer executes a command in a container
func (client dockerClient) ExecContainer(ctx context.Context, c *Container, command string, args []string, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":    c.Name(),
		"id":      c.ID(),
		"command": command,
		"dryrun":  dryrun,
	}).Info("exec container")
	if !dryrun {
		createRes, err := client.containerAPI.ContainerExecCreate(
			ctx, c.ID(), ctypes.ExecOptions{
				User:         "root",
				AttachStdout: true,
				AttachStderr: true,
				Cmd:          append([]string{command}, args...),
			},
		)
		if err != nil {
			return fmt.Errorf("exec create failed: %w", err)
		}

		attachRes, err := client.containerAPI.ContainerAttach(
			ctx, createRes.ID, ctypes.AttachOptions{},
		)
		if err != nil {
			return fmt.Errorf("exec attach failed: %w", err)
		}

		if err = client.containerAPI.ContainerExecStart(
			ctx, createRes.ID, ctypes.ExecStartOptions{},
		); err != nil {
			return fmt.Errorf("exec start failed: %w", err)
		}

		output, err := io.ReadAll(attachRes.Reader)
		if err != nil {
			return fmt.Errorf("reading output from exec reader failed: %w", err)
		}
		log.WithFields(log.Fields{
			"name":    c.Name(),
			"id":      c.ID(),
			"command": command,
			"args":    args,
			"dryrun":  dryrun,
		}).Info(string(output))

		res, err := client.containerAPI.ContainerExecInspect(ctx, createRes.ID)
		if err != nil {
			return fmt.Errorf("exec inspect failed: %w", err)
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
		// convert timeout to seconds
		timeoutSec := int(timeout.Seconds())
		if err := client.containerAPI.ContainerRestart(ctx, c.ID(), ctypes.StopOptions{Timeout: &timeoutSec}); err != nil {
			return fmt.Errorf("failed to restart container: %w", err)
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
			return fmt.Errorf("failed to kill container: %w", err)
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
				return fmt.Errorf("failed to kill container: %w", err)
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
		// convert timeout to seconds
		timeoutSec := int(timeout.Seconds())
		err := client.containerAPI.ContainerStop(ctx, containerID, ctypes.StopOptions{Timeout: &timeoutSec})
		if err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
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
		err := client.containerAPI.ContainerStart(ctx, c.ID(), ctypes.StartOptions{})
		if err != nil {
			return fmt.Errorf("failed to start container: %w", err)
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
		removeOpts := ctypes.RemoveOptions{
			RemoveVolumes: volumes,
			RemoveLinks:   links,
			Force:         force,
		}
		err := client.containerAPI.ContainerRemove(ctx, c.ID(), removeOpts)
		if err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}
	return nil
}

// NetemContainer injects sidecar netem container into the given container network namespace
func (client dockerClient) NetemContainer(ctx context.Context, c *Container, netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string, duration time.Duration, tcimg string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":     c.Name(),
		"id":       c.ID(),
		"command":  netemCmd,
		"ips":      ips,
		"sports":   sports,
		"dports":   dports,
		"duration": duration,
		"tc-img":   tcimg,
		"pull":     pull,
		"dryrun":   dryrun,
	}).Info("running netem on container")
	if len(ips) == 0 && len(sports) == 0 && len(dports) == 0 {
		return client.startNetemContainer(ctx, c, netInterface, netemCmd, tcimg, pull, dryrun)
	}
	return client.startNetemContainerIPFilter(ctx, c, netInterface, netemCmd, ips, sports, dports, tcimg, pull, dryrun)
}

// StopNetemContainer stops the netem container injected into the given container network namespace
func (client dockerClient) StopNetemContainer(ctx context.Context, c *Container, netInterface string, ip []*net.IPNet, sports, dports []string, tcimg string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"IPs":    ip,
		"sports": sports,
		"dports": dports,
		"iface":  netInterface,
		"tc-img": tcimg,
		"pull":   pull,
		"dryrun": dryrun,
	}).Info("stopping netem on container")
	return client.stopNetemContainer(ctx, c, netInterface, ip, sports, dports, tcimg, pull, dryrun)
}

// IPTablesContainer injects sidecar iptables container into the given container network namespace
func (client dockerClient) IPTablesContainer(ctx context.Context, c *Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, duration time.Duration, img string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":          c.Name(),
		"id":            c.ID(),
		"commandPrefix": cmdPrefix,
		"commandSuffix": cmdSuffix,
		"srcIPs":        srcIPs,
		"dstIPs":        dstIPs,
		"sports":        sports,
		"dports":        dports,
		"duration":      duration,
		"img":           img,
		"pull":          pull,
		"dryrun":        dryrun,
	}).Info("running iptables on container")
	if len(srcIPs) == 0 && len(dstIPs) == 0 && len(sports) == 0 && len(dports) == 0 {
		return client.ipTablesContainer(ctx, c, cmdPrefix, cmdSuffix, img, pull, dryrun)
	}
	return client.ipTablesContainerWithIPFilter(ctx, c, cmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports, img, pull, dryrun)
}

// StopIPTablesContainer stops the iptables container injected into the given container network namespace
func (client dockerClient) StopIPTablesContainer(ctx context.Context, c *Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, img string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":          c.Name(),
		"id":            c.ID(),
		"commandPrefix": cmdPrefix,
		"commandSuffix": cmdSuffix,
		"srcIPs":        srcIPs,
		"dstIPs":        dstIPs,
		"sports":        sports,
		"dports":        dports,
		"img":           img,
		"pull":          pull,
		"dryrun":        dryrun,
	}).Info("stopping netem on container")
	if len(srcIPs) == 0 && len(dstIPs) == 0 && len(sports) == 0 && len(dports) == 0 {
		return client.ipTablesContainer(ctx, c, cmdPrefix, cmdSuffix, img, pull, dryrun)
	}
	return client.ipTablesContainerWithIPFilter(ctx, c, cmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports, img, pull, dryrun)
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
			return fmt.Errorf("failed to pause container: %w", err)
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
			return fmt.Errorf("failed to unpause container: %w", err)
		}
	}
	return nil
}

// StressContainer starts stress test on a container (CPU, memory, network, io)
func (client dockerClient) StressContainer(ctx context.Context, c *Container, stressors []string, img string, pull bool, duration time.Duration, dryrun bool) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"name":      c.Name(),
		"id":        c.ID(),
		"stressors": stressors,
		"img":       img,
		"pull":      pull,
		"duration":  duration,
		"dryrun":    dryrun,
	}).Info("stress testing container")
	if !dryrun {
		return client.stressContainerCommand(ctx, c.ID(), stressors, img, pull)
	}
	return "", nil, nil, nil
}

func (client dockerClient) startNetemContainer(ctx context.Context, c *Container, netInterface string, netemCmd []string, tcimg string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"iface":  netInterface,
		"netem":  strings.Join(netemCmd, " "),
		"tcimg":  tcimg,
		"pull":   pull,
		"dryrun": dryrun,
	}).Debug("start netem for container")
	if !dryrun {
		// use dockerclient ExecStart to run Traffic Control:
		// 'tc qdisc add dev eth0 root netem delay 100ms'
		// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
		netemCommand := append([]string{"qdisc", "add", "dev", netInterface, "root", "netem"}, netemCmd...)
		// stop disruption command
		// netemStopCommand := "tc qdisc del dev eth0 root netem"
		log.WithField("netem", strings.Join(netemCommand, " ")).Debug("adding netem qdisc")
		return client.tcCommands(ctx, c, [][]string{netemCommand}, tcimg, pull)
	}
	return nil
}

func (client dockerClient) stopNetemContainer(ctx context.Context, c *Container, netInterface string, ips []*net.IPNet, sports, dports []string, tcimg string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"iface":  netInterface,
		"IPs":    ips,
		"tcimg":  tcimg,
		"pull":   pull,
		"dryrun": dryrun,
	}).Debug("stop netem for container")
	if !dryrun {
		var netemCommands [][]string
		if len(ips) != 0 || len(sports) != 0 || len(dports) != 0 {
			netemCommands = [][]string{
				// delete qdisc 'parent 1:1 handle 10:'
				// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
				{"qdisc", "del", "dev", netInterface, "parent", "1:1", "handle", "10:"},
				// delete qdisc 'parent 1:2 handle 20:'
				// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
				{"qdisc", "del", "dev", netInterface, "parent", "1:2", "handle", "20:"},
				// delete qdisc 'parent 1:3 handle 30:'
				// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
				{"qdisc", "del", "dev", netInterface, "parent", "1:3", "handle", "30:"},
				// delete qdisc 'root handle 1: prio'
				// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
				{"qdisc", "del", "dev", netInterface, "root", "handle", "1:", "prio"},
			}
		} else {
			netemCommands = [][]string{
				// stop netem command
				// http://www.linuxfoundation.org/collaborate/workgroups/networking/netem
				{"qdisc", "del", "dev", netInterface, "root", "netem"},
			}
		}
		err := client.tcCommands(ctx, c, netemCommands, tcimg, pull)
		if err != nil {
			return fmt.Errorf("failed to run netem tc commands: %w", err)
		}
	}
	return nil
}

func (client dockerClient) startNetemContainerIPFilter(ctx context.Context, c *Container, netInterface string, netemCmd []string,
	ips []*net.IPNet, sports []string, dports []string, tcimg string, pull bool, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"iface":  netInterface,
		"IPs":    ips,
		"Sports": sports,
		"Dports": dports,
		"tcimg":  tcimg,
		"pull":   pull,
		"dryrun": dryrun,
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

		commands := [][]string{
			// Create a priority-based queue. This *instantly* creates classes 1:1, 1:2, 1:3
			// 'tc qdisc add dev <netInterface> root handle 1: prio'
			// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
			{"qdisc", "add", "dev", netInterface, "root", "handle", "1:", "prio"},
			// Create Stochastic Fairness Queueing (sfq) queueing discipline for 1:1 class.
			// 'tc qdisc add dev <netInterface> parent 1:1 handle 10: sfq'
			// See more: https://linux.die.net/man/8/tc-sfq
			{"qdisc", "add", "dev", netInterface, "parent", "1:1", "handle", "10:", "sfq"},
			// Create Stochastic Fairness Queueing (sfq) queueing discipline for 1:2 class
			// 'tc qdisc add dev <netInterface> parent 1:2 handle 20: sfq'
			// See more: https://linux.die.net/man/8/tc-sfq
			{"qdisc", "add", "dev", netInterface, "parent", "1:2", "handle", "20:", "sfq"},
			// Add queueing discipline for 1:3 class. No traffic is going through 1:3 yet
			// 'tc qdisc add dev <netInterface> parent 1:3 handle 30: netem <netemCmd>'
			// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
			append([]string{"qdisc", "add", "dev", netInterface, "parent", "1:3", "handle", "30:", "netem"}, netemCmd...),
		}

		// # redirect traffic to specific IP through band 3
		// 'tc filter add dev <netInterface> protocol ip parent 1:0 prio 1 u32 match ip dst <targetIP> flowid 1:3'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		for _, ip := range ips {
			commands = append(commands, []string{"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "dst", ip.String(), "flowid", "1:3"})
		}

		// # redirect traffic to specific sport through band 3
		// 'tc filter add dev <netInterface> protocol ip parent 1:0 prio 1 u32 match ip <s/d>port <targetPort> 0xffff flowid 1:3'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		for _, sport := range sports {
			commands = append(commands, []string{"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "sport", sport, "0xffff", "flowid", "1:3"})
		}

		// # redirect traffic to specific dport through band 3
		// 'tc filter add dev <netInterface> protocol ip parent 1:0 prio 1 u32 match ip <s/d>port <targetPort> 0xffff flowid 1:3'
		// See more: http://man7.org/linux/man-pages/man8/tc-netem.8.html
		for _, dport := range dports {
			commands = append(commands, []string{"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "dport", dport, "0xffff", "flowid", "1:3"})
		}

		err := client.tcCommands(ctx, c, commands, tcimg, pull)
		if err != nil {
			return fmt.Errorf("failed to run tc commands: %w", err)
		}
	}
	return nil
}

func (client dockerClient) tcCommands(ctx context.Context, c *Container, argsList [][]string, tcimg string, pull bool) error {
	if tcimg == "" {
		for _, args := range argsList {
			if err := client.execOnContainer(ctx, c, "tc", args, true); err != nil {
				return fmt.Errorf("error running tc command on container: %v: %w", strings.Join(args, " "), err)
			}
		}
		return nil
	}
	return client.tcContainerCommands(ctx, c, argsList, tcimg, pull)
}

//nolint:dupl
func (client dockerClient) tcExecCommand(ctx context.Context, execID string, args []string) error {
	execConfig := ctypes.ExecOptions{
		Cmd: append([]string{"tc"}, args...),
	}
	execCreateResponse, err := client.containerAPI.ContainerExecCreate(ctx, execID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create tc-container exec: %w", err)
	}
	if err = client.containerAPI.ContainerExecStart(ctx, execCreateResponse.ID, ctypes.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start tc-container exec: %w", err)
	}
	log.WithField("args", strings.Join(args, " ")).Debug("run command on tc-container")
	return nil
}

// execute tc commands using other container (with iproute2 package installed), using target container network stack
// try to use `gaiadocker\iproute2` img (Alpine + iproute2 package)
func (client dockerClient) tcContainerCommands(ctx context.Context, target *Container, argsList [][]string, tcimg string, pull bool) error {
	log.WithFields(log.Fields{
		"container": target.ID(),
		"tc-img":    tcimg,
		"pull":      pull,
		"args-list": argsList,
	}).Debug("executing tc command in a separate container joining target container network namespace")

	// host config
	hconfig := ctypes.HostConfig{
		// Don't auto-remove, since we may want to run multiple commands
		AutoRemove: false,
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
	// pull docker img if required: can pull only public imgs
	if pull {
		log.WithField("img", tcimg).Debug("pulling tc-img")
		events, err := client.imageAPI.ImagePull(ctx, tcimg, imagetypes.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull tc-img: %w", err)
		}
		defer events.Close()
		d := json.NewDecoder(events)
		var pullResponse *imagePullResponse
		for {
			if err = d.Decode(&pullResponse); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return fmt.Errorf("failed to decode docker pull response for tc-img: %w", err)
			}
			log.Debug(pullResponse)
		}
	}

	// container config
	config := ctypes.Config{
		Labels: map[string]string{"com.gaiaadm.pumba.skip": "true"},
		// Use default entrypoint (tail -f /dev/null) from img to keep container alive
		Image: tcimg,
	}

	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")

	log.WithField("img", config.Image).Debug("creating tc-container")
	if err != nil {
		return fmt.Errorf("failed to create tc-container from tc-img: %w", err)
	}
	log.WithField("id", createResponse.ID).Debug("tc container created, starting it")
	err = client.containerAPI.ContainerStart(ctx, createResponse.ID, ctypes.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start tc-container: %w", err)
	}

	for _, args := range argsList {
		if err = client.tcExecCommand(ctx, createResponse.ID, args); err != nil {
			return fmt.Errorf("error running tc command on container: %v: %w", strings.Join(args, " "), err)
		}
	}

	if err = client.containerAPI.ContainerRemove(ctx, createResponse.ID, ctypes.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove tc-container: %w", err)
	}

	return nil
}

// execute a stress-ng command in stress-ng Docker container in target container cgroup
//
//nolint:funlen
func (client dockerClient) stressContainerCommand(ctx context.Context, targetID string, stressors []string, img string, pull bool) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"target":    targetID,
		"stressors": stressors,
		"img":       img,
		"pull":      pull,
	}).Debug("executing stress-ng command")
	dockhackArgs := append([]string{targetID, "stress-ng"}, stressors...)
	// container config
	config := ctypes.Config{
		Labels: map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Image:  img,
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
	// pull docker img if required: can pull only public imgs
	if pull {
		log.WithField("img", config.Image).Debug("pulling stress-ng img")
		events, err := client.imageAPI.ImagePull(ctx, config.Image, imagetypes.PullOptions{})
		if err != nil {
			close(outerr)
			close(output)
			return "", output, outerr, fmt.Errorf("failed to pull stress-ng img: %w", err)
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
				return "", output, outerr, fmt.Errorf("failed to decode docker pull result: %w", err)
			}
			log.Debug(pullResponse)
		}
	}
	// create stress-ng container
	log.WithField("img", config.Image).Debug("creating stress-ng container")
	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")
	if err != nil {
		close(outerr)
		close(output)
		return "", output, outerr, fmt.Errorf("failed to create stress-ng container: %w", err)
	}
	// attach to stress-ng container, capturing stdout and stderr
	opts := ctypes.AttachOptions{
		Stdin:  false,
		Stdout: true,
		Stderr: true,
		Stream: true,
	}
	attach, err := client.containerAPI.ContainerAttach(ctx, createResponse.ID, opts)
	if err != nil {
		close(outerr)
		close(output)
		return "", output, outerr, fmt.Errorf("failed to attach to stress-ng container: %w", err)
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
			outerr <- fmt.Errorf("failed to inspect stress-ng container: %w", e)
			return
		}
		// get status of stress-ng command
		if inspect.State.ExitCode != 0 {
			outerr <- fmt.Errorf("stress-ng exited with error: %v", stdout.String())
			return
		}
		output <- stdout.String()
	}()
	// start stress-ng container running stress-ng in target container cgroup
	log.WithField("id", createResponse.ID).Debug("stress-ng container created, starting it")
	err = client.containerAPI.ContainerStart(ctx, createResponse.ID, ctypes.StartOptions{})
	if err != nil {
		return createResponse.ID, output, outerr, fmt.Errorf("failed to start stress-ng container: %w", err)
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
	checkExists := ctypes.ExecOptions{
		Cmd: []string{"which", execCmd},
	}
	exec, err := client.containerAPI.ContainerExecCreate(ctx, c.ID(), checkExists)
	if err != nil {
		return fmt.Errorf("failed to create exec configuration to check if command exists: %w", err)
	}
	log.WithField("command", execCmd).Debugf("checking if command exists")
	err = client.containerAPI.ContainerExecStart(ctx, exec.ID, ctypes.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to check if command exists in a container: %w", err)
	}
	checkInspect, err := client.containerAPI.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect check execution: %w", err)
	}
	if checkInspect.ExitCode != 0 {
		return fmt.Errorf("command '%s' not found inside the %s container", execCmd, c.ID())
	}

	// if command found execute it
	log.WithField("command", execCmd).Debug("command found: continue execution")

	// prepare exec config
	config := ctypes.ExecOptions{
		Privileged: privileged,
		Cmd:        append([]string{execCmd}, execArgs...),
	}
	// execute the command
	exec, err = client.containerAPI.ContainerExecCreate(ctx, c.ID(), config)
	if err != nil {
		return fmt.Errorf("failed to create exec configuration for a command: %w", err)
	}
	log.Debugf("starting exec %s %s (%s)", execCmd, execArgs, exec.ID)
	err = client.containerAPI.ContainerExecStart(ctx, exec.ID, ctypes.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start command execution: %w", err)
	}
	exitInspect, err := client.containerAPI.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect command execution: %w", err)
	}
	if exitInspect.ExitCode != 0 {
		return fmt.Errorf("command '%s' failed in %s container; run it in manually to debug", execCmd, c.ID())
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
				return fmt.Errorf("failed to inspect container, while waiting to stop: %w", err)
			} else if !ci.State.Running {
				return nil
			}
		}
		time.Sleep(checkInterval)
	}
}

func (client dockerClient) ipTablesContainer(ctx context.Context, c *Container, cmdPrefix, cmdSuffix []string, img string, pull, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":      c.Name(),
		"id":        c.ID(),
		"cmdPrefix": strings.Join(cmdPrefix, " "),
		"cmdSuffix": strings.Join(cmdSuffix, " "),
		"img":       img,
		"pull":      pull,
		"dryrun":    dryrun,
	}).Debug("execute iptables for container")
	if !dryrun {
		var command []string
		command = append(command, cmdPrefix...)
		command = append(command, cmdSuffix...)
		log.WithField("iptables", strings.Join(command, " ")).Debug("executing iptables")
		return client.ipTablesCommands(ctx, c, [][]string{command}, img, pull)
	}
	return nil
}

func (client dockerClient) ipTablesContainerWithIPFilter(ctx context.Context, c *Container, cmdPrefix, cmdSuffix []string,
	srcIPs, dstIPs []*net.IPNet, sports, dports []string, img string, pull bool, dryrun bool) error {
	log.WithFields(log.Fields{
		"name":   c.Name(),
		"id":     c.ID(),
		"srcIPs": srcIPs,
		"dstIPs": dstIPs,
		"Sports": sports,
		"Dports": dports,
		"img":    img,
		"pull":   pull,
		"dryrun": dryrun,
	}).Debug("execute iptables for container with IP(s) filter")
	if !dryrun {
		// use docker client ExecStart to run iptables rules to filter network
		commands := [][]string{}

		// See more about the iptables statistics extension: https://www.man7.org/linux/man-pages/man8/iptables-extensions.8.html
		// # drop traffic to a specific source address

		for _, ip := range srcIPs {
			cmd := []string{}
			cmd = append(cmd, cmdPrefix...)
			cmd = append(cmd, "-s", ip.String())
			cmd = append(cmd, cmdSuffix...)
			commands = append(commands, cmd)
		}

		// # drop traffic to a specific destination address
		for _, ip := range dstIPs {
			cmd := []string{}
			cmd = append(cmd, cmdPrefix...)
			cmd = append(cmd, "-d", ip.String())
			cmd = append(cmd, cmdSuffix...)
			commands = append(commands, cmd)
		}

		// # drop traffic to a specific source port
		for _, sport := range sports {
			cmd := []string{}
			cmd = append(cmd, cmdPrefix...)
			cmd = append(cmd, "--sport", sport)
			cmd = append(cmd, cmdSuffix...)
			commands = append(commands, cmd)
		}

		// # drop traffic to a specific destination port
		for _, dport := range dports {
			cmd := []string{}
			cmd = append(cmd, cmdPrefix...)
			cmd = append(cmd, "--dport", dport)
			cmd = append(cmd, cmdSuffix...)
			commands = append(commands, cmd)
		}

		err := client.ipTablesCommands(ctx, c, commands, img, pull)
		if err != nil {
			return fmt.Errorf("failed to run iptables commands: %w", err)
		}
	}
	return nil
}

func (client dockerClient) ipTablesCommands(ctx context.Context, c *Container, argsList [][]string, tcimg string, pull bool) error {
	if tcimg == "" {
		for _, args := range argsList {
			if err := client.execOnContainer(ctx, c, "iptables", args, true); err != nil {
				return fmt.Errorf("error running iptables command on container: %v: %w", strings.Join(args, " "), err)
			}
		}
		return nil
	}
	return client.ipTablesContainerCommands(ctx, c, argsList, tcimg, pull)
}

//nolint:dupl
func (client dockerClient) ipTablesExecCommand(ctx context.Context, execID string, args []string) error {
	execConfig := ctypes.ExecOptions{
		Cmd: append([]string{"iptables"}, args...),
	}
	execCreateResponse, err := client.containerAPI.ContainerExecCreate(ctx, execID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create iptables-container exec: %w", err)
	}
	if err = client.containerAPI.ContainerExecStart(ctx, execCreateResponse.ID, ctypes.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start iptables-container exec: %w", err)
	}
	log.WithField("args", strings.Join(args, " ")).Debug("run command on iptables-container")
	return nil
}

// execute iptables commands using other container (with iproute2 and bind-tools package installed), using target container network stack
// try to use `biarca/iptables` img (Alpine + iproute2 and bind-tools package)
func (client dockerClient) ipTablesContainerCommands(ctx context.Context, target *Container, argsList [][]string, img string, pull bool) error {
	log.WithFields(log.Fields{
		"container": target.ID(),
		"img":       img,
		"pull":      pull,
		"args-list": argsList,
	}).Debug("executing iptables command in a separate container joining target container network namespace")

	// host config
	hconfig := ctypes.HostConfig{
		// Don't auto-remove, since we may want to run multiple commands
		AutoRemove: false,
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
	// pull docker img if required: can pull only public imgs
	if pull {
		log.WithField("img", img).Debug("pulling iptables-img")
		events, err := client.imageAPI.ImagePull(ctx, img, imagetypes.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull iptables-img: %w", err)
		}
		defer events.Close()
		d := json.NewDecoder(events)
		var pullResponse *imagePullResponse
		for {
			if err = d.Decode(&pullResponse); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return fmt.Errorf("failed to decode docker pull response for iptables-img: %w", err)
			}
			log.Debug(pullResponse)
		}
	}

	// container config, keep the container alive by tailing /dev/null
	config := ctypes.Config{
		Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Entrypoint: []string{"tail"},
		Cmd:        []string{"-f", "/dev/null"},
		Image:      img,
	}

	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")

	log.WithField("img", config.Image).Debug("creating iptables-container")
	if err != nil {
		return fmt.Errorf("failed to create iptables-container from iptables-img: %w", err)
	}
	log.WithField("id", createResponse.ID).Debug("iptables container created, starting it")
	err = client.containerAPI.ContainerStart(ctx, createResponse.ID, ctypes.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start iptables-container: %w", err)
	}

	for _, args := range argsList {
		if err = client.ipTablesExecCommand(ctx, createResponse.ID, args); err != nil {
			return fmt.Errorf("error running iptables command on container: %v: %w", strings.Join(args, " "), err)
		}
	}

	if err = client.containerAPI.ContainerRemove(ctx, createResponse.ID, ctypes.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove iptables-container: %w", err)
	}

	return nil
}
