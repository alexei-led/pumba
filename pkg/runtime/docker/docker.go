package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

type imagePullResponse struct {
	Status         string `json:"status"`
	Error          string `json:"error"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"progressDetail"`
}

const cgroupDriverSystemd = "systemd"
const cgroupDriverCgroupfs = "cgroupfs"

// NetemContainer injects sidecar netem container into the given container network namespace
func (client dockerClient) NetemContainer(ctx context.Context, c *ctr.Container, netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string, duration time.Duration, tcimg string, pull, dryrun bool) error {
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
func (client dockerClient) StopNetemContainer(ctx context.Context, c *ctr.Container, netInterface string, ip []*net.IPNet, sports, dports []string, tcimg string, pull, dryrun bool) error {
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
func (client dockerClient) IPTablesContainer(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, duration time.Duration, img string, pull, dryrun bool) error {
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
func (client dockerClient) StopIPTablesContainer(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, img string, pull, dryrun bool) error {
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

// StressContainer starts stress test on a container (CPU, memory, network, io)
func (client dockerClient) StressContainer(ctx context.Context, c *ctr.Container, stressors []string, img string, pull bool, duration time.Duration, injectCgroup, dryrun bool) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"name":          c.Name(),
		"id":            c.ID(),
		"stressors":     stressors,
		"img":           img,
		"pull":          pull,
		"duration":      duration,
		"inject-cgroup": injectCgroup,
		"dryrun":        dryrun,
	}).Info("stress testing container")
	if dryrun {
		return "", nil, nil, nil
	}
	return client.stressContainerCommand(ctx, c.ID(), stressors, img, pull, injectCgroup)
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

// sidecarRemoveTimeout bounds how long pumba will wait for ContainerRemove
// to reap an ephemeral tc/iptables sidecar after the caller's ctx cancels
// (e.g. SIGTERM). Podman's force-remove can take a few seconds on slow VMs.
const sidecarRemoveTimeout = 15 * time.Second

// removeSidecar force-removes an ephemeral tc/iptables sidecar container.
// Uses context.WithoutCancel with a short timeout so cleanup still runs
// when the caller's ctx was canceled by SIGTERM — otherwise pumba would
// leak the sidecar AND the rules it installed in the target's netns,
// because the caller early-returns on this error.
func (client dockerClient) removeSidecar(ctx context.Context, id string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarRemoveTimeout)
	defer cancel()
	return client.containerAPI.ContainerRemove(cleanupCtx, id, ctypes.RemoveOptions{Force: true})
}

// cgroupDriver queries the Docker daemon for its cgroup driver.
// Returns the driver name or empty string with error on failure.
func (client dockerClient) cgroupDriver(ctx context.Context) (string, error) {
	info, err := client.systemAPI.Info(ctx)
	if err != nil {
		log.WithError(err).Warn("failed to get docker info, assuming cgroupfs driver")
		return "", err
	}
	return info.CgroupDriver, nil
}

// containerLeafCgroup returns the leaf cgroup directory name for a container
// based on the cgroup driver. On cgroupfs the leaf is the container ID; on
// systemd it is a scope unit named "docker-<id>.scope".
func containerLeafCgroup(targetID, driver string) string {
	if driver == cgroupDriverSystemd {
		return "docker-" + targetID + ".scope"
	}
	return targetID
}

// inspectCgroupParent returns the target container's CgroupParent from inspect.
// Returns empty string when CgroupParent is not set (standalone Docker defaults)
// or when inspect fails.
func (client dockerClient) inspectCgroupParent(ctx context.Context, targetID string) string {
	inspect, err := client.containerAPI.ContainerInspect(ctx, targetID)
	if err != nil {
		log.WithError(err).Warn("failed to inspect target container for cgroup path")
		return ""
	}
	if inspect.HostConfig != nil && inspect.HostConfig.CgroupParent != "" {
		log.WithField("cgroup-parent", inspect.HostConfig.CgroupParent).Debug("resolved cgroup parent from container inspect")
		return inspect.HostConfig.CgroupParent
	}
	return ""
}

// defaultCgroupParent returns the default cgroup parent path based on the Docker
// daemon's cgroup driver when the target container has no explicit CgroupParent set.
func defaultCgroupParent(targetID, driver string) string {
	switch driver {
	case cgroupDriverSystemd:
		return "system.slice"
	default:
		return "/docker/" + targetID
	}
}

// stressContainerConfig builds the container and host config for a stress-ng container.
// cgroupPath is the target's cgroup base path resolved from ContainerInspect (may be empty).
// For inject-cgroup mode: when cgroupPath is known, uses --cgroup-path; otherwise falls back
// to --target-id + --cgroup-driver.
func stressContainerConfig(targetID string, stressors []string, img, driver, cgroupParent, cgroupPath string, injectCgroup bool) (ctypes.Config, ctypes.HostConfig) {
	if injectCgroup {
		var cmd []string
		if cgroupPath != "" {
			cmd = append([]string{"--cgroup-path", cgroupPath, "--", "/stress-ng"}, stressors...)
			log.WithField("cgroup-path", cgroupPath).Debug("using inject-cgroup mode with explicit cgroup path")
		} else {
			cmd = append([]string{"--target-id", targetID, "--cgroup-driver", driver, "--", "/stress-ng"}, stressors...)
			log.WithFields(log.Fields{
				"driver":    driver,
				"target-id": targetID,
			}).Debug("using inject-cgroup mode with driver-based path")
		}
		return ctypes.Config{
				Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
				Image:      img,
				Entrypoint: []string{"/cg-inject"},
				Cmd:        cmd,
			}, ctypes.HostConfig{
				AutoRemove:   true,
				CgroupnsMode: "host",
				Binds:        []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"},
			}
	}
	// default child-cgroup mode: use --cgroup-parent with the resolved path
	log.WithField("cgroup-parent", cgroupParent).Debug("resolved cgroup parent")
	return ctypes.Config{
			Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
			Image:      img,
			Entrypoint: []string{"/stress-ng"},
			Cmd:        stressors,
		}, ctypes.HostConfig{
			AutoRemove: true,
			Resources: ctypes.Resources{
				CgroupParent: cgroupParent,
			},
		}
}

// pullImage pulls a Docker image and drains the progress stream.
func (client dockerClient) pullImage(ctx context.Context, img string) error {
	log.WithField("img", img).Debug("pulling image")
	events, err := client.imageAPI.ImagePull(ctx, img, imagetypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull stress-ng img: %w", err)
	}
	defer events.Close()
	d := json.NewDecoder(events)
	var pullResponse *imagePullResponse
	for {
		if err = d.Decode(&pullResponse); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("failed to decode docker pull result: %w", err)
		}
		log.Debug(pullResponse)
	}
}

