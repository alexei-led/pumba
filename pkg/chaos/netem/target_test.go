package netem

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func webContainer() *container.Container {
	return &container.Container{
		ContainerID:   "abc1234567890def",
		ContainerName: "/web-1",
		Networks: map[string]container.NetworkLink{
			"bridge": {IPv4Address: "172.17.0.5"},
		},
	}
}

func TestResolveRequestTargetsDoesNotCacheResolvedAddresses(t *testing.T) {
	mockClient := container.NewMockClient(t)
	first := &container.Container{
		ContainerID: "peer-id", ContainerName: "peer",
		Networks: map[string]container.NetworkLink{"bridge": {IPv4Address: "10.0.0.1"}},
	}
	second := &container.Container{
		ContainerID: "peer-id", ContainerName: "peer",
		Networks: map[string]container.NetworkLink{"bridge": {IPv4Address: "10.0.0.2"}},
	}
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{first}, nil).Once()
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{second}, nil).Once()
	original := &container.NetemRequest{TargetNames: []string{"peer"}}

	firstRequest, err := resolveRequestTargets(context.Background(), mockClient, original)
	require.NoError(t, err)
	secondRequest, err := resolveRequestTargets(context.Background(), mockClient, original)
	require.NoError(t, err)

	assert.Equal(t, []string{"peer"}, original.TargetNames)
	assert.Empty(t, original.IPs)
	assert.Equal(t, "10.0.0.1/32", firstRequest.IPs[0].String())
	assert.Equal(t, "10.0.0.2/32", secondRequest.IPs[0].String())
}

func TestResolveTargetNames_NoOpWhenEmpty(t *testing.T) {
	// No ListContainers expectation is set up: if resolveTargetNames made a
	// runtime call for the plain IP/CIDR case, this mock would panic on the
	// unexpected call, failing the test.
	mockClient := container.NewMockClient(t)
	req := &container.NetemRequest{
		IPs: []*net.IPNet{{IP: net.IPv4(10, 0, 0, 1), Mask: net.CIDRMask(32, 32)}},
	}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.NoError(t, err)
	require.Len(t, req.IPs, 1)
	assert.Equal(t, "10.0.0.1/32", req.IPs[0].String())
}

func TestResolveTargetNames_ByName(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c := webContainer()
	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)

	req := &container.NetemRequest{TargetNames: []string{"web-1"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.NoError(t, err)
	require.Len(t, req.IPs, 1)
	assert.Equal(t, "172.17.0.5/32", req.IPs[0].String())
	assert.Empty(t, req.TargetNames, "TargetNames must be cleared once resolved")
}

func TestResolveTargetNames_DeduplicatesRepeatedContainer(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c := &container.Container{ContainerID: "peer-id", ContainerName: "peer", Networks: map[string]container.NetworkLink{}}
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)
	mockClient.EXPECT().ContainerAddresses(mock.Anything, c).
		Return([]net.IP{net.ParseIP("10.0.0.5")}, nil).Once()
	req := &container.NetemRequest{TargetNames: []string{"peer", "peer"}}

	err := resolveTargetNames(context.Background(), mockClient, req)

	require.NoError(t, err)
	require.Len(t, req.IPs, 1)
	assert.Equal(t, "10.0.0.5/32", req.IPs[0].String())
}

func TestResolveTargetNames_MixedIPAndName(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c := webContainer()
	mockClient.EXPECT().ListContainers(mock.Anything,
		mock.AnythingOfType("container.FilterFunc"), container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)

	req := &container.NetemRequest{
		IPs:         []*net.IPNet{{IP: net.IPv4(10, 0, 0, 1), Mask: net.CIDRMask(32, 32)}},
		TargetNames: []string{"web-1"},
	}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.NoError(t, err)
	require.Len(t, req.IPs, 2)
	assert.Equal(t, "10.0.0.1/32", req.IPs[0].String())
	assert.Equal(t, "172.17.0.5/32", req.IPs[1].String())
}

