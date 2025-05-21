package container

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go" // Required for capabilities
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io" // Required for io.Copy with pipes in Exec
)

// Client is an interface for interacting with a container runtime.
type Client interface {
	ListContainers(ctx context.Context, fn FilterFunc, opts ListOpts) ([]*Container, error)
	StopContainer(ctx context.Context, c *Container, timeout int, dryrun bool) error
	KillContainer(ctx context.Context, c *Container, signal string, dryrun bool) error
	RemoveContainer(ctx context.Context, c *Container, force, links, volumes, dryrun bool) error
	// TODO: Implement other methods from Docker client for completeness if necessary
	// StartContainer(ctx context.Context, c *Container, dryrun bool) error
	// PauseContainer(ctx context.Context, c *Container, dryrun bool) error
	// UnpauseContainer(ctx context.Context, c *Container, dryrun bool) error
	// ExecContainer(ctx context.Context, c *Container, command string, args []string, dryrun bool) error
	// RestartContainer(ctx context.Context, c *Container, timeout time.Duration, dryrun bool) error
	// NetemContainer(ctx context.Context, c *Container, netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string, duration time.Duration, tcimage string, pull, dryrun bool) error
	// StopNetemContainer(ctx context.Context, c *Container, netInterface string, ip []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) error
	// IPTablesContainer(ctx context.Context, c *Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, duration time.Duration, image string, pull, dryrun bool) error
	// StopIPTablesContainer(ctx context.Context, c *Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, image string, pull, dryrun bool) error
	// StressContainer(ctx context.Context, c *Container, stressors []string, image string, pull bool, duration time.Duration, dryrun bool) (string, <-chan string, <-chan error, error)
}

// containerdNew is used to create containerd.Client instances. It is a variable
// so tests can replace it with a mock implementation.
var containerdNew = containerd.New

// containerdClient implements the Client interface using containerd.
type containerdClient struct {
	client    *containerd.Client
	namespace string
}

// NewContainerdClient creates a new Client that uses containerd.
func NewContainerdClient(address, namespace string) (Client, error) {
	if address == "" {
		return nil, fmt.Errorf("containerd address cannot be empty")
	}
	if namespace == "" {
		return nil, fmt.Errorf("containerd namespace cannot be empty")
	}

	// Add WithDefaultPlatform for broader compatibility, especially on macOS, Windows.
	// See: https://github.com/containerd/containerd/issues/3240
	// client, err := containerd.New(address, containerd.WithDefaultPlatform())
	// For Pumba, it's often run in Linux an environment matching the target, so default might be fine.
	// Let's stick to the simpler New(address) for now unless platform issues arise.
	client, err := containerdNew(address)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to containerd at %s", address)
	}
	return &containerdClient{
		client:    client,
		namespace: namespace,
	}, nil
}

// ListContainers returns a list of containerd containers that match the given filter.
func (c *containerdClient) ListContainers(ctx context.Context, fn FilterFunc, opts ListOpts) ([]*Container, error) {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	log.Debug("listing containerd containers")

	var filterStrings []string
	for _, labelSelector := range opts.Labels {
		parts := strings.SplitN(labelSelector, "=", 2)
		if len(parts) == 2 {
			// containerd filter format is "labels.key==value"
			// Ensure special characters in label keys/values are handled if necessary,
			// though containerd's parser might be robust. Quoting might be needed for complex values.
			filterStrings = append(filterStrings, fmt.Sprintf("labels.%s==%q", parts[0], parts[1]))
		} else {
			// Check for presence of the label key: "labels.key"
			filterStrings = append(filterStrings, fmt.Sprintf("labels.%s", parts[0]))
		}
	}

	listedContainers, err := c.client.Containers(ctx, filterStrings...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list containerd containers from client")
	}

	var pumbaContainers []*Container
	for _, cont := range listedContainers {
		info, err := cont.Info(ctx, containerd.WithoutRuntime())
		if err != nil {
			// If a container is removed between List and Info calls.
			if errdefs.IsNotFound(err) {
				log.WithError(err).WithField("id", cont.ID()).Debug("container not found during info retrieval, skipping")
				continue
			}
			log.WithError(err).WithField("id", cont.ID()).Warn("failed to get info for containerd container")
			continue
		}

		spec, err := cont.Spec(ctx)
		if err != nil {
			if errdefs.IsNotFound(err) { // Handle cases where spec might not be found (e.g. deleted container)
				log.WithError(err).WithField("id", cont.ID()).Debug("container spec not found, skipping or using partial info")
				// Depending on requirements, we might skip or proceed with available info
				// For now, let's try to proceed with info from cont.Info()
			} else {
				log.WithError(err).WithField("id", cont.ID()).Warn("failed to get spec for containerd container")
			}
			// Continue with partial info if spec is unavailable but info is present
		}

		imageName := info.Image              // Default to info.Image
		if spec != nil && spec.Image != "" { // Prefer spec.Image if available and not empty
			imageName = spec.Image
		}

		// Determine status
		var statusStr string
		task, errTask := cont.Task(ctx, nil) // cio.Load is not needed for just status
		if errTask != nil {
			if errdefs.IsNotFound(errTask) {
				statusStr = string(containerd.Stopped) // Or "created" if we need to distinguish
				// Based on containerd.Status, this would be Created, Stopped or Unknown
				// For Pumba, "stopped" is a reasonable generalization if no task.
			} else {
				log.WithError(errTask).WithField("id", cont.ID()).Warn("failed to get task for status check")
				statusStr = string(containerd.Unknown) // Or "unknown"
			}
		} else {
			st, errStatus := task.Status(ctx)
			if errStatus != nil {
				log.WithError(errStatus).WithField("id", cont.ID()).Warn("failed to get task status")
				statusStr = string(containerd.Unknown) // Or "unknown"
			} else {
				statusStr = string(st.Status)
			}
		}

		// Filter by opts.All: if false, only include running containers
		// containerd.Running is the correct status for this check.
		if !opts.All && statusStr != string(containerd.Running) {
			continue
		}

		// Determine container name (prefer specific labels, fallback to ID)
		containerName := cont.ID()                                         // Default to ID
		if name, ok := info.Labels[oci.AnnotationName]; ok && name != "" { // Standard OCI annotation for name
			containerName = name
		} else if name, ok := info.Labels["io.kubernetes.cri.container-name"]; ok && name != "" { // K8s CRI specific
			containerName = name
		} else if name, ok := info.Labels["name"]; ok && name != "" { // Generic name label
			containerName = name
		}

		pumbaC := &Container{
			Cid:        cont.ID(),
			Cname:      containerName,
			Clabels:    info.Labels,
			CimageName: imageName,
			Cstatus:    statusStr,
		}
		// Populate CstopSignal from labels, using the Labels() method which correctly accesses Clabels.
		pumbaC.CstopSignal = pumbaC.Labels()[signalLabel]

		if fn(pumbaC) {
			pumbaContainers = append(pumbaContainers, pumbaC)
			log.WithFields(log.Fields{
				"id":    pumbaC.ID(),
				"name":  pumbaC.Name(),
				"image": pumbaC.ImageName(),
				// "labels": pumbaC.Labels(), // Can be verbose
				"status": pumbaC.Status(),
			}).Debug("found matching containerd container")
		}
	}

	return pumbaContainers, nil
}

// parseSignal converts a signal string (e.g., "SIGKILL") to a syscall.Signal.
func parseSignal(signalStr string) (syscall.Signal, error) {
	s := strings.ToUpper(signalStr)
	if !strings.HasPrefix(s, "SIG") {
		s = "SIG" + s
	}
	// Based on https://github.com/docker/docker/blob/master/api/types/signal/signal.go
	// and common signals.
	switch s {
	case "SIGABRT":
		return syscall.SIGABRT, nil
	case "SIGALRM":
		return syscall.SIGALRM, nil
	case "SIGBUS":
		return syscall.SIGBUS, nil
	case "SIGCHLD":
		return syscall.SIGCHLD, nil
	case "SIGCONT":
		return syscall.SIGCONT, nil
	case "SIGFPE":
		return syscall.SIGFPE, nil
	case "SIGHUP":
		return syscall.SIGHUP, nil
	case "SIGILL":
		return syscall.SIGILL, nil
	case "SIGINT":
		return syscall.SIGINT, nil
	case "SIGIO": // Also SIGPOLL
		return syscall.SIGIO, nil
	case "SIGIOT":
		return syscall.SIGIOT, nil
	case "SIGKILL":
		return syscall.SIGKILL, nil
	case "SIGPIPE":
		return syscall.SIGPIPE, nil
	case "SIGPROF":
		return syscall.SIGPROF, nil
	case "SIGQUIT":
		return syscall.SIGQUIT, nil
	case "SIGSEGV":
		return syscall.SIGSEGV, nil
	case "SIGSTOP":
		return syscall.SIGSTOP, nil
	case "SIGSYS":
		return syscall.SIGSYS, nil
	case "SIGTERM":
		return syscall.SIGTERM, nil
	case "SIGTRAP":
		return syscall.SIGTRAP, nil
	case "SIGTSTP":
		return syscall.SIGTSTP, nil
	case "SIGTTIN":
		return syscall.SIGTTIN, nil
	case "SIGTTOU":
		return syscall.SIGTTOU, nil
	case "SIGURG":
		return syscall.SIGURG, nil
	case "SIGUSR1":
		return syscall.SIGUSR1, nil
	case "SIGUSR2":
		return syscall.SIGUSR2, nil
	case "SIGVTALRM":
		return syscall.SIGVTALRM, nil
	case "SIGWINCH":
		return syscall.SIGWINCH, nil
	case "SIGXCPU":
		return syscall.SIGXCPU, nil
	case "SIGXFSZ":
		return syscall.SIGXFSZ, nil
	default:
		// Try to parse as an integer if not a known string
		if val, err := strconv.Atoi(signalStr); err == nil {
			return syscall.Signal(val), nil
		}
		return 0, fmt.Errorf("unknown signal: %s", signalStr)
	}
}

