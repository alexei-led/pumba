package docker

import (
	"context"
	"net"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerAddressesUsesInspectedNetworks(t *testing.T) {
	client := &dockerClient{}
	container := &ctr.Container{
		Networks: map[string]ctr.NetworkLink{
			"front": {IPv4Address: "10.0.1.5", IPv6Address: "fd00::5"},
			"back":  {IPv4Address: "10.0.2.5"},
		},
	}

	addresses, err := client.ContainerAddresses(context.Background(), container)

	require.NoError(t, err)
	assert.Equal(t, []net.IP{
		net.ParseIP("10.0.1.5").To4(),
		net.ParseIP("10.0.2.5").To4(),
		net.ParseIP("fd00::5"),
	}, addresses)
}
