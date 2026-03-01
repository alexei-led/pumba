package containerd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
)

// cgroupReader reads the cgroup file for a process. Overrideable in tests.
var cgroupReader = func(pid uint32) ([]byte, error) {
	return os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
}

// sidecarExec creates a short-lived sidecar container that shares the target
// container's network namespace and runs the given command+args inside it.
func (c *containerdClient) sidecarExec(ctx context.Context, target *ctr.Container, sidecarImage string, pull bool, command string, argsList [][]string) error {
	ctx = c.nsCtx(ctx)

	if pull {
		if err := c.pullImage(ctx, sidecarImage); err != nil {
			return fmt.Errorf("failed to pull sidecar image %s: %w", sidecarImage, err)
		}
	}

	targetTask, err := c.getTask(ctx, target.ID())
	if err != nil {
		return fmt.Errorf("failed to get target task for sidecar: %w", err)
	}
	targetPID := targetTask.Pid()
	if targetPID == 0 {
		return fmt.Errorf("target task for %s has PID 0 (not running)", target.ID())
	}

	image, err := c.client.GetImage(ctx, sidecarImage)
	if err != nil {
		return fmt.Errorf("failed to get sidecar image %s: %w", sidecarImage, err)
	}

	sidecarID := fmt.Sprintf("pumba-sidecar-%d", execCounter.Add(1))
	sidecarContainer, err := c.client.NewContainer(ctx, sidecarID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(sidecarID+"-snapshot", image),
		containerd.WithNewSpec(
			oci.WithImageConfig(image),
			oci.WithProcessArgs("sleep", "infinity"),
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.NetworkNamespace,
				Path: fmt.Sprintf("/proc/%d/ns/net", targetPID),
			}),
			oci.WithCapabilities(networkCapabilities),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create sidecar container: %w", err)
	}

	// Use context.WithoutCancel so cleanup succeeds even if the parent ctx is canceled.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarCleanupTimeout)
		defer cancel()
		cleanupCtx = c.nsCtx(cleanupCtx)
		if cleanupErr := c.cleanupSidecar(cleanupCtx, sidecarContainer); cleanupErr != nil {
			log.WithError(cleanupErr).Warn("failed to clean up sidecar container")
		}
	}()

	task, err := sidecarContainer.NewTask(ctx, cio.NullIO)
	if err != nil {
		return fmt.Errorf("failed to create sidecar task: %w", err)
	}
	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start sidecar task: %w", err)
	}

	for _, args := range argsList {
		if err := c.runSidecarCmd(ctx, task, command, args); err != nil {
			return err
		}
	}

	return nil
}

// runSidecarCmd executes a single command inside a running sidecar task.
func (c *containerdClient) runSidecarCmd(ctx context.Context, task containerd.Task, command string, args []string) error {
	cmdArgs := make([]string, 0, 1+len(args))
	cmdArgs = append(cmdArgs, command)
	cmdArgs = append(cmdArgs, args...)
	execID := fmt.Sprintf("pumba-sc-exec-%d", execCounter.Add(1))
	pspec := &specs.Process{
		Args: cmdArgs,
		Cwd:  "/",
		Env:  []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		User: specs.User{UID: 0, GID: 0},
		Capabilities: &specs.LinuxCapabilities{
			Effective:   networkCapabilities,
			Bounding:    networkCapabilities,
			Permitted:   networkCapabilities,
			Inheritable: networkCapabilities,
		},
	}

	if err := execTask(ctx, task, pspec, execID, fmt.Sprintf("sidecar exec '%s'", strings.Join(cmdArgs, " "))); err != nil {
		return err
	}
	log.WithField("args", strings.Join(args, " ")).Debug("sidecar exec completed")
	return nil
}

// pullImage pulls an image via containerd.
func (c *containerdClient) pullImage(ctx context.Context, ref string) error {
	log.WithField("image", ref).Debug("pulling image via containerd")
	_, err := c.client.Pull(ctx, ref, containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	return nil
}

const (
	sidecarCleanupTimeout = 30 * time.Second
	sidecarKillTimeout    = 5 * time.Second
)

// networkCapabilities are the Linux capabilities required for network manipulation.
var networkCapabilities = []string{"CAP_NET_ADMIN", "CAP_NET_RAW"}

// resolveCgroupPath parses /proc/<pid>/cgroup and returns the target's cgroup path
// and its parent directory. On cgroups v2, expects "0::<path>". On cgroups v1,
// falls back to the "memory" subsystem path.
func resolveCgroupPath(pid uint32) (cgroupPath, cgroupParent string, err error) {
	data, err := cgroupReader(pid)
	if err != nil {
		return "", "", fmt.Errorf("failed to read cgroup for PID %d: %w", pid, err)
	}
	var v1MemoryPath string
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		const cgroupFields = 3
		parts := strings.SplitN(line, ":", cgroupFields)
		if len(parts) != 3 || parts[2] == "/" {
			continue
		}
		// cgroups v2: hierarchy 0, empty subsystem list
		if parts[0] == "0" && parts[1] == "" {
			cgroupPath = parts[2]
			break
		}
		// cgroups v1: look for memory subsystem as representative
		if v1MemoryPath == "" && strings.Contains(parts[1], "memory") {
			v1MemoryPath = parts[2]
		}
	}
	if cgroupPath == "" {
		cgroupPath = v1MemoryPath
	}
	if cgroupPath == "" {
		return "", "", fmt.Errorf("could not parse cgroup path for PID %d (content length: %d bytes)", pid, len(data))
	}
	cgroupParent = "/"
	if lastSlash := strings.LastIndex(cgroupPath, "/"); lastSlash > 0 {
		cgroupParent = cgroupPath[:lastSlash]
	}
	return cgroupPath, cgroupParent, nil
}