// stressResolveDriver resolves the cgroup driver, parent, and target cgroup path
// for stress container setup. For default mode, cgroupParent is the resolved path
// for --cgroup-parent. For inject-cgroup mode, cgroupPath is the target's full
// cgroup path (if known) to pass as --cgroup-path to cg-inject.
func (client dockerClient) stressResolveDriver(ctx context.Context, targetID string, injectCgroup bool) (driver, cgroupParent, cgroupPath string, err error) {
	// resolve the cgroup driver first — needed for correct leaf cgroup naming
	driver, err = client.cgroupDriver(ctx)
	if err != nil {
		// try inspect anyway; if it yields a parent we can still proceed
		inspectParent := client.inspectCgroupParent(ctx, targetID)
		if inspectParent == "" {
			return "", "", "", fmt.Errorf("failed to get docker info: %w", err)
		}
		// infer driver from parent path: systemd parents end with .slice
		if strings.HasSuffix(inspectParent, ".slice") {
			driver = cgroupDriverSystemd
		} else {
			driver = cgroupDriverCgroupfs
		}
		cgroupPath = inspectParent + "/" + containerLeafCgroup(targetID, driver)
	} else {
		if driver == "" {
			driver = cgroupDriverCgroupfs
		}
		if inspectParent := client.inspectCgroupParent(ctx, targetID); inspectParent != "" {
			cgroupPath = inspectParent + "/" + containerLeafCgroup(targetID, driver)
		}
	}

	if injectCgroup {
		return driver, cgroupParent, cgroupPath, nil
	}
	if cgroupPath == "" {
		cgroupParent = defaultCgroupParent(targetID, driver)
		return driver, cgroupParent, cgroupPath, nil
	}
	// For default mode, CgroupParent must be a value Docker accepts.
	// systemd requires a valid slice name (*.slice); cgroupfs accepts any path.
	if driver == cgroupDriverSystemd {
		cgroupParent = cgroupPath[:strings.LastIndex(cgroupPath, "/")]
	} else {
		cgroupParent = cgroupPath
	}
	return driver, cgroupParent, cgroupPath, nil
}

