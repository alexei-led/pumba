package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

// NetemContainer injects sidecar netem container into the given container network namespace
func (client dockerClient) NetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{
		"name":     req.Container.Name(),
		"id":       req.Container.ID(),
		"command":  req.Command,
		"ips":      req.IPs,
		"sports":   req.SPorts,
		"dports":   req.DPorts,
		"duration": req.Duration,
		"tc-img":   req.Sidecar.Image,
		"pull":     req.Sidecar.Pull,
		"dryrun":   req.DryRun,
	}).Info("running netem on container")
	if len(req.IPs) == 0 && len(req.SPorts) == 0 && len(req.DPorts) == 0 {
		return client.startNetemContainer(ctx, req.Container, req.Interface, req.Command, req.Sidecar.Image, req.Sidecar.Pull, req.DryRun)
	}
	return client.startNetemContainerIPFilter(ctx, req.Container, req.Interface, req.Command, req.IPs, req.SPorts, req.DPorts, req.Sidecar.Image, req.Sidecar.Pull, req.DryRun)
}

// StopNetemContainer stops the netem container injected into the given container network namespace
func (client dockerClient) StopNetemContainer(ctx context.Context, req *ctr.NetemRequest) error {
	log.WithFields(log.Fields{
		"name":   req.Container.Name(),
		"id":     req.Container.ID(),
		"IPs":    req.IPs,
		"sports": req.SPorts,
		"dports": req.DPorts,
		"iface":  req.Interface,
		"tc-img": req.Sidecar.Image,
		"pull":   req.Sidecar.Pull,
		"dryrun": req.DryRun,
	}).Info("stopping netem on container")
	return client.stopNetemContainer(ctx, req.Container, req.Interface, req.IPs, req.SPorts, req.DPorts, req.Sidecar.Image, req.Sidecar.Pull, req.DryRun)
}

func (client dockerClient) startNetemContainer(ctx context.Context, c *ctr.Container, netInterface string, netemCmd []string, tcimg string, pull, dryrun bool) error {
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

func (client dockerClient) stopNetemContainer(ctx context.Context, c *ctr.Container, netInterface string, ips []*net.IPNet, sports, dports []string, tcimg string, pull, dryrun bool) error {
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

func (client dockerClient) startNetemContainerIPFilter(ctx context.Context, c *ctr.Container, netInterface string, netemCmd []string,
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
		//       10:  20:  30:    qdiscs
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

func (client dockerClient) tcCommands(ctx context.Context, c *ctr.Container, argsList [][]string, tcimg string, pull bool) error {
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

// execute tc commands using other container (with iproute2 package installed), using target container network stack
// try to use `gaiadocker\iproute2` img (Alpine + iproute2 package)
//
//nolint:dupl // intentionally parallel to ipTablesContainerCommands; keeping them separate reads clearer at callsite
func (client dockerClient) tcContainerCommands(ctx context.Context, target *ctr.Container, argsList [][]string, tcimg string, pull bool) error {
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

	// container config — explicit Entrypoint/Cmd so the sidecar stays alive
	// regardless of the tc-image's default (e.g. nicolaka/netshoot defaults
	// to zsh which exits immediately in detached mode). StopSignal: SIGKILL
	// skips the SIGTERM-then-wait grace period on `rm -f`: tail as PID 1
	// ignores SIGTERM, which otherwise makes Podman wait the full 10 s
	// StopTimeout before escalating (~tens of seconds per chaos cycle).
	config := ctypes.Config{
		Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Entrypoint: []string{"tail"},
		Cmd:        []string{"-f", "/dev/null"},
		Image:      tcimg,
		StopSignal: "SIGKILL",
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
			_ = client.removeSidecar(ctx, createResponse.ID)
			return fmt.Errorf("error running tc command on container: %v: %w", strings.Join(args, " "), err)
		}
	}

	if err = client.removeSidecar(ctx, createResponse.ID); err != nil {
		return fmt.Errorf("failed to remove tc-container: %w", err)
	}

	return nil
}
