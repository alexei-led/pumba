package containerd

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"syscall"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/errdefs"
	log "github.com/sirupsen/logrus"
)

const (
	defaultSocket    = "/run/containerd/containerd.sock"
	defaultNamespace = "k8s.io"
)

// containerdClient implements ctr.Client for the containerd runtime.
type containerdClient struct {
	client    apiClient
	namespace string
}

// NewClient creates a new containerd client connected to the given socket.
func NewClient(socket, namespace string) (ctr.Client, error) {
	if socket == "" {
		socket = defaultSocket
	}
	if namespace == "" {
		namespace = defaultNamespace
	}
	c, err := containerd.New(socket)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	return &containerdClient{client: c, namespace: namespace}, nil
}

// Close releases the containerd client connection.
func (c *containerdClient) Close() error {
	return c.client.Close()
}

// nsCtx returns a context with the containerd namespace set.
func (c *containerdClient) nsCtx(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, c.namespace)
}

// resolveStopSignal parses the container's configured stop signal, falling back to SIGTERM.
func resolveStopSignal(container *ctr.Container) syscall.Signal {
	sig := syscall.SIGTERM
	if s := container.StopSignal(); s != "" {
		parsed, err := parseSignal(s)
		if err != nil {
			log.WithError(err).WithField("id", container.ID()).Warn("invalid stop signal, using SIGTERM")
		} else {
			sig = parsed
		}
	}
	return sig
}

// forceKillTask kills and deletes a container's task for forced removal.
func (c *containerdClient) forceKillTask(ctx context.Context, cntr containerd.Container, id string) {
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			log.WithError(err).WithField("id", id).Warn("failed to get task during force remove")
		}
		return
	}
	waitCh, wErr := task.Wait(ctx)
	if killErr := task.Kill(ctx, syscall.SIGKILL); killErr != nil {
		log.WithError(killErr).WithField("id", id).Warn("failed to kill task during force remove")
	}
	if wErr == nil {
		select {
		case <-waitCh:
		case <-ctx.Done():
		}
	}
	_, _ = task.Delete(ctx)
}

// ListContainers lists containers from containerd and applies the filter.
func (c *containerdClient) ListContainers(ctx context.Context, fn ctr.FilterFunc, opts ctr.ListOpts) ([]*ctr.Container, error) {
	if len(opts.Labels) > 0 {
		return nil, fmt.Errorf("containerd runtime: label filtering is not yet implemented")
	}
	ctx = c.nsCtx(ctx)
	containers, err := c.client.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list containerd containers: %w", err)
	}

	var result []*ctr.Container
	for _, cntr := range containers {
		pc, skip, err := toContainer(ctx, cntr, opts.All)
		if err != nil {
			log.WithError(err).WithField("id", cntr.ID()).Warn("skipping container")
			continue
		}
		if skip {
			continue
		}
		if fn == nil || fn(pc) {
			result = append(result, pc)
		}
	}
	return result, nil
}

// StopContainer stops a container by sending its configured stop signal and waiting.
func (c *containerdClient) StopContainer(ctx context.Context, container *ctr.Container, timeout int, dryrun bool) error {
	sig := resolveStopSignal(container)
	log.WithFields(log.Fields{"id": container.ID(), "timeout": timeout, "signal": sig}).Debug("stopping containerd container")
	if dryrun {
		return nil
	}
	return c.stopTask(c.nsCtx(ctx), container.ID(), sig, time.Duration(timeout)*time.Second)
}

// KillContainer sends a signal to the container's task.
func (c *containerdClient) KillContainer(ctx context.Context, container *ctr.Container, signal string, dryrun bool) error {
	log.WithFields(log.Fields{"id": container.ID(), "signal": signal}).Debug("killing containerd container")
	if dryrun {
		return nil
	}
	return c.killTask(c.nsCtx(ctx), container.ID(), signal)
}

// StartContainer starts a container's task.
func (c *containerdClient) StartContainer(ctx context.Context, container *ctr.Container, dryrun bool) error {
	log.WithField("id", container.ID()).Debug("starting containerd container")
	if dryrun {
		return nil
	}
	return c.startTask(c.nsCtx(ctx), container.ID())
}

