package containerd

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
)

// execCounter generates unique IDs for exec and sidecar operations.
var execCounter atomic.Uint64

// signalMap maps signal names to syscall signals.
var signalMap = map[string]syscall.Signal{
	"SIGABRT": syscall.SIGABRT,
	"SIGALRM": syscall.SIGALRM,
	"SIGCONT": syscall.SIGCONT,
	"SIGHUP":  syscall.SIGHUP,
	"SIGINT":  syscall.SIGINT,
	"SIGKILL": syscall.SIGKILL,
	"SIGPIPE": syscall.SIGPIPE,
	"SIGQUIT": syscall.SIGQUIT,
	"SIGSTOP": syscall.SIGSTOP,
	"SIGTERM": syscall.SIGTERM,
	"SIGTRAP": syscall.SIGTRAP,
	"SIGUSR1": syscall.SIGUSR1,
	"SIGUSR2": syscall.SIGUSR2,
}

func parseSignal(signal string) (syscall.Signal, error) {
	if num, err := strconv.Atoi(signal); err == nil {
		if num < 1 {
			return 0, fmt.Errorf("invalid signal number: %d", num)
		}
		return syscall.Signal(num), nil
	}
	signal = strings.ToUpper(signal)
	if !strings.HasPrefix(signal, "SIG") {
		signal = "SIG" + signal
	}
	if sig, ok := signalMap[signal]; ok {
		return sig, nil
	}
	return 0, fmt.Errorf("unknown signal: %s", signal)
}

// killTimeout is the maximum time to wait for SIGKILL to take effect.
const killTimeout = 30 * time.Second

func (c *containerdClient) getTask(ctx context.Context, containerID string) (containerd.Task, error) {
	cntr, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to load container %s: %w", containerID, err)
	}
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get task for %s: %w", containerID, err)
	}
	return task, nil
}

func (c *containerdClient) stopTask(ctx context.Context, containerID string, signal syscall.Signal, timeout time.Duration) error {
	task, err := c.getTask(ctx, containerID)
	if err != nil {
		return err
	}
	waitCh, err := task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait on task %s: %w", containerID, err)
	}
	if err := task.Kill(ctx, signal); err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to send %s to %s: %w", signal, containerID, err)
		}
	}
	stopTimer := time.NewTimer(timeout)
	defer stopTimer.Stop()
	select {
	case <-waitCh:
		_, _ = task.Delete(ctx)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context canceled waiting for %s on %s: %w", signal, containerID, ctx.Err())
	case <-stopTimer.C:
		log.WithField("id", containerID).Debug("graceful stop timeout, sending SIGKILL")
		if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
			if !errdefs.IsNotFound(err) {
				return fmt.Errorf("failed to send SIGKILL to %s: %w", containerID, err)
			}
		}
		killTimer := time.NewTimer(killTimeout)
		defer killTimer.Stop()
		select {
		case <-waitCh:
			_, _ = task.Delete(ctx)
			return nil
		case <-ctx.Done():
			return fmt.Errorf("context canceled waiting for SIGKILL on %s: %w", containerID, ctx.Err())
		case <-killTimer.C:
			return fmt.Errorf("timeout waiting for SIGKILL on %s", containerID)
		}
	}
}

func (c *containerdClient) killTask(ctx context.Context, containerID, signal string) error {
	task, err := c.getTask(ctx, containerID)
	if err != nil {
		return err
	}
	sig, err := parseSignal(signal)
	if err != nil {
		return fmt.Errorf("invalid signal for %s: %w", containerID, err)
	}
	if err := task.Kill(ctx, sig); err != nil {
		return fmt.Errorf("failed to kill task %s: %w", containerID, err)
	}
	return nil
}

func (c *containerdClient) startTask(ctx context.Context, containerID string) error {
	cntr, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container %s: %w", containerID, err)
	}
	task, err := cntr.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to get task for %s: %w", containerID, err)
		}
		task, err = cntr.NewTask(ctx, cio.NullIO)
		if err != nil {
			return fmt.Errorf("failed to create task for %s: %w", containerID, err)
		}
	}
	return task.Start(ctx)
}

func (c *containerdClient) pauseTask(ctx context.Context, containerID string) error {
	task, err := c.getTask(ctx, containerID)
	if err != nil {
		return err
	}
	return task.Pause(ctx)
}

func (c *containerdClient) resumeTask(ctx context.Context, containerID string) error {
	task, err := c.getTask(ctx, containerID)
	if err != nil {
		return err
	}
	return task.Resume(ctx)
}

// execTask runs a command inside a containerd task and waits for completion.
// Handles the full exec lifecycle: create, wait, start, collect exit status, delete.
func execTask(ctx context.Context, task containerd.Task, pspec *specs.Process, execID, description string) error {
	var stdout, stderr bytes.Buffer
	execProcess, err := task.Exec(ctx, execID, pspec, cio.NewCreator(
		cio.WithStreams(nil, &stdout, &stderr),
	))
	if err != nil {
		return fmt.Errorf("failed to exec %s: %w", description, err)
	}
	defer func() {
		if _, delErr := execProcess.Delete(ctx); delErr != nil {
			log.WithError(delErr).WithField("exec", description).Warn("failed to delete exec process")
		}
	}()

	exitCh, err := execProcess.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait on %s: %w", description, err)
	}

	if err := execProcess.Start(ctx); err != nil {
		return fmt.Errorf("failed to start %s: %w", description, err)
	}

	select {
	case status := <-exitCh:
		code, _, err := status.Result()
		if err != nil {
			return fmt.Errorf("%s failed: %w", description, err)
		}
		if code != 0 {
			return fmt.Errorf("%s exited with code %d: %s", description, code, stderr.String())
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s canceled: %w", description, ctx.Err())
	}
}

func (c *containerdClient) execInContainer(ctx context.Context, containerID, command string, args []string) error {
	task, err := c.getTask(ctx, containerID)
	if err != nil {
		return err
	}

	cmdArgs := make([]string, 0, 1+len(args))
	cmdArgs = append(cmdArgs, command)
	cmdArgs = append(cmdArgs, args...)
	execID := fmt.Sprintf("pumba-exec-%d", execCounter.Add(1))
	pspec := &specs.Process{
		Args: cmdArgs,
		Cwd:  "/",
		Env:  []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		User: specs.User{UID: 0, GID: 0},
	}

	return execTask(ctx, task, pspec, execID, fmt.Sprintf("exec in %s '%s'", containerID, strings.Join(cmdArgs, " ")))
}
