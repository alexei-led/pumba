package podman

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types/system"
	dockerapi "github.com/docker/docker/client"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewClient_ExplicitSocketUnreachable(t *testing.T) {
	_, err := NewClient("/tmp/pumba-podman-missing-xyz")
	require.Error(t, err)
	require.Contains(t, err.Error(), "explicit socket")
}

func TestNewClient_APIClientFactoryError(t *testing.T) {
	path := makeSocketFile(t)
	stubNewAPIClient(t, func(string, *tls.Config) (*dockerapi.Client, error) {
		return nil, errors.New("factory boom")
	})
	_, err := NewClient(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create api client")
	require.Contains(t, err.Error(), "factory boom")
}

func TestNewClient_DelegateWrapError(t *testing.T) {
	path := makeSocketFile(t)
	stubNewDelegate(t, func(*dockerapi.Client) (ctr.Client, error) {
		return nil, errors.New("wrap boom")
	})
	_, err := NewClient(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrap docker delegate")
	require.Contains(t, err.Error(), "wrap boom")
}

func TestNewClient_InfoError(t *testing.T) {
	path := makeSocketFile(t)
	stubFetchInfo(t, func(context.Context, apiBackend) (system.Info, error) {
		return system.Info{}, errors.New("info boom")
	})
	_, err := NewClient(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "query /info")
	require.Contains(t, err.Error(), "info boom")
}

func TestNewClient_RootlessDetected(t *testing.T) {
	path := makeSocketFile(t)
	stubFetchInfo(t, func(context.Context, apiBackend) (system.Info, error) {
		return system.Info{SecurityOptions: []string{"name=seccomp,profile=default", "name=rootless"}}, nil
	})
	c, err := NewClient(path)
	require.NoError(t, err)
	defer c.Close()

	p, ok := c.(*podmanClient)
	require.True(t, ok)
	require.True(t, p.rootless)
	require.Equal(t, "unix://"+path, p.socketURI)
	require.NotNil(t, p.api)
	require.NotNil(t, p.Client)
}

func TestNewClient_Rootful(t *testing.T) {
	path := makeSocketFile(t)
	stubFetchInfo(t, func(context.Context, apiBackend) (system.Info, error) {
		return system.Info{SecurityOptions: []string{"name=seccomp,profile=default"}}, nil
	})
	c, err := NewClient(path)
	require.NoError(t, err)
	defer c.Close()

	p := c.(*podmanClient)
	require.False(t, p.rootless)
}

func TestPodmanClient_Close_NilAPIIsNoOp(t *testing.T) {
	p := &podmanClient{}
	require.NoError(t, p.Close())
}

func TestPodmanClient_RootlessGuards_ReturnError(t *testing.T) {
	// Mock starts with no expectations; if any delegate method is invoked,
	// mock.AssertExpectations will flag it via t.Cleanup.
	mockDelegate := ctr.NewMockClient(t)
	p := &podmanClient{
		Client:    mockDelegate,
		rootless:  true,
		socketURI: "unix:///run/user/1000/podman/podman.sock",
	}

	ctx := context.Background()
	target := &ctr.Container{ContainerID: "abc", ContainerName: "/x"}

	t.Run("NetemContainer", func(t *testing.T) {
		err := p.NetemContainer(ctx, &ctr.NetemRequest{
			Container: target,
			Interface: "eth0",
			Command:   []string{"delay", "100ms"},
			Duration:  time.Second,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "netem")
		require.Contains(t, err.Error(), p.socketURI)
	})

	t.Run("StopNetemContainer", func(t *testing.T) {
		err := p.StopNetemContainer(ctx, &ctr.NetemRequest{
			Container: target,
			Interface: "eth0",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "netem")
		require.Contains(t, err.Error(), p.socketURI)
	})

	t.Run("IPTablesContainer", func(t *testing.T) {
		err := p.IPTablesContainer(ctx, &ctr.IPTablesRequest{
			Container: target,
			CmdPrefix: []string{"-A", "INPUT"},
			CmdSuffix: []string{"-j", "DROP"},
			Duration:  time.Second,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "iptables")
		require.Contains(t, err.Error(), p.socketURI)
	})

	t.Run("StopIPTablesContainer", func(t *testing.T) {
		err := p.StopIPTablesContainer(ctx, &ctr.IPTablesRequest{
			Container: target,
			CmdPrefix: []string{"-D", "INPUT"},
			CmdSuffix: []string{"-j", "DROP"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "iptables")
		require.Contains(t, err.Error(), p.socketURI)
	})
}

func TestPodmanClient_RootfulGuards_Delegate(t *testing.T) {
	mockDelegate := ctr.NewMockClient(t)
	p := &podmanClient{
		Client:    mockDelegate,
		rootless:  false,
		socketURI: "unix:///run/podman/podman.sock",
	}

	ctx := context.Background()
	target := &ctr.Container{ContainerID: "abc", ContainerName: "/x"}
	netemCmd := []string{"delay", "100ms"}
	img := "img"

	netemReq := &ctr.NetemRequest{
		Container: target,
		Interface: "eth0",
		Command:   netemCmd,
		Duration:  time.Second,
		Sidecar:   ctr.SidecarSpec{Image: img},
	}
	mockDelegate.EXPECT().NetemContainer(ctx, netemReq).Return(nil).Once()
	require.NoError(t, p.NetemContainer(ctx, netemReq))

	stopNetemReq := &ctr.NetemRequest{
		Container: target,
		Interface: "eth0",
		Sidecar:   ctr.SidecarSpec{Image: img},
	}
	mockDelegate.EXPECT().StopNetemContainer(ctx, stopNetemReq).Return(nil).Once()
	require.NoError(t, p.StopNetemContainer(ctx, stopNetemReq))

	prefix := []string{"-A", "INPUT"}
	suffix := []string{"-j", "DROP"}
	ipReq := &ctr.IPTablesRequest{
		Container: target,
		CmdPrefix: prefix,
		CmdSuffix: suffix,
		Duration:  time.Second,
		Sidecar:   ctr.SidecarSpec{Image: img},
	}
	mockDelegate.EXPECT().IPTablesContainer(ctx, ipReq).Return(nil).Once()
	require.NoError(t, p.IPTablesContainer(ctx, ipReq))

	stopIPReq := &ctr.IPTablesRequest{
		Container: target,
		CmdPrefix: prefix,
		CmdSuffix: suffix,
		Sidecar:   ctr.SidecarSpec{Image: img},
	}
	mockDelegate.EXPECT().StopIPTablesContainer(ctx, stopIPReq).Return(nil).Once()
	require.NoError(t, p.StopIPTablesContainer(ctx, stopIPReq))
}

func TestPodmanClient_PromotedMethodsDelegate(t *testing.T) {
	// Sanity check: methods not explicitly overridden (e.g. KillContainer) are
	// promoted from the embedded ctr.Client. Ensures the embedding wiring works.
	mockDelegate := ctr.NewMockClient(t)
	p := &podmanClient{Client: mockDelegate}
	target := &ctr.Container{ContainerID: "x"}

	mockDelegate.EXPECT().KillContainer(mock.Anything, target, "SIGKILL", false).Return(nil).Once()
	require.NoError(t, p.KillContainer(context.Background(), target, "SIGKILL", false))
}

// --- test helpers ---

func stubNewAPIClient(t *testing.T, fn func(host string, tlsConfig *tls.Config) (*dockerapi.Client, error)) {
	t.Helper()
	orig := newAPIClient
	newAPIClient = fn
	t.Cleanup(func() { newAPIClient = orig })
}

func stubNewDelegate(t *testing.T, fn func(*dockerapi.Client) (ctr.Client, error)) {
	t.Helper()
	orig := newDelegate
	newDelegate = fn
	t.Cleanup(func() { newDelegate = orig })
}

func stubFetchInfo(t *testing.T, fn func(context.Context, apiBackend) (system.Info, error)) {
	t.Helper()
	orig := fetchInfo
	fetchInfo = fn
	t.Cleanup(func() { fetchInfo = orig })
}
