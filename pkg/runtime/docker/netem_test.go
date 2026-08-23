package docker

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNetemContainerRunsSharedOwnershipPlan(t *testing.T) {
	client, api := newNetemTestClient(t, 0)
	req := &ctr.NetemRequest{
		Container: &ctr.Container{ContainerID: "target"}, Interface: "eth0",
		Command: []string{"delay", "100ms"},
		IPs:     []*net.IPNet{{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(32, 32)}},
	}

	require.NoError(t, client.NetemContainer(context.Background(), req))
	assertNetemPlan(t, api, "target", "root handle 504d: prio", "tc qdisc show dev 'eth0'")
}

func TestStopNetemContainerRunsVerifiedCleanupPlan(t *testing.T) {
	client, api := newNetemTestClient(t, 0)
	req := &ctr.NetemRequest{
		Container: &ctr.Container{ContainerID: "target"}, Interface: "eth0",
		IPs: []*net.IPNet{{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(32, 32)}},
	}

	require.NoError(t, client.StopNetemContainer(context.Background(), req))
	assertNetemPlan(t, api, "target", "refusing to remove unverified Pumba qdisc topology", "qdisc del dev 'eth0' parent 504d:3 handle 5050:")
}

func TestNetemContainerPropagatesCleanupError(t *testing.T) {
	client, api := newNetemTestClient(t, 1)
	req := &ctr.NetemRequest{Container: &ctr.Container{ContainerID: "target"}, Interface: "eth0"}

	err := client.StopNetemContainer(context.Background(), req)
	require.Error(t, err)
	api.AssertExpectations(t)
}

func TestNetemContainerDryRunDoesNotExecutePlan(t *testing.T) {
	api := NewMockEngine(t)
	client := dockerClient{containerAPI: api}
	err := client.NetemContainer(context.Background(), &ctr.NetemRequest{
		Container: &ctr.Container{ContainerID: "target"}, Interface: "eth0", DryRun: true,
	})
	require.NoError(t, err)
	api.AssertNotCalled(t, "ContainerExecCreate", mock.Anything, mock.Anything, mock.Anything)
}

func newNetemTestClient(t *testing.T, exitCode int) (dockerClient, *mocks.APIClient) {
	t.Helper()
	api := NewMockEngine(t)
	api.EXPECT().ContainerExecCreate(mock.Anything, "target", ctypes.ExecOptions{
		AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "sh"},
	}).Return(ctypes.ExecCreateResponse{ID: "check"}, nil)
	api.EXPECT().ContainerExecAttach(mock.Anything, "check", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	api.EXPECT().ContainerExecInspect(mock.Anything, "check").Return(ctypes.ExecInspect{}, nil)
	api.EXPECT().ContainerExecCreate(mock.Anything, "target", mock.MatchedBy(func(options ctypes.ExecOptions) bool {
		return options.Privileged && len(options.Cmd) == 3 && options.Cmd[0] == "sh" && options.Cmd[1] == "-ec"
	})).Return(ctypes.ExecCreateResponse{ID: "plan"}, nil)
	api.EXPECT().ContainerExecAttach(mock.Anything, "plan", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	api.EXPECT().ContainerExecInspect(mock.Anything, "plan").Return(ctypes.ExecInspect{ExitCode: exitCode}, nil)
	return dockerClient{containerAPI: api}, api
}

func assertNetemPlan(t *testing.T, api *mocks.APIClient, target string, snippets ...string) {
	t.Helper()
	for _, call := range api.Calls {
		if call.Method != "ContainerExecCreate" || call.Arguments.Get(1) != target {
			continue
		}
		options, ok := call.Arguments.Get(2).(ctypes.ExecOptions)
		if !ok || len(options.Cmd) != 3 || options.Cmd[0] != "sh" {
			continue
		}
		for _, snippet := range snippets {
			assert.True(t, strings.Contains(options.Cmd[2], snippet), "plan must contain %q", snippet)
		}
		return
	}
	t.Fatal("netem plan was not executed")
}
