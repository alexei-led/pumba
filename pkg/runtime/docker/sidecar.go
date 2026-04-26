package docker

import (
	"context"
	"time"

	ctypes "github.com/docker/docker/api/types/container"
)

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