func TestResolveTargetNames_ByExactID(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c := webContainer()
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)

	req := &container.NetemRequest{TargetNames: []string{"abc1234567890def"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.NoError(t, err)
	require.Len(t, req.IPs, 1)
	assert.Equal(t, "172.17.0.5/32", req.IPs[0].String())
}

func TestResolveTargetNames_ByUniqueIDPrefix(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c := webContainer()
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)

	req := &container.NetemRequest{TargetNames: []string{"abc123"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.NoError(t, err)
	require.Len(t, req.IPs, 1)
	assert.Equal(t, "172.17.0.5/32", req.IPs[0].String())
}

func TestResolveTargetNames_ShortIDPrefixNotMatched(t *testing.T) {
	// Prefixes shorter than minIDPrefixLen are never tried against IDs, only
	// exact names, to avoid a short typo accidentally landing on an
	// unrelated container in a large fleet.
	mockClient := container.NewMockClient(t)
	c := webContainer()
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)

	req := &container.NetemRequest{TargetNames: []string{"abc"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"abc"`)
	assert.Contains(t, err.Error(), "does not match")
}

func TestResolveTargetNames_NotFound(t *testing.T) {
	mockClient := container.NewMockClient(t)
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{webContainer()}, nil)

	req := &container.NetemRequest{TargetNames: []string{"does-not-exist"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"does-not-exist"`)
	assert.Contains(t, err.Error(), "not a valid IP/CIDR")
	assert.Contains(t, err.Error(), "does not match any running container")
}

func TestResolveTargetNames_AmbiguousName(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c1 := &container.Container{ContainerID: "id1", ContainerName: "dup", Networks: map[string]container.NetworkLink{"n": {IPv4Address: "10.1.1.1"}}}
	c2 := &container.Container{ContainerID: "id2", ContainerName: "dup", Networks: map[string]container.NetworkLink{"n": {IPv4Address: "10.1.1.2"}}}
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c1, c2}, nil)

	req := &container.NetemRequest{TargetNames: []string{"dup"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestResolveTargetNames_AmbiguousAcrossExactNameAndID(t *testing.T) {
	mockClient := container.NewMockClient(t)
	byName := &container.Container{ContainerID: "id-1", ContainerName: "shared", Networks: map[string]container.NetworkLink{}}
	byID := &container.Container{ContainerID: "shared", ContainerName: "other", Networks: map[string]container.NetworkLink{}}
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{byName, byID}, nil)

	req := &container.NetemRequest{TargetNames: []string{"shared"}}
	err := resolveTargetNames(context.Background(), mockClient, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestResolveTargetNames_AmbiguousIDPrefix(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c1 := &container.Container{ContainerID: "abcdef123456", ContainerName: "c1", Networks: map[string]container.NetworkLink{"n": {IPv4Address: "10.1.1.1"}}}
	c2 := &container.Container{ContainerID: "abcdef789012", ContainerName: "c2", Networks: map[string]container.NetworkLink{"n": {IPv4Address: "10.1.1.2"}}}
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c1, c2}, nil)

	req := &container.NetemRequest{TargetNames: []string{"abcdef"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestResolveTargetNames_RejectsIPv6(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c := &container.Container{ContainerID: "id1", ContainerName: "web-1", Networks: map[string]container.NetworkLink{}}
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)
	mockClient.EXPECT().ContainerAddresses(mock.Anything, c).
		Return([]net.IP{net.ParseIP("fd00::5")}, nil)

	req := &container.NetemRequest{TargetNames: []string{"web-1"}}
	err := resolveTargetNames(context.Background(), mockClient, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "IPv6")
	assert.Contains(t, err.Error(), "not supported")
}

func TestResolveTargetNames_AddressResolutionError(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c := &container.Container{ContainerID: "id1", ContainerName: "web-1", Networks: map[string]container.NetworkLink{}}
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)
	mockClient.EXPECT().ContainerAddresses(mock.Anything, c).
		Return(nil, errors.New("network namespace unavailable"))

	req := &container.NetemRequest{TargetNames: []string{"web-1"}}
	err := resolveTargetNames(context.Background(), mockClient, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "network namespace unavailable")
}

func TestResolveTargetNames_ContainerWithNoIP(t *testing.T) {
	mockClient := container.NewMockClient(t)
	c := &container.Container{ContainerID: "id1", ContainerName: "headless", Networks: map[string]container.NetworkLink{"host": {}}}
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)
	mockClient.EXPECT().ContainerAddresses(mock.Anything, c).Return(nil, nil)

	req := &container.NetemRequest{TargetNames: []string{"headless"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no IP address")
}

func TestResolveTargetNames_MultiNetworkContainer(t *testing.T) {
	// A container attached to more than one network contributes every IP it
	// has, not just one arbitrarily chosen network.
	mockClient := container.NewMockClient(t)
	c := &container.Container{
		ContainerID:   "id1",
		ContainerName: "multi",
		Networks: map[string]container.NetworkLink{
			"front": {IPv4Address: "10.0.1.5"},
			"back":  {IPv4Address: "10.0.2.5"},
		},
	}
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{c}, nil)

	req := &container.NetemRequest{TargetNames: []string{"multi"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.NoError(t, err)
	require.Len(t, req.IPs, 2)
	assert.Equal(t, "10.0.1.5/32", req.IPs[0].String())
	assert.Equal(t, "10.0.2.5/32", req.IPs[1].String())
}

func TestResolveTargetNames_ListContainersError(t *testing.T) {
	mockClient := container.NewMockClient(t)
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return(nil, errors.New("daemon unreachable"))

	req := &container.NetemRequest{TargetNames: []string{"anything"}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon unreachable")
}

func TestResolveTargetNames_EmptyTargetRejected(t *testing.T) {
	mockClient := container.NewMockClient(t)
	mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything, container.ListOpts{All: false}).
		Return([]*container.Container{webContainer()}, nil)

	req := &container.NetemRequest{TargetNames: []string{""}}
	err := resolveTargetNames(context.Background(), mockClient, req)
	require.Error(t, err)
}