// RestartContainer stops and starts a container's task.
func (c *containerdClient) RestartContainer(ctx context.Context, container *ctr.Container, timeout time.Duration, dryrun bool) error {
	log.WithFields(log.Fields{"id": container.ID(), "timeout": timeout}).Debug("restarting containerd container")
	if dryrun {
		return nil
	}
	ctx = c.nsCtx(ctx)
	sig := resolveStopSignal(container)
	if err := c.stopTask(ctx, container.ID(), sig, timeout); err != nil {
		return fmt.Errorf("restart: stop failed: %w", err)
	}
	return c.startTask(ctx, container.ID())
}

// RemoveContainer deletes a container and optionally its task.
func (c *containerdClient) RemoveContainer(ctx context.Context, container *ctr.Container, force, links, volumes, dryrun bool) error {
	log.WithFields(log.Fields{"id": container.ID(), "force": force}).Debug("removing containerd container")
	if links || volumes {
		log.WithField("id", container.ID()).Debug("containerd runtime: links/volumes removal not supported, ignored")
	}
	if dryrun {
		return nil
	}
	ctx = c.nsCtx(ctx)
	cntr, err := c.client.LoadContainer(ctx, container.ID())
	if err != nil {
		return fmt.Errorf("failed to load container %s: %w", container.ID(), err)
	}
	if force {
		c.forceKillTask(ctx, cntr, container.ID())
	}
	// Try to delete with snapshot cleanup first; if that fails (e.g. Docker-managed
	// containers in the moby namespace have no snapshot key), fall back to plain delete.
	// Note: For Docker-managed containers, Docker daemon may react to the task being
	// killed and clean up the container automatically, so "not found" is acceptable.
	info, infoErr := cntr.Info(ctx)
	if infoErr != nil {
		if errdefs.IsNotFound(infoErr) {
			return nil
		}
		return fmt.Errorf("failed to get container info %s: %w", container.ID(), infoErr)
	}
	var deleteErr error
	if info.SnapshotKey != "" {
		deleteErr = cntr.Delete(ctx, containerd.WithSnapshotCleanup)
	} else {
		deleteErr = cntr.Delete(ctx)
	}
	if deleteErr != nil {
		if errdefs.IsNotFound(deleteErr) {
			return nil
		}
		return fmt.Errorf("failed to delete container %s: %w", container.ID(), deleteErr)
	}
	return nil
}

// PauseContainer pauses a container's task.
func (c *containerdClient) PauseContainer(ctx context.Context, container *ctr.Container, dryrun bool) error {
	log.WithField("id", container.ID()).Debug("pausing containerd container")
	if dryrun {
		return nil
	}
	return c.pauseTask(c.nsCtx(ctx), container.ID())
}

// UnpauseContainer resumes a paused container's task.
func (c *containerdClient) UnpauseContainer(ctx context.Context, container *ctr.Container, dryrun bool) error {
	log.WithField("id", container.ID()).Debug("unpausing containerd container")
	if dryrun {
		return nil
	}
	return c.resumeTask(c.nsCtx(ctx), container.ID())
}

// StopContainerWithID stops a container by ID.
func (c *containerdClient) StopContainerWithID(ctx context.Context, containerID string, timeout time.Duration, dryrun bool) error {
	log.WithFields(log.Fields{"id": containerID, "timeout": timeout}).Debug("stopping containerd container by ID")
	if dryrun {
		return nil
	}
	return c.stopTask(c.nsCtx(ctx), containerID, syscall.SIGTERM, timeout)
}

// ExecContainer executes a command inside a running container.
func (c *containerdClient) ExecContainer(ctx context.Context, container *ctr.Container, command string, args []string, dryrun bool) error {
	log.WithFields(log.Fields{"id": container.ID(), "command": command, "args": args}).Debug("exec in containerd container")
	if dryrun {
		return nil
	}
	if len(args) == 0 {
		fields := strings.Fields(command)
		if len(fields) > 0 {
			command = fields[0]
			if len(fields) > 1 {
				args = fields[1:]
			}
		}
	}
	return c.execInContainer(c.nsCtx(ctx), container.ID(), command, args)
}