// StopContainer stops a containerd container.
// It first sends SIGTERM, waits for the timeout, then sends SIGKILL if the container hasn't stopped.
func (c *containerdClient) StopContainer(ctx context.Context, pumbaContainer *Container, timeout int, dryrun bool) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	logFields := log.Fields{
		"id":      pumbaContainer.ID(),
		"name":    pumbaContainer.Name(),
		"timeout": timeout,
		"dryrun":  dryrun,
	}
	log.WithFields(logFields).Info("stopping containerd container")

	if dryrun {
		return nil
	}

	cont, err := c.client.LoadContainer(ctx, pumbaContainer.ID())
	if err != nil {
		if errdefs.IsNotFound(err) {
			log.WithFields(logFields).Warn("container not found for stopping")
			return errors.Wrapf(err, "container %s not found", pumbaContainer.ID())
		}
		return errors.Wrapf(err, "failed to load container %s", pumbaContainer.ID())
	}

	task, err := cont.Task(ctx, nil) // No need for cio.Load for just signaling
	if err != nil {
		if errdefs.IsNotFound(err) {
			log.WithFields(logFields).Info("task not found for container (already stopped or never started)")
			return nil // No task, so it's stopped or in a state where stop is not applicable.
		}
		return errors.Wrapf(err, "failed to get task for container %s", cont.ID())
	}

	// Get custom stop signal from container labels, default to SIGTERM
	stopSignalStr := pumbaContainer.StopSignal()
	if stopSignalStr == "" {
		stopSignalStr = "SIGTERM"
	}
	sigterm, err := parseSignal(stopSignalStr)
	if err != nil {
		log.WithError(err).WithFields(logFields).Warnf("invalid stop signal %s, defaulting to SIGTERM", stopSignalStr)
		sigterm = syscall.SIGTERM
	}

	log.WithFields(logFields).Debugf("sending signal %s to task", sigterm)
	if err := task.Kill(ctx, sigterm); err != nil {
		// If task is already stopped or stopping, this might error.
		// We should check the error type.
		if !strings.Contains(err.Error(), "process already finished") && !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to send %s to task for container %s", sigterm, cont.ID())
		}
		log.WithFields(logFields).Debugf("task already finished or not found when sending %s", sigterm)
	}

	// Wait for the task to stop or timeout
	statusC, err := task.Wait(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to wait for task %s", cont.ID())
	}

	select {
	case status := <-statusC:
		log.WithFields(logFields).Infof("container stopped with status %s", status.Status())
		if _, err := task.Delete(ctx); err != nil {
			// Log error but don't fail the stop operation if delete fails here,
			// as the primary goal (stopping) was achieved.
			if !errdefs.IsNotFound(err) && !strings.Contains(err.Error(), "process already finished") {
				log.WithError(err).WithFields(logFields).Warn("failed to delete task after stop")
			}
		}
		return nil
	case <-time.After(time.Duration(timeout) * time.Second):
		log.WithFields(logFields).Warnf("container did not stop within %d seconds timeout, sending SIGKILL", timeout)
		sigkill := syscall.SIGKILL
		if err := task.Kill(ctx, sigkill); err != nil {
			if !strings.Contains(err.Error(), "process already finished") && !errdefs.IsNotFound(err) {
				return errors.Wrapf(err, "failed to send SIGKILL to task for container %s", cont.ID())
			}
			log.WithFields(logFields).Debug("task already finished or not found when sending SIGKILL")
		}
		// Wait again briefly for SIGKILL to take effect
		select {
		case status := <-statusC: // This might be the original statusC or a new one if Wait was re-called.
			log.WithFields(logFields).Infof("container killed with status %s", status.Status())
		case <-time.After(5 * time.Second): // Give a few seconds for SIGKILL
			log.WithFields(logFields).Error("container did not stop even after SIGKILL")
			// Fall through to task.Delete with Force option if possible or just log.
			// The task might be in a zombie state or unkillable.
		}
		if _, err := task.Delete(ctx); err != nil {
			if !errdefs.IsNotFound(err) && !strings.Contains(err.Error(), "process already finished") {
				log.WithError(err).WithFields(logFields).Warn("failed to delete task after SIGKILL")
			}
		}
		return nil // Or an error if SIGKILL failed to make it stop? Docker client seems to proceed.
	case <-ctx.Done():
		log.WithFields(logFields).Warn("context cancelled while waiting for container to stop")
		return ctx.Err()
	}
}

// KillContainer kills a containerd container with the given signal.
func (c *containerdClient) KillContainer(ctx context.Context, pumbaContainer *Container, signalStr string, dryrun bool) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	logFields := log.Fields{
		"id":     pumbaContainer.ID(),
		"name":   pumbaContainer.Name(),
		"signal": signalStr,
		"dryrun": dryrun,
	}
	log.WithFields(logFields).Info("killing containerd container")

	if dryrun {
		return nil
	}

	sig, err := parseSignal(signalStr)
	if err != nil {
		return errors.Wrapf(err, "invalid signal %q", signalStr)
	}

	cont, err := c.client.LoadContainer(ctx, pumbaContainer.ID())
	if err != nil {
		if errdefs.IsNotFound(err) {
			log.WithFields(logFields).Warn("container not found for killing")
			return errors.Wrapf(err, "container %s not found", pumbaContainer.ID())
		}
		return errors.Wrapf(err, "failed to load container %s", pumbaContainer.ID())
	}

	task, err := cont.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			log.WithFields(logFields).Info("task not found for container (already stopped or never started)")
			// If the task is already gone, we can consider the "kill" successful in a way.
			return nil
		}
		return errors.Wrapf(err, "failed to get task for container %s", cont.ID())
	}

	log.WithFields(logFields).Debugf("sending signal %s to task", sig)
	if err := task.Kill(ctx, sig); err != nil {
		// If task is already stopped or stopping, this might error.
		if !strings.Contains(err.Error(), "process already finished") && !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to send signal %s to task for container %s", sig, cont.ID())
		}
		log.WithFields(logFields).Debug("task already finished or not found when sending signal")
	}

	// For SIGKILL, containerd might not immediately reflect the status change via Wait.
	// However, Pumba's KillContainer (Docker) doesn't explicitly wait after sending signal.
	// If signal is SIGKILL, the task might be cleaned up quickly.
	// If it's a graceful signal like SIGTERM, the caller might expect it to still be running.
	// For consistency with Docker client's KillContainer, we don't wait here.
	// The task state will eventually be updated.
	// If an immediate cleanup of task resources is needed, `task.Delete` could be called,
	// but that's usually part of RemoveContainer or StopContainer.

	return nil
}

