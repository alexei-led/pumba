package containerd

import (
	"context"
	"fmt"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
)

// toContainer converts a containerd container to the runtime-agnostic Container type.
// If all is false, only running containers (those with an active task) are included.
// Returns (container, skip, error) where skip=true means the container should be filtered out.
func toContainer(ctx context.Context, c containerd.Container, all bool) (*ctr.Container, bool, error) {
	info, err := c.Info(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get container info for %s: %w", c.ID(), err)
	}

	state := ctr.StateExited
	task, err := c.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, false, fmt.Errorf("failed to get task for %s: %w", c.ID(), err)
		}
		if !all {
			return nil, true, nil
		}
	} else {
		status, err := task.Status(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get task status for %s: %w", c.ID(), err)
		}
		switch status.Status { //nolint:exhaustive // only Running needs special handling
		case containerd.Running:
			state = ctr.StateRunning
		default:
			if !all {
				return nil, true, nil
			}
		}
	}

	labels := info.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	return &ctr.Container{
		ContainerID:   c.ID(),
		ContainerName: c.ID(),
		Image:         info.Image,
		ImageID:       info.Image,
		State:         state,
		Labels:        labels,
		Networks:      make(map[string]ctr.NetworkLink),
	}, false, nil
}