// buildStressSpecOpts builds OCI spec options for the stress sidecar container.
// For inject-cgroup mode (uses cgroupPath): runs /cg-inject with host cgroupns and /sys/fs/cgroup bind mount.
// For default sidecar mode (uses cgroupParent): runs /stress-ng directly, placed under target's cgroup parent.
func buildStressSpecOpts(image containerd.Image, stressors []string, cgroupPath, cgroupParent string, injectCgroup bool) []oci.SpecOpts {
	if injectCgroup {
		prefix := []string{"/cg-inject", "--cgroup-path", cgroupPath, "--", "/stress-ng"}
		args := make([]string, 0, len(prefix)+len(stressors))
		args = append(args, prefix...)
		args = append(args, stressors...)
		return []oci.SpecOpts{
			oci.WithImageConfig(image),
			oci.WithProcessArgs(args...),
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.CgroupNamespace,
				Path: "/proc/1/ns/cgroup",
			}),
			oci.WithMounts([]specs.Mount{
				{
					Type:        "bind",
					Source:      "/sys/fs/cgroup",
					Destination: "/sys/fs/cgroup",
					Options:     []string{"bind", "rw"},
				},
			}),
		}
	}
	args := make([]string, 0, len(stressors)+1)
	args = append(args, "/stress-ng")
	args = append(args, stressors...)
	return []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithProcessArgs(args...),
		oci.WithCgroup(cgroupParent),
	}
}

// stressSidecar creates a long-lived sidecar container running stress-ng (or cg-inject)
// as its main process. Returns the sidecar ID and output/error channels. A goroutine waits
// for the task to exit and performs full cleanup (task delete + container/snapshot removal).
func (c *containerdClient) stressSidecar(
	ctx context.Context,
	target *ctr.Container,
	sidecarImage string,
	stressors []string,
	injectCgroup bool,
	pull bool,
) (string, <-chan string, <-chan error, error) {
	ctx = c.nsCtx(ctx)

	sidecarID, sidecarContainer, task, waitCh, err := c.createStressSidecar(ctx, target, sidecarImage, stressors, injectCgroup, pull)
	if err != nil {
		return "", nil, nil, err
	}

	outCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go c.waitStressSidecar(ctx, sidecarID, sidecarContainer, task, waitCh, outCh, errCh)

	return sidecarID, outCh, errCh, nil
}

// createStressSidecar handles the setup: resolve target cgroup, pull image, create and start the sidecar.
func (c *containerdClient) createStressSidecar(
	ctx context.Context,
	target *ctr.Container,
	sidecarImage string,
	stressors []string,
	injectCgroup bool,
	pull bool,
) (string, containerd.Container, containerd.Task, <-chan containerd.ExitStatus, error) {
	targetTask, err := c.getTask(ctx, target.ID())
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to get target task for stress sidecar: %w", err)
	}
	targetPID := targetTask.Pid()
	if targetPID == 0 {
		return "", nil, nil, nil, fmt.Errorf("target task for %s has PID 0 (not running)", target.ID())
	}

	cgroupPath, cgroupParent, err := resolveCgroupPath(targetPID)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to resolve cgroup for stress sidecar: %w", err)
	}

	if pull {
		if err := c.pullImage(ctx, sidecarImage); err != nil {
			return "", nil, nil, nil, fmt.Errorf("failed to pull stress image: %w", err)
		}
	}
	log.WithFields(log.Fields{
		"target":        target.ID(),
		"pid":           targetPID,
		"cgroup-path":   cgroupPath,
		"cgroup-parent": cgroupParent,
		"inject-cgroup": injectCgroup,
	}).Debug("resolved target cgroup for stress sidecar")

	image, err := c.client.GetImage(ctx, sidecarImage)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to get stress image %s: %w", sidecarImage, err)
	}

	specOpts := buildStressSpecOpts(image, stressors, cgroupPath, cgroupParent, injectCgroup)
	sidecarID := fmt.Sprintf("pumba-stress-%d", execCounter.Add(1))

	sidecarContainer, err := c.client.NewContainer(ctx, sidecarID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(sidecarID+"-snapshot", image),
		containerd.WithNewSpec(specOpts...),
		containerd.WithContainerLabels(map[string]string{"com.gaiaadm.pumba.skip": "true"}),
	)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to create stress sidecar: %w", err)
	}

	task, waitCh, err := c.startSidecarTask(ctx, sidecarContainer)
	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarCleanupTimeout)
		defer cancel()
		c.deleteContainer(c.nsCtx(cleanupCtx), sidecarContainer)
		return "", nil, nil, nil, err
	}

	return sidecarID, sidecarContainer, task, waitCh, nil
}