// RemoveContainer removes a containerd container.
// `links` are ignored as they are a Docker-specific concept not directly applicable.
// `volumes` maps to deleting the snapshot if true.
func (c *containerdClient) RemoveContainer(ctx context.Context, pumbaContainer *Container, force, links, volumes, dryrun bool) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	logFields := log.Fields{
		"id":      pumbaContainer.ID(),
		"name":    pumbaContainer.Name(),
		"force":   force,
		"volumes": volumes, // This will mean snapshot cleanup for containerd
		"dryrun":  dryrun,
	}
	if links { // Log if links are requested, as it's not applicable
		log.WithFields(logFields).Debug("containerd RemoveContainer does not support 'links' option, it will be ignored")
	}
	log.WithFields(logFields).Info("removing containerd container")

	if dryrun {
		return nil
	}

	cont, err := c.client.LoadContainer(ctx, pumbaContainer.ID())
	if err != nil {
		if errdefs.IsNotFound(err) {
			log.WithFields(logFields).Warn("container not found for removal")
			return nil // If not found, it's already removed effectively.
		}
		return errors.Wrapf(err, "failed to load container %s for removal", pumbaContainer.ID())
	}

	// If force is true, try to stop/kill the container first.
	if force {
		task, err := cont.Task(ctx, nil)
		if err == nil { // Task exists, so it might be running
			log.WithFields(logFields).Debug("force remove: attempting to kill and delete task")
			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
				// Log error but proceed, delete might still work or container is already stopped
				if !strings.Contains(err.Error(), "process already finished") && !errdefs.IsNotFound(err) {
					log.WithError(err).WithFields(logFields).Warn("failed to send SIGKILL to task during force remove, proceeding with delete attempt")
				}
			}
			// Wait for task to exit after SIGKILL
			statusC, waitErr := task.Wait(ctx)
			if waitErr == nil {
				select {
				case <-statusC: // Task exited
				case <-time.After(5 * time.Second): // Timeout waiting for task to exit
					log.WithFields(logFields).Warn("timeout waiting for task to exit after SIGKILL during force remove")
				case <-ctx.Done():
					return ctx.Err()
				}
			} else if !errdefs.IsNotFound(waitErr) { // Don't log if task already gone
				log.WithError(waitErr).WithFields(logFields).Warn("failed to wait for task during force remove")
			}

			// Delete the task
			if _, err := task.Delete(ctx); err != nil {
				if !errdefs.IsNotFound(err) && !strings.Contains(err.Error(), "process already finished") { // It's okay if the task is already gone
					log.WithError(err).WithFields(logFields).Warn("failed to delete task during force remove, proceeding with container delete")
				}
			}
		} else if !errdefs.IsNotFound(err) { // Error other than task not found
			return errors.Wrapf(err, "failed to get task for force remove of container %s", cont.ID())
		}
		// If task not found, it's already stopped, proceed to delete container.
	}

	var deleteOpts []containerd.DeleteOpts
	if volumes {
		log.WithFields(logFields).Debug("removing container with snapshot cleanup (volumes=true)")
		deleteOpts = append(deleteOpts, containerd.WithSnapshotCleanup)
	}

	if err := cont.Delete(ctx, deleteOpts...); err != nil {
		if errdefs.IsNotFound(err) {
			log.WithFields(logFields).Info("container already removed")
			return nil
		}
		// If 'force' was not used and the container is running, Delete will fail.
		// containerd error: "container is running" or "task must be deleted"
		if strings.Contains(err.Error(), "container is running") || strings.Contains(err.Error(), "task must be deleted") {
			if force { // Should have been handled above, but as a safeguard
				log.WithError(err).WithFields(logFields).Error("failed to delete container even with force (task likely still exists)")
			} else {
				log.WithError(err).WithFields(logFields).Warn("failed to delete container: it is likely running or task not deleted (try with force=true)")
			}
		}
		return errors.Wrapf(err, "failed to delete container %s", cont.ID())
	}

	log.WithFields(logFields).Info("container removed successfully")
	return nil
}

// Ensure containerdClient implements Client at compile time.
var _ Client = (*containerdClient)(nil)

// PauseContainer pauses the main process of a containerd container.
func (c *containerdClient) PauseContainer(ctx context.Context, pumbaContainer *Container, dryrun bool) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	logFields := log.Fields{
		"id":     pumbaContainer.ID(),
		"name":   pumbaContainer.Name(),
		"dryrun": dryrun,
	}
	log.WithFields(logFields).Info("pausing containerd container")

	if dryrun {
		return nil
	}

	cont, err := c.client.LoadContainer(ctx, pumbaContainer.ID())
	if err != nil {
		if errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "container %s not found for pause", pumbaContainer.ID())
		}
		return errors.Wrapf(err, "failed to load container %s for pause", pumbaContainer.ID())
	}

	task, err := cont.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			log.WithFields(logFields).Warn("task not found for container (cannot pause, likely not running)")
			return errors.Wrap(err, "task not found, container may not be running")
		}
		return errors.Wrapf(err, "failed to get task for container %s", cont.ID())
	}

	status, err := task.Status(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get task status for container %s before pausing", cont.ID())
	}

	if status.Status == containerd.Paused || status.Status == containerd.Pausing {
		log.WithFields(logFields).Info("container is already paused or pausing")
		return nil
	}
	if status.Status != containerd.Running {
		log.WithFields(logFields).Warnf("cannot pause container in state %s", status.Status)
		return fmt.Errorf("cannot pause container in state %s", status.Status)
	}

	if err := task.Pause(ctx); err != nil {
		return errors.Wrapf(err, "failed to pause task for container %s", cont.ID())
	}

	log.WithFields(logFields).Info("container paused successfully")
	return nil
}

// UnpauseContainer unpauses the main process of a containerd container.
func (c *containerdClient) UnpauseContainer(ctx context.Context, pumbaContainer *Container, dryrun bool) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	logFields := log.Fields{
		"id":     pumbaContainer.ID(),
		"name":   pumbaContainer.Name(),
		"dryrun": dryrun,
	}
	log.WithFields(logFields).Info("unpausing containerd container")

	if dryrun {
		return nil
	}

	cont, err := c.client.LoadContainer(ctx, pumbaContainer.ID())
	if err != nil {
		if errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "container %s not found for unpause", pumbaContainer.ID())
		}
		return errors.Wrapf(err, "failed to load container %s for unpause", pumbaContainer.ID())
	}

	task, err := cont.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			log.WithFields(logFields).Warn("task not found for container (cannot unpause, likely not running or paused)")
			return errors.Wrap(err, "task not found, container may not be running or paused")
		}
		return errors.Wrapf(err, "failed to get task for container %s", cont.ID())
	}

	status, err := task.Status(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get task status for container %s before unpausing", cont.ID())
	}

	if status.Status == containerd.Running {
		log.WithFields(logFields).Info("container is already running")
		return nil
	}
	// Allow unpausing from Paused or Pausing state.
	if status.Status != containerd.Paused && status.Status != containerd.Pausing {
		log.WithFields(logFields).Warnf("cannot unpause container in state %s", status.Status)
		return fmt.Errorf("cannot unpause container in state %s", status.Status)
	}

	if err := task.Resume(ctx); err != nil {
		return errors.Wrapf(err, "failed to resume task for container %s", cont.ID())
	}

	log.WithFields(logFields).Info("container unpaused successfully")
	return nil
}