// execute a stress-ng command in stress-ng Docker container in target container cgroup
func (client dockerClient) stressContainerCommand(ctx context.Context, targetID string, stressors []string, img string, pull, injectCgroup bool) (string, <-chan string, <-chan error, error) {
	log.WithFields(log.Fields{
		"target":        targetID,
		"stressors":     stressors,
		"img":           img,
		"pull":          pull,
		"inject-cgroup": injectCgroup,
	}).Debug("executing stress-ng command")

	driver, cgroupParent, cgroupPath, err := client.stressResolveDriver(ctx, targetID, injectCgroup)
	if err != nil {
		return "", nil, nil, err
	}

	config, hconfig := stressContainerConfig(targetID, stressors, img, driver, cgroupParent, cgroupPath, injectCgroup)
	if pull {
		if err := client.pullImage(ctx, config.Image); err != nil {
			return "", nil, nil, err
		}
	}
	// create stress-ng container
	log.WithField("img", config.Image).Debug("creating stress-ng container")
	createResponse, err := client.containerAPI.ContainerCreate(ctx, &config, &hconfig, nil, nil, "")
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create stress-ng container: %w", err)
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
		return "", nil, nil, fmt.Errorf("failed to attach to stress-ng container: %w", err)
	}
	output := make(chan string, 1)
	outerr := make(chan error, 1)
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

func (client dockerClient) ipTablesContainer(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, img string, pull, dryrun bool) error {
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

func (client dockerClient) ipTablesContainerWithIPFilter(ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string,
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

func (client dockerClient) ipTablesCommands(ctx context.Context, c *ctr.Container, argsList [][]string, tcimg string, pull bool) error {
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

// execute iptables commands using other container (with iproute2 and bind-tools package installed), using target container network stack
// try to use `biarca/iptables` img (Alpine + iproute2 and bind-tools package)
//
//nolint:dupl // intentionally parallel to tcContainerCommands; keeping them separate reads clearer at callsite
func (client dockerClient) ipTablesContainerCommands(ctx context.Context, target *ctr.Container, argsList [][]string, img string, pull bool) error {
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

	// container config, keep the container alive by tailing /dev/null.
	// StopSignal: SIGKILL avoids the 10 s SIGTERM grace period on `rm -f`
	// (tail as PID 1 ignores SIGTERM), matching tcContainerCommands.
	config := ctypes.Config{
		Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Entrypoint: []string{"tail"},
		Cmd:        []string{"-f", "/dev/null"},
		Image:      img,
		StopSignal: "SIGKILL",
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
			_ = client.removeSidecar(ctx, createResponse.ID)
			return fmt.Errorf("error running iptables command on container: %v: %w", strings.Join(args, " "), err)
		}
	}

	if err = client.removeSidecar(ctx, createResponse.ID); err != nil {
		return fmt.Errorf("failed to remove iptables-container: %w", err)
	}

	return nil
}
