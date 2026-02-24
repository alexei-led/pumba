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
		if status.Status == containerd.Running {
			state = ctr.StateRunning
		} else if !all {
			return nil, true, nil
		}
	}

	return &ctr.Container{
		ContainerID:   c.ID(),
		ContainerName: resolveContainerName(c.ID(), info.Labels),
		Image:         info.Image,
		ImageID:       info.Image,
		State:         state,
		Labels:        info.Labels,
		Networks:      make(map[string]ctr.NetworkLink),
	}, false, nil
}

// resolveContainerName tries to extract a human-readable name from well-known
// container labels. Falls back to the container ID if no name label is found.
//
// Supported label sources (checked in priority order):
//   - Kubernetes: io.kubernetes.container.name (+ pod name + namespace)
//   - nerdctl:    nerdctl/name
//   - Docker:     com.docker.compose.service
func resolveContainerName(id string, labels map[string]string) string {
	if name := labels["io.kubernetes.container.name"]; name != "" {
		pod := labels["io.kubernetes.pod.name"]
		ns := labels["io.kubernetes.pod.namespace"]
		if pod != "" && ns != "" {
			return ns + "/" + pod + "/" + name
		}
		if pod != "" {
			return pod + "/" + name
		}
		return name
	}
	if name := labels["nerdctl/name"]; name != "" {
		return name
	}
	if name := labels["com.docker.compose.service"]; name != "" {
		return name
	}
	return id
}