// ExecContainer executes a command in a running containerd container.
func (c *containerdClient) ExecContainer(ctx context.Context, pumbaContainer *Container, command string, args []string, dryrun bool) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	fullCommand := command
	if len(args) > 0 {
		fullCommand = command + " " + strings.Join(args, " ")
	}
	logFields := log.Fields{
		"id":      pumbaContainer.ID(),
		"name":    pumbaContainer.Name(),
		"command": fullCommand,
		"dryrun":  dryrun,
	}
	log.WithFields(logFields).Info("executing command in containerd container")

	if dryrun {
		return nil
	}

	cont, err := c.client.LoadContainer(ctx, pumbaContainer.ID())
	if err != nil {
		return errors.Wrapf(err, "failed to load container %s for exec", pumbaContainer.ID())
	}

	task, err := cont.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "task not found for container %s (not running?) for exec", pumbaContainer.ID())
		}
		return errors.Wrapf(err, "failed to get task for container %s for exec", pumbaContainer.ID())
	}

	// Ensure task is running before trying to exec
	taskStatus, err := task.Status(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get task status for container %s before exec", cont.ID())
	}
	if taskStatus.Status != containerd.Running {
		return fmt.Errorf("cannot exec in container %s: task status is %s, not running", cont.ID(), taskStatus.Status)
	}

	// Prepare spec for the new process
	containerSpec, err := cont.Spec(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get container spec for %s for exec", pumbaContainer.ID())
	}
	// Use OCI process spec as a base for Cwd, Env, User, etc.
	ociProcessSpec := containerSpec.Process
	if ociProcessSpec == nil {
		// Fallback if Process is not set in the main spec (should be rare for runnable containers)
		ociProcessSpec = &oci.Process{Cwd: "/", User: oci.User{UID: 0, GID: 0}} // Basic root default
	}

	execProcessSpec := &containerd.ProcessSpec{
		Args: append([]string{command}, args...),
		Cwd:  ociProcessSpec.Cwd, // Use Cwd from container spec
		Env:  ociProcessSpec.Env, // Use Env from container spec
		User: &ociProcessSpec.User,
		// Terminal: false, // Pumba typically runs non-interactive commands
	}
	// If ociProcessSpec.User is nil or empty, provide a default (root)
	if execProcessSpec.User == nil || (execProcessSpec.User.UID == 0 && execProcessSpec.User.GID == 0 && execProcessSpec.User.Username == "") {
		execProcessSpec.User = &oci.User{UID: 0, GID: 0}
	}

	execID := uuid.New().String()

	// Setup stdio for capturing output
	// Using pipes to capture stdout and stderr
	stdoutReadPipe, stdoutWritePipe, err := os.Pipe()
	if err != nil {
		return errors.Wrap(err, "failed to create stdout pipe for exec")
	}
	defer stdoutReadPipe.Close() // Close reader in this function

	stderrReadPipe, stderrWritePipe, err := os.Pipe()
	if err != nil {
		stdoutWritePipe.Close() // Close the already opened pipe
		return errors.Wrap(err, "failed to create stderr pipe for exec")
	}
	defer stderrReadPipe.Close() // Close reader in this function

	// Goroutine to capture stdout
	var execStdout, execStderr strings.Builder
	stdoutCaptureDone := make(chan struct{})
	go func() {
		defer close(stdoutCaptureDone)
		defer stdoutWritePipe.Close() // Close writer when done reading from it in task
		if _, errCopy := io.Copy(&execStdout, stdoutReadPipe); errCopy != nil {
			log.WithError(errCopy).WithFields(logFields).Warn("error copying stdout from exec process")
		}
	}()

	// Goroutine to capture stderr
	stderrCaptureDone := make(chan struct{})
	go func() {
		defer close(stderrCaptureDone)
		defer stderrWritePipe.Close() // Close writer when done reading from it in task
		if _, errCopy := io.Copy(&execStderr, stderrReadPipe); errCopy != nil {
			log.WithError(errCopy).WithFields(logFields).Warn("error copying stderr from exec process")
		}
	}()

	// Create the cio.FIFOSet with the pipes. Stdin is nil as Pumba doesn't provide input.
	ioCreator := cio.NewCreator(cio.WithStreams(nil, stdoutWritePipe, stderrWritePipe))

	process, err := task.Exec(ctx, execID, execProcessSpec, ioCreator)
	if err != nil {
		// stdoutWritePipe.Close() and stderrWritePipe.Close() will be closed by the capture goroutines
		// if the goroutines started. If Exec fails before goroutines start, they might not be closed.
		// However, the defer on read pipes will eventually lead to writers being unblocked if they were used.
		return errors.Wrapf(err, "failed to create exec process %s in container %s", execID, pumbaContainer.ID())
	}

	// Start the process
	if err := process.Start(ctx); err != nil {
		return errors.Wrapf(err, "failed to start exec process %s in container %s", execID, pumbaContainer.ID())
	}

	// Wait for the process to exit
	status, err := process.Wait(ctx)
	if err != nil {
		// process.Delete might be needed even if Wait fails
		if _, delErr := process.Delete(ctx); delErr != nil && !errdefs.IsNotFound(delErr) {
			log.WithError(delErr).WithFields(logFields).Warnf("failed to delete exec process %s after wait error", execID)
		}
		return errors.Wrapf(err, "failed to wait for exec process %s in container %s", execID, pumbaContainer.ID())
	}

	// Ensure output capture is complete
	// Closing the write pipes explicitly here ensures the io.Copy goroutines will finish.
	// If they were not closed by the time the process exited, this is a final signal.
	stdoutWritePipe.Close()
	stderrWritePipe.Close()
	<-stdoutCaptureDone
	<-stderrCaptureDone

	// Delete the process
	if _, err := process.Delete(ctx); err != nil {
		// Log error but don't fail the command if delete fails, as command execution might have succeeded.
		if !errdefs.IsNotFound(err) { // Don't log if already deleted or gone
			log.WithError(err).WithFields(logFields).Warnf("failed to delete exec process %s", execID)
		}
	}

	log.WithFields(logFields).WithField("stdout", execStdout.String()).WithField("stderr", execStderr.String()).Debug("exec command output")

	if status.ExitStatus() != 0 {
		errMsg := fmt.Sprintf("exec command '%s' failed in container %s with exit code %d",
			fullCommand, pumbaContainer.ID(), status.ExitStatus())
		if execStderr.Len() > 0 {
			errMsg = fmt.Sprintf("%s: %s", errMsg, execStderr.String())
		} else if execStdout.Len() > 0 { // Add stdout if stderr is empty, for more context
			errMsg = fmt.Sprintf("%s: %s", errMsg, execStdout.String())
		}
		return errors.New(errMsg)
	}

	log.WithFields(logFields).Infof("exec command completed successfully with exit code %d", status.ExitStatus())
	return nil
}

// StartContainer ensures a task is running for the given container.
// If a task exists and is paused, it's resumed.
// If a task exists and is stopped/exited, it's deleted and a new one is created and started.
// If no task exists, a new one is created and started.
func (c *containerdClient) StartContainer(ctx context.Context, pumbaContainer *Container, dryrun bool) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	logFields := log.Fields{
		"id":     pumbaContainer.ID(),
		"name":   pumbaContainer.Name(),
		"dryrun": dryrun,
	}
	log.WithFields(logFields).Info("starting containerd container")

	if dryrun {
		return nil
	}

	cont, err := c.client.LoadContainer(ctx, pumbaContainer.ID())
	if err != nil {
		return errors.Wrapf(err, "failed to load container %s for start", pumbaContainer.ID())
	}

	// Attempt to get existing task
	task, err := cont.Task(ctx, nil)
	if err == nil { // Task exists
		status, statusErr := task.Status(ctx)
		if statusErr != nil {
			return errors.Wrapf(statusErr, "failed to get status for existing task of container %s", cont.ID())
		}

		log.WithFields(logFields).Debugf("existing task found with status: %s", status.Status)

		switch status.Status {
		case containerd.Running:
			log.WithFields(logFields).Info("container is already running")
			return nil
		case containerd.Paused, containerd.Pausing:
			log.WithFields(logFields).Info("container task is paused, resuming")
			if err := task.Resume(ctx); err != nil {
				return errors.Wrapf(err, "failed to resume paused task for container %s", cont.ID())
			}
			log.WithFields(logFields).Info("container resumed successfully")
			return nil
		case containerd.Created:
			log.WithFields(logFields).Info("container task is created, starting it")
			if err := task.Start(ctx); err != nil {
				return errors.Wrapf(err, "failed to start created task for container %s", cont.ID())
			}
			log.WithFields(logFields).Info("container started successfully")
			return nil
		case containerd.Stopped, containerd.Unknown: // Includes Exited which is a form of Stopped
			log.WithFields(logFields).Infof("task found in state %s, deleting and creating a new one", status.Status)
			if _, err := task.Delete(ctx); err != nil {
				if !errdefs.IsNotFound(err) && !strings.Contains(err.Error(), "process already finished") {
					return errors.Wrapf(err, "failed to delete existing task in %s state for container %s", status.Status, cont.ID())
				}
			}
			// Proceed to create a new task (fallthrough to logic below)
		default: // Any other state, treat as needing a fresh start.
			log.WithFields(logFields).Infof("task found in unhandled state %s, attempting to delete and create a new one", status.Status)
			if _, err := task.Delete(ctx); err != nil {
				if !errdefs.IsNotFound(err) && !strings.Contains(err.Error(), "process already finished") {
					log.WithError(err).WithFields(logFields).Warnf("failed to delete task in %s state, new task creation might fail", status.Status)
				}
			}
			// Proceed to create a new task
		}
	} else if !errdefs.IsNotFound(err) { // Error other than "not found"
		return errors.Wrapf(err, "failed to get task for container %s", cont.ID())
	}
	// If err is IsNotFound or we fell through from deleting an old task: create and start a new task.
	log.WithFields(logFields).Info("no running/paused task found, creating and starting a new task")

	// We don't need the full OCI spec just to create a task from an existing container.
	// cont.NewTask will use the container's existing spec.
	// For attaching IO: using NullIO as Pumba's Start doesn't typically interact with stdio.
	// If specific IO handling is needed later, this can be changed.
	// Example: ioCreator := cio.NewCreator(cio.WithStdio) or custom FIFOs.
	ioCreator := cio.NullIO // Creates /dev/null FIFOs if not already present.

	newTask, err := cont.NewTask(ctx, ioCreator)
	if err != nil {
		return errors.Wrapf(err, "failed to create new task for container %s", cont.ID())
	}

	if err := newTask.Start(ctx); err != nil {
		// Attempt to clean up the task if Start fails
		if _, delErr := newTask.Delete(ctx); delErr != nil && !errdefs.IsNotFound(delErr) {
			log.WithError(delErr).WithFields(logFields).Warnf("failed to delete new task after start failure for %s", cont.ID())
		}
		return errors.Wrapf(err, "failed to start new task for container %s", cont.ID())
	}

	log.WithFields(logFields).Info("new task started successfully for container")
	return nil
}