// startSidecarTask creates, registers wait, and starts a task on the given container.
func (c *containerdClient) startSidecarTask(ctx context.Context, cntr containerd.Container) (containerd.Task, <-chan containerd.ExitStatus, error) {
	task, err := cntr.NewTask(ctx, cio.NullIO)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stress task: %w", err)
	}

	waitCh, err := task.Wait(ctx)
	if err != nil {
		_, _ = task.Delete(ctx)
		return nil, nil, fmt.Errorf("failed to set up stress task wait: %w", err)
	}

	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(ctx)
		return nil, nil, fmt.Errorf("failed to start stress task: %w", err)
	}

	return task, waitCh, nil
}

// waitStressSidecar waits for the stress task to exit, performs cleanup, and reports the result.
func (c *containerdClient) waitStressSidecar(
	ctx context.Context,
	sidecarID string,
	sidecarContainer containerd.Container,
	task containerd.Task,
	waitCh <-chan containerd.ExitStatus,
	outCh chan<- string,
	errCh chan<- error,
) {
	defer close(outCh)
	defer close(errCh)

	var status containerd.ExitStatus
	var ok bool
	select {
	case status, ok = <-waitCh:
	case <-ctx.Done():
		// Context canceled — force-kill the task so waitCh unblocks
		if killErr := task.Kill(context.WithoutCancel(ctx), syscall.SIGKILL); killErr != nil && !errdefs.IsNotFound(killErr) {
			log.WithError(killErr).WithField("id", sidecarID).Warn("failed to kill stress task")
		}
		killTimer := time.NewTimer(sidecarKillTimeout)
		defer killTimer.Stop()
		select {
		case status, ok = <-waitCh:
		case <-killTimer.C:
			log.WithField("id", sidecarID).Warn("timed out waiting for stress task after kill")
		}
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarCleanupTimeout)
	defer cancel()
	cleanupCtx = c.nsCtx(cleanupCtx)
	if _, delErr := task.Delete(cleanupCtx); delErr != nil && !errdefs.IsNotFound(delErr) {
		log.WithError(delErr).WithField("id", sidecarID).Warn("failed to delete stress task")
	}
	c.deleteContainer(cleanupCtx, sidecarContainer)

	if !ok {
		errCh <- fmt.Errorf("stress sidecar %s: wait channel closed unexpectedly", sidecarID)
		return
	}
	code, _, exitErr := status.Result()
	if exitErr != nil {
		errCh <- fmt.Errorf("stress sidecar error: %w", exitErr)
		return
	}
	if code != 0 {
		errCh <- fmt.Errorf("stress sidecar exited with code %d", code)
		return
	}
	outCh <- sidecarID
}

// deleteContainer removes a sidecar container and its snapshot.
// The caller is responsible for providing a properly scoped context (e.g. with timeout and namespace).
func (c *containerdClient) deleteContainer(ctx context.Context, cntr containerd.Container) {
	if err := cntr.Delete(ctx, containerd.WithSnapshotCleanup); err != nil && !errdefs.IsNotFound(err) {
		log.WithError(err).Warn("failed to clean up sidecar container")
	}
}

// cleanupSidecar kills the task and removes the sidecar container and its snapshot.
func (c *containerdClient) cleanupSidecar(ctx context.Context, cntr containerd.Container) error {
	task, err := cntr.Task(ctx, nil)
	if err == nil {
		waitCh, waitErr := task.Wait(ctx)
		_ = task.Kill(ctx, syscall.SIGKILL)
		if waitErr == nil {
			killTimer := time.NewTimer(sidecarKillTimeout)
			defer killTimer.Stop()
			select {
			case <-waitCh:
			case <-killTimer.C:
			}
		}
		_, _ = task.Delete(ctx)
	} else if !errdefs.IsNotFound(err) {
		log.WithError(err).Warn("failed to get sidecar task for cleanup")
	}

	if err := cntr.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to delete sidecar container: %w", err)
		}
	}
	return nil
}
