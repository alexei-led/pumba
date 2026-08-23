package docker

import (
	"context"
	"net"

	ctr "github.com/alexei-led/pumba/pkg/container"
)

// ContainerAddresses returns addresses already captured by the runtime inspect
// performed while listing containers.
func (c dockerClient) ContainerAddresses(_ context.Context, container *ctr.Container) ([]net.IP, error) {
	return container.IPs(), nil
}
