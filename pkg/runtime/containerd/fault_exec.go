package containerd

import (
	"context"
	"errors"
	"fmt"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// faultExecTimeout bounds only an individual mutating tc/iptables exec, not
// experiment duration, sidecar setup, inspection, user exec or stress. Caller
// cancellation cannot let rollback overtake an in-flight fault script.
const faultExecTimeout = 2 * time.Minute

func execFaultTaskOutput(ctx context.Context, task containerd.Task, pspec *specs.Process, execID, description string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	return execFaultTaskOutputWithTimeout(ctx, task, pspec, execID, description, faultExecTimeout)
}

func execFaultTaskOutputWithTimeout(ctx context.Context, task containerd.Task, pspec *specs.Process, execID, description string, timeout time.Duration) (string, string, error) {
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	out, errOut, err := runTaskExec(execCtx, task, pspec, execID, description, true)
	if execCtx.Err() != nil {
		return "", "", fmt.Errorf("fault exec exceeded its internal deadline; shell descendants may require manual cleanup: %w", errors.Join(execCtx.Err(), err))
	}
	return out, errOut, err
}

func (c *containerdClient) execFaultInContainer(ctx context.Context, containerID, command string, args []string) error {
	_, _, err := c.execContainerCommand(ctx, containerID, command, args, execFaultTaskOutput)
	return err
}