// RestartContainer restarts a containerd container.
// It attempts to stop the container gracefully, then forcefully if needed, and then starts it again.
func (c *containerdClient) RestartContainer(ctx context.Context, pumbaContainer *Container, timeout time.Duration, dryrun bool) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	logFields := log.Fields{
		"id":      pumbaContainer.ID(),
		"name":    pumbaContainer.Name(),
		"timeout": timeout,
		"dryrun":  dryrun,
	}
	log.WithFields(logFields).Info("restarting containerd container")

	if dryrun {
		// In dryrun, StopContainer and StartContainer will also respect dryrun.
		// So, we can call them to log their intended actions.
		log.WithFields(logFields).Info("[dryrun] intending to stop container as part of restart")
		//nolint:errcheck // best effort in dryrun
		c.StopContainer(ctx, pumbaContainer, int(timeout.Seconds()), true)
		log.WithFields(logFields).Info("[dryrun] intending to start container as part of restart")
		//nolint:errcheck // best effort in dryrun
		c.StartContainer(ctx, pumbaContainer, true)
		return nil
	}

	// Stop the container
	// The existing StopContainer handles various states (running, paused, stopped, not found task)
	// and includes its own logging.
	log.WithFields(logFields).Debug("attempting to stop container as part of restart")
	if err := c.StopContainer(ctx, pumbaContainer, int(timeout.Seconds()), false); err != nil {
		// Log the error but proceed to StartContainer, as the container might be in a state
		// where starting is still possible/desirable (e.g., already stopped, or stop failed uncleanly).
		// StopContainer already logs detailed errors.
		log.WithError(err).WithFields(logFields).Warn("error during stop phase of restart, attempting to start anyway")
	} else {
		log.WithFields(logFields).Info("container stopped successfully as part of restart")
	}

	// Start the container
	// The existing StartContainer handles various states (no task, existing task stopped/paused/running)
	// and includes its own logging.
	log.WithFields(logFields).Debug("attempting to start container as part of restart")
	if err := c.StartContainer(ctx, pumbaContainer, false); err != nil {
		return errors.Wrapf(err, "failed to start container %s during restart", pumbaContainer.ID())
	}

	log.WithFields(logFields).Info("container restarted successfully")
	return nil
}

// StopContainerWithID stops a containerd container identified by its ID.
// It fetches the necessary container information and then calls the main StopContainer logic.
func (c *containerdClient) StopContainerWithID(ctx context.Context, containerID string, timeout time.Duration, dryrun bool) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	logFields := log.Fields{
		"id":      containerID,
		"timeout": timeout,
		"dryrun":  dryrun,
	}
	log.WithFields(logFields).Info("stopping containerd container by ID")

	if dryrun {
		// Construct a dummy container for dry-run logging within StopContainer
		dummyContainer := &Container{
			Cid:   containerID,
			Cname: containerID, // Name might not be known in dry run without a lookup
		}
		log.WithFields(logFields).Info("[dryrun] intending to stop container (details will be logged by StopContainer)")
		return c.StopContainer(ctx, dummyContainer, int(timeout.Seconds()), true)
	}

	cont, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			log.WithError(err).WithFields(logFields).Warn("container not found for StopContainerWithID")
			// Docker client's StopContainerWithID doesn't return an error if not found, just logs.
			// However, returning an error might be cleaner for the caller to understand state.
			// For consistency with typical Pumba behavior of not erroring on "not found" for stop/remove,
			// we can return nil here. Let's return wrapped error for now.
			return errors.Wrapf(err, "container %s not found", containerID)
		}
		return errors.Wrapf(err, "failed to load container %s for StopContainerWithID", containerID)
	}

	info, err := cont.Info(ctx, containerd.WithoutRuntime())
	if err != nil {
		// If info cannot be fetched, we might still proceed with ID only,
		// but StopSignal would be default.
		log.WithError(err).WithFields(logFields).Warn("failed to get container info for StopContainerWithID, stop signal might be default")
		// Construct a minimal container object even if info fails
		minimalPumbaContainer := &Container{
			Cid:   containerID,
			Cname: containerID, // Fallback name
		}
		return c.StopContainer(ctx, minimalPumbaContainer, int(timeout.Seconds()), false)
	}

	containerName := containerID // Default to ID
	if name, ok := info.Labels[oci.AnnotationName]; ok && name != "" {
		containerName = name
	} else if name, ok := info.Labels["io.kubernetes.cri.container-name"]; ok && name != "" {
		containerName = name
	} else if name, ok := info.Labels["name"]; ok && name != "" {
		containerName = name
	}

	// Construct the Pumba Container object needed by StopContainer
	pumbaC := &Container{
		Cid:        containerID,
		Cname:      containerName,
		Clabels:    info.Labels,
		CimageName: info.Image, // Image name might not be strictly needed for stop, but good to have
		// Cstatus will be determined by StopContainer if needed
	}
	pumbaC.CstopSignal = pumbaC.Labels()[signalLabel]

	log.WithFields(logFields).Infof("calling StopContainer for container %s (resolved name: %s)", containerID, pumbaC.Name())
	return c.StopContainer(ctx, pumbaC, int(timeout.Seconds()), false)
}