// NetemContainer applies network emulation to a container by executing tc commands.
func (c *containerdClient) NetemContainer(ctx context.Context, container *ctr.Container, netInterface string, netemCmd []string,
	ips []*net.IPNet, sports, dports []string, _ time.Duration, tcimg string, pull, dryrun bool) error {
	log.WithFields(log.Fields{"id": container.ID(), "interface": netInterface, "tc-image": tcimg}).Debug("netem on containerd container")
	if dryrun {
		return nil
	}
	tcArgs, err := buildNetemArgs(netInterface, netemCmd, ips, sports, dports)
	if err != nil {
		return err
	}
	if tcimg != "" {
		return c.sidecarExec(ctx, container, tcimg, pull, "tc", [][]string{tcArgs})
	}
	return c.execInContainer(c.nsCtx(ctx), container.ID(), "tc", tcArgs)
}

// StopNetemContainer removes network emulation from a container.
func (c *containerdClient) StopNetemContainer(ctx context.Context, container *ctr.Container, netInterface string,
	ips []*net.IPNet, sports, dports []string, tcimg string, pull, dryrun bool) error {
	log.WithFields(log.Fields{"id": container.ID(), "interface": netInterface, "tc-image": tcimg}).Debug("stop netem on containerd container")
	if len(ips) > 0 || len(sports) > 0 || len(dports) > 0 {
		log.WithField("id", container.ID()).Warn("containerd runtime: IP/port filters ignored during netem cleanup")
	}
	if dryrun {
		return nil
	}
	tcArgs := buildStopNetemArgs(netInterface)
	if tcimg != "" {
		return c.sidecarExec(ctx, container, tcimg, pull, "tc", [][]string{tcArgs})
	}
	return c.execInContainer(c.nsCtx(ctx), container.ID(), "tc", tcArgs)
}

// IPTablesContainer applies iptables rules to a container.
func (c *containerdClient) IPTablesContainer(ctx context.Context, container *ctr.Container,
	cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string,
	_ time.Duration, tcimg string, pull, dryrun bool) error {
	log.WithField("id", container.ID()).Debug("iptables on containerd container")
	if dryrun {
		return nil
	}
	commands := buildIPTablesCommands(cmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports)
	if tcimg != "" {
		return c.sidecarExec(ctx, container, tcimg, pull, "iptables", commands)
	}
	return c.runIPTablesCommands(ctx, container.ID(), commands)
}

// StopIPTablesContainer removes iptables rules from a container.
func (c *containerdClient) StopIPTablesContainer(ctx context.Context, container *ctr.Container,
	cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string,
	tcimg string, pull, dryrun bool) error {
	log.WithField("id", container.ID()).Debug("stop iptables on containerd container")
	if dryrun {
		return nil
	}
	commands := buildIPTablesCommands(cmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports)
	if tcimg != "" {
		return c.sidecarExec(ctx, container, tcimg, pull, "iptables", commands)
	}
	return c.runIPTablesCommands(ctx, container.ID(), commands)
}

func (c *containerdClient) runIPTablesCommands(ctx context.Context, containerID string, commands [][]string) error {
	ctx = c.nsCtx(ctx)
	for _, args := range commands {
		if err := c.execInContainer(ctx, containerID, "iptables", args); err != nil {
			return fmt.Errorf("failed to run iptables command: %w", err)
		}
	}
	return nil
}

// StressContainer runs stress-ng inside a container.
func (c *containerdClient) StressContainer(ctx context.Context, container *ctr.Container,
	stressors []string, image string, pull bool, duration time.Duration, injectCgroup, dryrun bool) (string, <-chan string, <-chan error, error) {
	log.WithField("id", container.ID()).Debug("stress on containerd container")
	if image != "" || pull || injectCgroup {
		log.WithField("id", container.ID()).Debug("containerd runtime: sidecar/inject-cgroup stress modes not supported, using direct exec")
	}
	if dryrun {
		return "", nil, nil, nil
	}
	errCh := make(chan error, 1)
	outCh := make(chan string, 1)
	go func() {
		defer close(errCh)
		defer close(outCh)
		secs := max(1, int(math.Ceil(duration.Seconds())))
		args := append([]string{"--timeout", fmt.Sprintf("%ds", secs)}, stressors...)
		if err := c.execInContainer(c.nsCtx(ctx), container.ID(), "stress-ng", args); err != nil {
			errCh <- err
			return
		}
		outCh <- container.ID()
	}()
	return container.ID(), outCh, errCh, nil
}
