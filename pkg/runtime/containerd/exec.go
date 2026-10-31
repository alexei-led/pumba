package containerd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
)

// execTaskOutput runs a command inside a containerd task, waits for
// completion and returns its stdout. Handles the full exec lifecycle: create,
// wait, start, collect exit status, delete. A non-zero exit is an error
// carrying stderr.
func execTaskOutput(ctx context.Context, task containerd.Task, pspec *specs.Process, execID, description string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	execProcess, err := task.Exec(ctx, execID, pspec, cio.NewCreator(
		cio.WithStreams(nil, &stdout, &stderr),
	))
	if err != nil {
		return "", "", fmt.Errorf("failed to exec %s: %w", description, err)
	}
	defer func() {
		if _, delErr := execProcess.Delete(ctx); delErr != nil {
			log.WithError(delErr).WithField("exec", description).Warn("failed to delete exec process")
		}
	}()

	exitCh, err := execProcess.Wait(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to wait on %s: %w", description, err)
	}

	if err := execProcess.Start(ctx); err != nil {
		return "", "", fmt.Errorf("failed to start %s: %w", description, err)
	}

	select {
	case status := <-exitCh:
		code, _, err := status.Result()
		if err != nil {
			return "", "", fmt.Errorf("%s failed: %w", description, err)
		}
		drainErr := waitIO(ctx, execProcess, ioDrainTimeout)
		if drainErr != nil && !errors.Is(drainErr, errIncompleteOutput) {
			return "", "", fmt.Errorf("%s canceled: %w", description, drainErr)
		}
		if code != 0 {
			return "", "", fmt.Errorf("%s exited with code %d: %s", description, code, stderr.String())
		}
		if drainErr != nil {
			return "", "", fmt.Errorf("%s: %w", description, drainErr)
		}
		return stdout.String(), stderr.String(), nil
	case <-ctx.Done():
		return "", "", fmt.Errorf("%s canceled: %w", description, ctx.Err())
	}
}

// ioDrainTimeout bounds how long execTaskOutput waits for the exec's output
// after the process exits. A background child that inherits stdout keeps the
// FIFOs open past exit, so EOF may never come.
const ioDrainTimeout = 2 * time.Second

var errIncompleteOutput = errors.New("incomplete exec output: timed out draining stdout/stderr")

// waitIO waits until the exec's output is fully copied into the buffers; the
// exit status can arrive before the copy finishes. After timeout (or ctx
// cancel) it aborts the copy and waits for it to stop. The buffers are then
// safe to read, but incomplete output must not be used as inspection evidence.
func waitIO(ctx context.Context, p containerd.Process, timeout time.Duration) error {
	pio := p.IO()
	if pio == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		pio.Wait()
		close(done)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		pio.Cancel()
		<-done
		return errIncompleteOutput
	case <-ctx.Done():
		pio.Cancel()
		<-done
		return ctx.Err()
	}
}

// execUserCommand preserves arbitrary user commands that exit with a child
// still holding stdout open. Fault execution and inspection must instead fail
// on incomplete captured output; neither may treat a prefix as complete.
func (c *containerdClient) execUserCommand(ctx context.Context, containerID, command string, args []string) error {
	err := c.execInContainer(ctx, containerID, command, args)
	if errors.Is(err, errIncompleteOutput) {
		return nil
	}
	return err
}

func (c *containerdClient) execInContainer(ctx context.Context, containerID, command string, args []string) error {
	_, _, err := c.execInContainerOutput(ctx, containerID, command, args)
	return err
}

// execInContainerOutput is execInContainer returning the command's stdout.
func (c *containerdClient) execInContainerOutput(ctx context.Context, containerID, command string, args []string) (string, string, error) {
	task, err := c.getTask(ctx, containerID)
	if err != nil {
		return "", "", err
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

	return execTaskOutput(ctx, task, pspec, execID, fmt.Sprintf("exec in %s '%s'", containerID, strings.Join(cmdArgs, " ")))
}