// runNetworkCmdHelperContainer is a helper function to run network commands (tc or iptables)
// in a temporary helper container that shares the network namespace of the target container.
func (c *containerdClient) runNetworkCmdHelperContainer(
	originalCtx context.Context, // Use a separate context for helper container operations if main ctx can be short-lived
	targetPumbaContainer *Container,
	helperImageName string,
	commandsToRun [][]string, // Each inner slice is a command and its arguments
	pullImage bool,
	dryrun bool,
) error {
	ctx := namespaces.WithNamespace(originalCtx, c.namespace)
	helperLogFields := log.Fields{
		"target_id":    targetPumbaContainer.ID(),
		"target_name":  targetPumbaContainer.Name(),
		"helper_image": helperImageName,
		"dryrun":       dryrun,
	}
	log.WithFields(helperLogFields).Info("preparing to run network commands in helper container")

	if dryrun {
		for i, cmd := range commandsToRun {
			log.WithFields(helperLogFields).Infof("[dryrun] would execute command %d: %s", i+1, strings.Join(cmd, " "))
		}
		return nil
	}

	// 1. Load target container and get its task
	targetCont, err := c.client.LoadContainer(ctx, targetPumbaContainer.ID())
	if err != nil {
		return errors.Wrapf(err, "failed to load target container %s", targetPumbaContainer.ID())
	}
	targetTask, err := targetCont.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "target container %s is not running (no task found)", targetPumbaContainer.ID())
		}
		return errors.Wrapf(err, "failed to get task for target container %s", targetPumbaContainer.ID())
	}
	targetTaskStatus, err := targetTask.Status(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get status for target container task %s", targetPumbaContainer.ID())
	}
	if targetTaskStatus.Status != containerd.Running && targetTaskStatus.Status != containerd.Paused {
		// Paused is okay as network namespace exists, though commands effect might be delayed until unpause.
		return fmt.Errorf("target container %s task is not running or paused (status: %s)", targetPumbaContainer.ID(), targetTaskStatus.Status)
	}

	// 2. Pull helper image if requested
	if pullImage {
		log.WithFields(helperLogFields).Infof("pulling helper image %s", helperImageName)
		if _, err := c.client.Pull(ctx, helperImageName, containerd.WithPullUnpack); err != nil {
			// Try with default platform if it fails - useful for some registries/images
			log.WithError(err).WithFields(helperLogFields).Warnf("failed to pull %s, trying with default platform", helperImageName)
			if _, err = c.client.Pull(ctx, helperImageName, containerd.WithPullUnpack, containerd.WithDefaultPlatform()); err != nil {
				return errors.Wrapf(err, "failed to pull helper image %s", helperImageName)
			}
		}
		log.WithFields(helperLogFields).Infof("helper image %s pulled successfully", helperImageName)
	}
	// Ensure image is actually available after pull (or if pullImage was false)
	img, err := c.client.GetImage(ctx, helperImageName)
	if err != nil {
		return errors.Wrapf(err, "helper image %s not found after pull attempt or pull=false", helperImageName)
	}

	// 3. Create Helper Container Spec & Container
	helperContainerID := "pumba-nethelper-" + targetPumbaContainer.ID() + "-" + uuid.New().String()[:8]
	helperSnapshotID := "pumba-nethelper-snapshot-" + helperContainerID

	log.WithFields(helperLogFields).WithField("helper_id", helperContainerID).Info("creating helper container spec")

	// Using WithNewSpec and oci.WithImageConfig to get defaults from the image.
	// The helper container will just sleep; commands are run via exec.
	helperSpecOpts := []oci.SpecOpts{
		oci.WithImageConfig(img), // Apply image's default config (entrypoint, cmd, env, etc.)
		oci.WithHostname(helperContainerID),
		oci.WithAddedCapabilities([]string{
			specs.LinuxCapabilityNetAdmin, // For tc and iptables
			specs.LinuxCapabilityNetRaw,   // Often needed alongside NetAdmin
		}),
		// Override command to sleep, as we'll use exec.
		oci.WithProcessArgs("sleep", "300"), // Keep helper alive for a bit
		// Add Pumba skip label
		oci.WithAnnotations(map[string]string{pumbaSkipLabel: trueValue}),
	}

	// Create the helper container
	helperCont, err := c.client.NewContainer(
		ctx,
		helperContainerID,
		containerd.WithNewSnapshot(helperSnapshotID, img),
		containerd.WithSpec(helperSpecOpts...),
	)
	if err != nil {
		return errors.Wrapf(err, "failed to create helper container %s", helperContainerID)
	}

	// Defer cleanup of the helper container and its snapshot
	defer func() {
		log.WithFields(helperLogFields).WithField("helper_id", helperContainerID).Info("cleaning up helper container")
		if delErr := helperCont.Delete(originalCtx, containerd.WithSnapshotCleanup); delErr != nil {
			log.WithError(delErr).WithFields(helperLogFields).WithField("helper_id", helperContainerID).Warn("failed to delete helper container")
		}
	}()

	// 4. Create and Start Helper Task (joining target's network namespace)
	log.WithFields(helperLogFields).WithField("helper_id", helperContainerID).Info("creating and starting helper task")
	// Use cio.NullIO as we are not interacting with the main 'sleep' task's stdio
	helperTask, err := helperCont.NewTask(ctx, cio.NullIO, containerd.WithTaskNetwork(targetTask.Pid()))
	if err != nil {
		return errors.Wrapf(err, "failed to create task for helper container %s", helperContainerID)
	}

	// Defer cleanup of the helper task
	defer func() {
		log.WithFields(helperLogFields).WithField("helper_id", helperContainerID).Info("deleting helper task")
		if _, delErr := helperTask.Delete(originalCtx, containerd.WithKill); delErr != nil { // containerd.WithKill ensures task is killed if running
			if !errdefs.IsNotFound(delErr) { // Don't log if already gone
				log.WithError(delErr).WithFields(helperLogFields).WithField("helper_id", helperContainerID).Warn("failed to delete helper task")
			}
		}
	}()

	if err := helperTask.Start(ctx); err != nil {
		return errors.Wrapf(err, "failed to start task for helper container %s", helperContainerID)
	}
	log.WithFields(helperLogFields).WithField("helper_id", helperContainerID).Info("helper task started")

	// 5. Execute commands in the helper container's task
	for i, cmdAndArgs := range commandsToRun {
		if len(cmdAndArgs) == 0 {
			log.WithFields(helperLogFields).Warnf("command %d is empty, skipping", i+1)
			continue
		}
		cmd := cmdAndArgs[0]
		args := cmdAndArgs[1:]
		execLogFields := helperLogFields
		execLogFields["command"] = strings.Join(cmdAndArgs, " ")

		log.WithFields(execLogFields).Infof("executing command %d in helper container", i+1)

		execID := "exec-" + helperContainerID + "-" + uuid.New().String()[:8]

		// Create process spec for exec
		execSpec := &containerd.ProcessSpec{
			Args: cmdAndArgs,
			Cwd:  "/",                       // Default CWD for commands
			User: &oci.User{UID: 0, GID: 0}, // Run as root
			// Terminal: false, // Default
		}

		// Setup stdio for capturing output of exec
		var stdoutBuf, stderrBuf strings.Builder
		stdoutRead, stdoutWrite, _ := os.Pipe()
		stderrRead, stderrWrite, _ := os.Pipe()

		execIOCreator := cio.NewCreator(cio.WithStreams(nil, stdoutWrite, stderrWrite))

		execProcess, err := helperTask.Exec(ctx, execID, execSpec, execIOCreator)
		if err != nil {
			stdoutRead.Close()
			stdoutWrite.Close()
			stderrRead.Close()
			stderrWrite.Close()
			return errors.Wrapf(err, "failed to create exec process for command '%s'", cmd)
		}

		var wg sync.WaitGroup
		wg.Add(2) // For stdout and stderr copying

		go func() {
			defer wg.Done()
			defer stdoutRead.Close()
			defer stdoutWrite.Close()
			io.Copy(&stdoutBuf, stdoutRead)
		}()
		go func() {
			defer wg.Done()
			defer stderrRead.Close()
			defer stderrWrite.Close()
			io.Copy(&stderrBuf, stderrRead)
		}()

		if err := execProcess.Start(ctx); err != nil {
			// Ensure pipes are closed if start fails before goroutines manage them fully
			stdoutWrite.Close()
			stderrWrite.Close()
			wg.Wait() // Wait for any partial copy to finish
			return errors.Wrapf(err, "failed to start exec process for command '%s'", cmd)
		}

		// Close our ends of the write pipes so io.Copy can complete
		stdoutWrite.Close()
		stderrWrite.Close()
		wg.Wait() // Wait for stdout/stderr copying to complete

		status, err := execProcess.Wait(ctx)
		if err != nil {
			log.WithError(err).WithFields(execLogFields).Errorf("failed waiting for exec process for command '%s'", cmd)
			// Continue to delete, then return error
		}

		if _, delErr := execProcess.Delete(ctx); delErr != nil && !errdefs.IsNotFound(delErr) {
			log.WithError(delErr).WithFields(execLogFields).Warnf("failed to delete exec process %s", execID)
		}

		if err != nil { // From Wait()
			return errors.Wrapf(err, "error waiting for command '%s': stdout: %s, stderr: %s", cmd, stdoutBuf.String(), stderrBuf.String())
		}
		if status.ExitStatus() != 0 {
			errMsg := fmt.Sprintf("command '%s' failed with exit code %d", strings.Join(cmdAndArgs, " "), status.ExitStatus())
			if stdoutBuf.Len() > 0 {
				errMsg += fmt.Sprintf("\nstdout: %s", stdoutBuf.String())
			}
			if stderrBuf.Len() > 0 {
				errMsg += fmt.Sprintf("\nstderr: %s", stderrBuf.String())
			}
			log.WithFields(execLogFields).Error(errMsg)
			return errors.New(errMsg)
		}
		log.WithFields(execLogFields).Infof("command %d completed successfully. Stdout: %s, Stderr: %s", i+1, stdoutBuf.String(), stderrBuf.String())
	}

	log.WithFields(helperLogFields).Info("all network commands executed successfully in helper container")
	return nil
}

// NetemContainer applies network emulation rules using tc in a helper container.
func (c *containerdClient) NetemContainer(ctx context.Context, pumbaCont *Container, netInterface string, netemCmd []string,
	ips []*net.IPNet, sports, dports []string, duration time.Duration, tcimage string, pull, dryrun bool) error {
	logFields := log.Fields{
		"id":           pumbaCont.ID(),
		"name":         pumbaCont.Name(),
		"netInterface": netInterface,
		"command":      strings.Join(netemCmd, " "),
		"ips":          ips,
		"sports":       sports,
		"dports":       dports,
		"duration":     duration, // Duration is for caller's scheduling, not directly used here
		"tc-image":     tcimage,
		"pull":         pull,
		"dryrun":       dryrun,
	}
	log.WithFields(logFields).Info("setting netem on container using containerd helper")

	var tcCommands [][]string
	if len(ips) == 0 && len(sports) == 0 && len(dports) == 0 {
		// Simple case: tc qdisc add dev <netInterface> root netem <netemCmd>
		cmd := append([]string{"tc", "qdisc", "add", "dev", netInterface, "root", "netem"}, netemCmd...)
		tcCommands = append(tcCommands, cmd)
	} else {
		// Complex case with IP/port filtering (similar to dockerClient.startNetemContainerIPFilter)
		// This creates a prio qdisc and filters traffic to specific bands.
		tcCommands = append(tcCommands,
			[]string{"tc", "qdisc", "add", "dev", netInterface, "root", "handle", "1:", "prio"},
			[]string{"tc", "qdisc", "add", "dev", netInterface, "parent", "1:1", "handle", "10:", "sfq"},                                // band 0
			[]string{"tc", "qdisc", "add", "dev", netInterface, "parent", "1:2", "handle", "20:", "sfq"},                                // band 1
			append(append([]string{"tc", "qdisc", "add", "dev", netInterface, "parent", "1:3", "handle", "30:", "netem"}, netemCmd...)), // band 2 with netem
		)
		// Add filters to route traffic to band 2 (1:3)
		for _, ip := range ips {
			tcCommands = append(tcCommands, []string{"tc", "filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "dst", ip.String(), "flowid", "1:3"})
		}
		for _, sport := range sports {
			tcCommands = append(tcCommands, []string{"tc", "filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "sport", sport, "0xffff", "flowid", "1:3"})
		}
		for _, dport := range dports {
			tcCommands = append(tcCommands, []string{"tc", "filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
				"u32", "match", "ip", "dport", dport, "0xffff", "flowid", "1:3"})
		}
	}

	return c.runNetworkCmdHelperContainer(ctx, pumbaCont, tcimage, tcCommands, pull, dryrun)
}

// StopNetemContainer removes network emulation rules using tc in a helper container.
func (c *containerdClient) StopNetemContainer(ctx context.Context, pumbaCont *Container, netInterface string,
	ips []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) error {
	logFields := log.Fields{
		"id":           pumbaCont.ID(),
		"name":         pumbaCont.Name(),
		"netInterface": netInterface,
		"ips":          ips,
		"sports":       sports,
		"dports":       dports,
		"tc-image":     tcimage,
		"pull":         pull,
		"dryrun":       dryrun,
	}
	log.WithFields(logFields).Info("stopping netem on container using containerd helper")

	var tcCommands [][]string
	if len(ips) == 0 && len(sports) == 0 && len(dports) == 0 {
		// Simple case: tc qdisc del dev <netInterface> root netem
		tcCommands = append(tcCommands, []string{"tc", "qdisc", "del", "dev", netInterface, "root", "netem"})
	} else {
		// Complex case: remove the prio qdisc and all its filters/children
		// Deleting the root prio qdisc '1:' should clean up everything associated with it.
		// Simpler than deleting individual filters and child qdiscs.
		tcCommands = append(tcCommands, []string{"tc", "qdisc", "del", "dev", netInterface, "root", "handle", "1:", "prio"})
		// As a fallback or more explicit cleanup, one might list and delete filters and specific qdiscs
		// like in dockerClient, but deleting the root prio qdisc is generally sufficient.
		// tcCommands = [][]string{
		// 	{"tc", "qdisc", "del", "dev", netInterface, "parent", "1:1", "handle", "10:"},
		// 	{"tc", "qdisc", "del", "dev", netInterface, "parent", "1:2", "handle", "20:"},
		// 	{"tc", "qdisc", "del", "dev", netInterface, "parent", "1:3", "handle", "30:"},
		// 	{"tc", "qdisc", "del", "dev", netInterface, "root", "handle", "1:", "prio"},
		// }
		// Note: Filters are implicitly removed when the qdisc they belong to (1:0) is removed.
	}

	return c.runNetworkCmdHelperContainer(ctx, pumbaCont, tcimage, tcCommands, pull, dryrun)
}

// IPTablesContainer is a placeholder
func (c *containerdClient) IPTablesContainer(ctx context.Context, pumbaCont *Container, cmdPrefix, cmdSuffix []string,
	srcIPs, dstIPs []*net.IPNet, sports, dports []string, duration time.Duration, image string, pull, dryrun bool) error {
	logFields := log.Fields{
		"id":            pumbaCont.ID(),
		"name":          pumbaCont.Name(),
		"cmdPrefix":     cmdPrefix,
		"cmdSuffix":     cmdSuffix,
		"srcIPs":        srcIPs,
		"dstIPs":        dstIPs,
		"sports":        sports,
		"dports":        dports,
		"duration":      duration, // Duration is for caller's scheduling
		"iptablesImage": image,
		"pull":          pull,
		"dryrun":        dryrun,
	}
	log.WithFields(logFields).Info("setting iptables rules on container using containerd helper")

	var iptablesCommands [][]string
	baseCmd := append(cmdPrefix, "-w", "5") // Add wait option to iptables

	if len(srcIPs) == 0 && len(dstIPs) == 0 && len(sports) == 0 && len(dports) == 0 {
		cmd := baseCmd
		cmd = append(cmd, cmdSuffix...)
		iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
	} else {
		// Iterate and create commands for each IP/port filter criteria
		for _, ip := range srcIPs {
			cmd := baseCmd
			cmd = append(cmd, "-s", ip.String())
			cmd = append(cmd, cmdSuffix...)
			iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
		}
		for _, ip := range dstIPs {
			cmd := baseCmd
			cmd = append(cmd, "-d", ip.String())
			cmd = append(cmd, cmdSuffix...)
			iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
		}
		for _, sport := range sports {
			cmd := baseCmd
			// Docker client uses "--sport", but iptables might take "-p tcp --sport" or similar.
			// Assuming cmdPrefix might contain protocol, or it's a general match.
			// For simplicity, directly using --sport. Adjust if protocol needed.
			cmd = append(cmd, "--sport", sport)
			cmd = append(cmd, cmdSuffix...)
			iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
		}
		for _, dport := range dports {
			cmd := baseCmd
			cmd = append(cmd, "--dport", dport)
			cmd = append(cmd, cmdSuffix...)
			iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
		}
	}
	if len(iptablesCommands) == 0 {
		log.WithFields(logFields).Warn("no iptables commands generated, check filter criteria")
		return nil // Or an error if this state is unexpected
	}

	return c.runNetworkCmdHelperContainer(ctx, pumbaCont, image, iptablesCommands, pull, dryrun)
}

// StressContainer applies stress to the target container using a helper stress-ng container.
// It attempts to place the helper container into the same cgroup as the target container.
func (c *containerdClient) StressContainer(
	originalCtx context.Context, // Renamed to avoid conflict with ctx used for namespace
	pumbaTargetContainer *Container,
	stressors []string,
	helperImageName string,
	pull bool,
	duration time.Duration,
	dryrun bool,
) (string, <-chan string, <-chan error, error) {
	ctx := namespaces.WithNamespace(originalCtx, c.namespace) // ctx for most operations

	logFields := log.Fields{
		"target_id":    pumbaTargetContainer.ID(),
		"target_name":  pumbaTargetContainer.Name(),
		"stressors":    strings.Join(stressors, " "),
		"helper_image": helperImageName,
		"duration":     duration,
		"pull":         pull,
		"dryrun":       dryrun,
	}
	log.WithFields(logFields).Info("applying stress to container using containerd helper")

	// Prepare channels
	outputChan := make(chan string)
	errChan := make(chan error, 1) // Buffered to allow sending error without immediate receive

	if dryrun {
		go func() {
			log.WithFields(logFields).Info("[dryrun] StressContainer would proceed.")
			close(outputChan)
			close(errChan)
		}()
		return "pumba-stress-dryrun-" + uuid.NewString()[:8], outputChan, errChan, nil
	}

	// 1. Load target container and ensure it's running (has a task)
	targetCont, err := c.client.LoadContainer(ctx, pumbaTargetContainer.ID())
	if err != nil {
		err = errors.Wrapf(err, "failed to load target container %s", pumbaTargetContainer.ID())
		go func() { errChan <- err; close(outputChan); close(errChan) }()
		return "", outputChan, errChan, err
	}
	targetTask, err := targetCont.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			err = errors.Wrapf(err, "target container %s is not running (no task found)", pumbaTargetContainer.ID())
		} else {
			err = errors.Wrapf(err, "failed to get task for target container %s", pumbaTargetContainer.ID())
		}
		go func() { errChan <- err; close(outputChan); close(errChan) }()
		return "", outputChan, errChan, err
	}
	targetTaskStatus, err := targetTask.Status(ctx)
	if err != nil {
		err = errors.Wrapf(err, "failed to get status for target container task %s", pumbaTargetContainer.ID())
		go func() { errChan <- err; close(outputChan); close(errChan) }()
		return "", outputChan, errChan, err
	}
	if targetTaskStatus.Status != containerd.Running {
		err = fmt.Errorf("target container %s task is not running (status: %s)", pumbaTargetContainer.ID(), targetTaskStatus.Status)
		go func() { errChan <- err; close(outputChan); close(errChan) }()
		return "", outputChan, errChan, err
	}

	// 2. Pull helper image if requested
	var img containerd.Image // Declared here to be in scope for OCI spec creation
	if pull {
		log.WithFields(logFields).Infof("pulling helper image %s", helperImageName)
		pulledImg, pullErr := c.client.Pull(ctx, helperImageName, containerd.WithPullUnpack)
		if pullErr != nil {
			log.WithError(pullErr).WithFields(logFields).Warnf("failed to pull %s, trying with default platform", helperImageName)
			pulledImg, pullErr = c.client.Pull(ctx, helperImageName, containerd.WithPullUnpack, containerd.WithDefaultPlatform())
			if pullErr != nil {
				err = errors.Wrapf(pullErr, "failed to pull helper image %s", helperImageName)
				go func() { errChan <- err; close(outputChan); close(errChan) }()
				return "", outputChan, errChan, err
			}
		}
		img = pulledImg
		log.WithFields(logFields).Infof("helper image %s pulled successfully", helperImageName)
	} else {
		loadedImg, loadErr := c.client.GetImage(ctx, helperImageName)
		if loadErr != nil {
			err = errors.Wrapf(loadErr, "helper image %s not found locally (pull=false)", helperImageName)
			go func() { errChan <- err; close(outputChan); close(errChan) }()
			return "", outputChan, errChan, err
		}
		img = loadedImg
	}

	// 3. Get target container's CgroupsPath
	targetSpec, err := targetCont.Spec(ctx)
	if err != nil {
		err = errors.Wrapf(err, "failed to get OCI spec for target container %s", targetCont.ID())
		go func() { errChan <- err; close(outputChan); close(errChan) }()
		return "", outputChan, errChan, err
	}
	if targetSpec == nil || targetSpec.Linux == nil || targetSpec.Linux.CgroupsPath == "" {
		// This should ideally not happen for a running container managed by containerd.
		err = fmt.Errorf("could not determine cgroups path for target container %s", targetCont.ID())
		go func() { errChan <- err; close(outputChan); close(errChan) }()
		return "", outputChan, errChan, err
	}
	targetCgroupPath := targetSpec.Linux.CgroupsPath
	log.WithFields(logFields).Infof("target container cgroup path: %s", targetCgroupPath)

	// 4. Prepare helper container details
	helperContainerID := "pumba-stress-" + pumbaTargetContainer.ID() + "-" + uuid.NewString()[:8]
	helperSnapshotID := "pumba-stress-snapshot-" + helperContainerID

	// Construct stress-ng command
	stressCmd := []string{"stress-ng"}
	if duration > 0 {
		stressCmd = append(stressCmd, "--timeout", fmt.Sprintf("%ds", int(duration.Seconds())))
	}
	stressCmd = append(stressCmd, "--verbose") // For more output
	stressCmd = append(stressCmd, stressors...)

	// Goroutine to manage the helper container lifecycle and stream IO
	go func() {
		defer close(outputChan)
		defer close(errChan)

		// Create context for this goroutine, can be originalCtx or a new one for cleanup
		goroutineCtx := namespaces.WithNamespace(originalCtx, c.namespace) // Use originalCtx for cleanup tasks too

		helperSpecOpts := []oci.SpecOpts{
			oci.WithImageConfig(img), // Apply image's default config
			oci.WithHostname(helperContainerID),
			oci.WithProcessArgs(stressCmd...),          // Set stress-ng as the command
			oci.WithLinuxCgroupsPath(targetCgroupPath), // CRITICAL: Place helper in target's cgroup
			oci.WithAddedCapabilities([]string{ // Capabilities needed by stress-ng
				specs.LinuxCapabilitySYSAdmin, // Often needed for various stressors
				specs.LinuxCapabilityKill,     // For some stressors like --zombie
				// Add other capabilities as identified by stress-ng documentation if specific stressors fail
			}),
			oci.WithMounts([]specs.Mount{ // Mount cgroupfs to allow stress-ng to read it
				{
					Destination: "/sys/fs/cgroup",
					Type:        "cgroup",         // Or "bind" if source is /sys/fs/cgroup from host
					Source:      "/sys/fs/cgroup", // This assumes cgroupfs is at /sys/fs/cgroup on the host
					Options:     []string{"ro", "nosuid", "noexec", "nodev"},
				},
			}),
			oci.WithAnnotations(map[string]string{pumbaSkipLabel: trueValue}),
		}

		// Create the helper container
		helperCont, createErr := c.client.NewContainer(
			goroutineCtx,
			helperContainerID,
			containerd.WithNewSnapshot(helperSnapshotID, img),
			containerd.WithSpec(helperSpecOpts...),
			// No WithContainerRemoveOnExit, manual cleanup is more robust with task
		)
		if createErr != nil {
			errChan <- errors.Wrapf(createErr, "failed to create stress helper container %s", helperContainerID)
			return
		}

		// Defer helper container deletion with snapshot cleanup
		defer func() {
			deleteCtx := context.Background() // Use fresh context for cleanup
			deleteCtx = namespaces.WithNamespace(deleteCtx, c.namespace)
			log.WithFields(logFields).WithField("helper_id", helperContainerID).Info("deleting stress helper container and snapshot")
			if delErr := helperCont.Delete(deleteCtx, containerd.WithSnapshotCleanup); delErr != nil {
				log.WithError(delErr).WithFields(logFields).WithField("helper_id", helperContainerID).Warn("failed to delete stress helper container")
			}
		}()

		// Setup stdio for capturing output from stress-ng
		var stdoutBuffer, stderrBuffer strings.Builder
		stdoutRead, stdoutWrite, _ := os.Pipe()
		stderrRead, stderrWrite, _ := os.Pipe()

		ioCreator := cio.NewCreator(cio.WithStreams(nil, stdoutWrite, stderrWrite))

		// Create and start the helper task
		helperTask, taskErr := helperCont.NewTask(goroutineCtx, ioCreator, containerd.WithTaskDeleteOnExit()) // Auto-delete task on exit
		if taskErr != nil {
			errChan <- errors.Wrapf(taskErr, "failed to create task for stress helper %s", helperContainerID)
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); defer stdoutRead.Close(); io.Copy(&stdoutBuffer, stdoutRead) }()
		go func() { defer wg.Done(); defer stderrRead.Close(); io.Copy(&stderrBuffer, stderrRead) }()

		if startErr := helperTask.Start(goroutineCtx); startErr != nil {
			stdoutWrite.Close()
			stderrWrite.Close() // Ensure copiers can exit
			wg.Wait()
			errChan <- errors.Wrapf(startErr, "failed to start task for stress helper %s", helperContainerID)
			return
		}
		// Close our ends of write pipes once task is started
		stdoutWrite.Close()
		stderrWrite.Close()

		log.WithFields(logFields).WithField("helper_id", helperContainerID).WithField("helper_task_pid", helperTask.Pid()).Info("stress helper task started")

		// Wait for the task to complete
		status, waitErr := helperTask.Wait(goroutineCtx)
		wg.Wait() // Ensure all IO is copied before proceeding

		if waitErr != nil {
			errChan <- errors.Wrapf(waitErr, "error waiting for stress helper task %s. Stdout: %s, Stderr: %s",
				helperContainerID, stdoutBuffer.String(), stderrBuffer.String())
			return
		}

		exitCode := status.ExitStatus()
		outputCombined := "Stdout:\n" + stdoutBuffer.String() + "\nStderr:\n" + stderrBuffer.String()
		log.WithFields(logFields).WithField("helper_id", helperContainerID).Infof("stress helper task exited with code %d. Output:\n%s", exitCode, outputCombined)

		if exitCode != 0 {
			errChan <- fmt.Errorf("stress-ng helper %s exited with code %d: %s", helperContainerID, exitCode, outputCombined)
		} else {
			// Send combined output to the output channel on success
			outputChan <- outputCombined
		}
	}()

	return helperContainerID, outputChan, errChan, nil
}

// StopIPTablesContainer removes iptables rules using a helper container.
// Note: It essentially re-applies the commands with "-D" (delete) instead of "-A" (append) or "-I" (insert).
// This requires that cmdPrefix is structured to allow easy replacement of -A/-I with -D.
func (c *containerdClient) StopIPTablesContainer(ctx context.Context, pumbaCont *Container, cmdPrefix, cmdSuffix []string,
	srcIPs, dstIPs []*net.IPNet, sports, dports []string, image string, pull, dryrun bool) error {

	// Create a new cmdPrefix for deletion by replacing -A or -I with -D
	deleteCmdPrefix := make([]string, len(cmdPrefix))
	copied := false
	for i, part := range cmdPrefix {
		if (strings.EqualFold(part, "-A") || strings.EqualFold(part, "-I")) && !copied {
			deleteCmdPrefix[i] = "-D"
			copied = true // ensure only first occurrence is replaced if multiple exist by mistake
		} else {
			deleteCmdPrefix[i] = part
		}
	}
	// If no -A or -I was found, this won't work as expected. Caller must provide correct prefix for adding rules.
	if !copied {
		log.WithField("cmdPrefix", cmdPrefix).Warn("StopIPTablesContainer could not find -A or -I in cmdPrefix to replace with -D. Rule deletion might fail.")
		// Depending on strictness, one might return an error here.
		// For now, try to proceed; maybe the prefix is already for deletion or is more complex.
	}

	logFields := log.Fields{
		"id":              pumbaCont.ID(),
		"name":            pumbaCont.Name(),
		"deleteCmdPrefix": deleteCmdPrefix,
		"cmdSuffix":       cmdSuffix,
		"srcIPs":          srcIPs,
		"dstIPs":          dstIPs,
		"sports":          sports,
		"dports":          dports,
		"iptablesImage":   image,
		"pull":            pull,
		"dryrun":          dryrun,
	}
	log.WithFields(logFields).Info("stopping iptables rules on container using containerd helper")

	// The logic for generating commands is identical to IPTablesContainer, just with deleteCmdPrefix
	var iptablesCommands [][]string
	baseCmd := append(deleteCmdPrefix, "-w", "5") // Add wait option

	if len(srcIPs) == 0 && len(dstIPs) == 0 && len(sports) == 0 && len(dports) == 0 {
		cmd := baseCmd
		cmd = append(cmd, cmdSuffix...)
		iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
	} else {
		for _, ip := range srcIPs {
			cmd := baseCmd
			cmd = append(cmd, "-s", ip.String())
			cmd = append(cmd, cmdSuffix...)
			iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
		}
		for _, ip := range dstIPs {
			cmd := baseCmd
			cmd = append(cmd, "-d", ip.String())
			cmd = append(cmd, cmdSuffix...)
			iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
		}
		for _, sport := range sports {
			cmd := baseCmd
			cmd = append(cmd, "--sport", sport)
			cmd = append(cmd, cmdSuffix...)
			iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
		}
		for _, dport := range dports {
			cmd := baseCmd
			cmd = append(cmd, "--dport", dport)
			cmd = append(cmd, cmdSuffix...)
			iptablesCommands = append(iptablesCommands, append([]string{"iptables"}, cmd...))
		}
	}
	if len(iptablesCommands) == 0 {
		log.WithFields(logFields).Warn("no iptables stop commands generated, check filter criteria")
		return nil
	}

	return c.runNetworkCmdHelperContainer(ctx, pumbaCont, image, iptablesCommands, pull, dryrun)
}
