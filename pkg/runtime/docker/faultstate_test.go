package docker

import (
	"bufio"
	"bytes"
	"context"
	"testing"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	tcplan "github.com/alexei-led/pumba/pkg/tc"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	qdiscShowNetem   = "qdisc noqueue 0: dev lo root refcnt 2\nqdisc netem 504d: dev eth0 root refcnt 2 limit 1000 delay 100ms\n"
	iptablesShowRule = "-P INPUT ACCEPT\n-A INPUT -i eth0 -m comment --comment \"pumba:run1\" -j DROP\n"
)

// execAttachOutput returns an attach stream multiplexed the way Docker sends
// non-TTY exec output.
func execAttachOutput(stdout, stderr string) types.HijackedResponse {
	var buf bytes.Buffer
	if stdout != "" {
		_, _ = stdcopy.NewStdWriter(&buf, stdcopy.Stdout).Write([]byte(stdout))
	}
	if stderr != "" {
		_, _ = stdcopy.NewStdWriter(&buf, stdcopy.Stderr).Write([]byte(stderr))
	}
	conn := &mockConn{}
	conn.On("Close").Return(nil)
	return types.HijackedResponse{Conn: conn, Reader: bufio.NewReader(&buf)}
}

type execStep struct {
	argv     []string
	stdout   string
	stderr   string
	exitCode int
}

func expectExec(api *mocks.APIClient, containerID string, privileged bool, steps ...execStep) {
	for _, step := range steps {
		execID := "exec-" + step.argv[0]
		api.EXPECT().ContainerExecCreate(mock.Anything, containerID, ctypes.ExecOptions{
			AttachStdout: true, AttachStderr: true, Privileged: privileged, Cmd: step.argv,
		}).Return(ctypes.ExecCreateResponse{ID: execID}, nil).Once()
		api.EXPECT().ContainerExecAttach(mock.Anything, execID, ctypes.ExecAttachOptions{}).Return(execAttachOutput(step.stdout, step.stderr), nil).Once()
		api.EXPECT().ContainerExecInspect(mock.Anything, execID).Return(ctypes.ExecInspect{ExitCode: step.exitCode}, nil).Once()
	}
}

func expectSidecar(api *mocks.APIClient, targetID, sidecarID string, labels map[string]string) {
	api.EXPECT().ContainerCreate(mock.Anything,
		mock.MatchedBy(func(c *ctypes.Config) bool { return assert.ObjectsAreEqual(labels, c.Labels) }),
		mock.MatchedBy(func(h *ctypes.HostConfig) bool { return h.NetworkMode == ctypes.NetworkMode("container:"+targetID) }),
		mock.Anything, mock.Anything, "",
	).Return(ctypes.CreateResponse{ID: sidecarID}, nil).Once()
	api.EXPECT().ContainerStart(mock.Anything, sidecarID, ctypes.StartOptions{}).Return(nil).Once()
	api.EXPECT().ContainerRemove(mock.Anything, sidecarID, ctypes.RemoveOptions{Force: true}).Return(nil).Once()
}

func inspectReq(image string) *ctr.InspectRequest {
	return &ctr.InspectRequest{
		Container: &ctr.Container{ContainerID: "target"},
		Sidecar:   ctr.SidecarSpec{Image: image},
		RunID:     "status1",
	}
}

func TestInspectNetnsInLabeledSidecar(t *testing.T) {
	api := NewMockEngine(t)
	expectSidecar(api, "target", "inspector", ctr.SidecarLabels("status1", "target", ctr.SidecarKindInspect))
	expectExec(api, "inspector", false,
		execStep{argv: tcplan.QdiscShowCommand(), stdout: qdiscShowNetem},
		execStep{argv: tcplan.IPTablesShowCommand(), stdout: iptablesShowRule, stderr: "# Warning: iptables-legacy tables present\n"},
	)
	client := dockerClient{containerAPI: api, imageAPI: api}

	state, err := client.InspectNetns(context.Background(), inspectReq("nettools"))

	require.NoError(t, err)
	assert.Equal(t, &ctr.FaultState{
		Qdiscs: []ctr.PumbaQdisc{{Device: "eth0", Kind: "netem", Ownership: ctr.QdiscPumbaOwned}},
		Rules:  []ctr.PumbaRule{{RunID: "run1", Chain: "INPUT", Spec: "-A INPUT -i eth0 -m comment --comment \"pumba:run1\" -j DROP"}},
	}, state)
}

func TestInspectNetnsTCFailureIsAnError(t *testing.T) {
	api := NewMockEngine(t)
	expectSidecar(api, "target", "inspector", ctr.SidecarLabels("status1", "target", ctr.SidecarKindInspect))
	expectExec(api, "inspector", false, execStep{argv: tcplan.QdiscShowCommand(), stderr: "RTNETLINK answers: Operation not permitted\n", exitCode: 2})
	client := dockerClient{containerAPI: api, imageAPI: api}

	state, err := client.InspectNetns(context.Background(), inspectReq("nettools"))

	require.ErrorContains(t, err, "failed to list qdiscs")
	assert.ErrorContains(t, err, "Operation not permitted")
	assert.Nil(t, state)
}

func TestInspectNetnsInTargetWithoutIPTablesIsAnError(t *testing.T) {
	api := NewMockEngine(t)
	expectExec(api, "target", true,
		execStep{argv: tcplan.QdiscShowCommand(), stdout: "qdisc noqueue 0: dev eth0 root refcnt 2\n"},
		execStep{argv: tcplan.IPTablesShowCommand(), stderr: "exec: \"iptables\": executable file not found in $PATH\n", exitCode: 127},
	)
	client := dockerClient{containerAPI: api}

	state, err := client.InspectNetns(context.Background(), inspectReq(""))

	require.ErrorContains(t, err, "failed to list iptables rules")
	assert.ErrorContains(t, err, "exit code 127")
	assert.Nil(t, state)
}

func TestIPTablesContainerTagsRulesInSidecar(t *testing.T) {
	api := NewMockEngine(t)
	expectSidecar(api, "target", "ipt", ctr.SidecarLabels("run1", "target", ctr.SidecarKindIPTables))
	expectExec(api, "ipt", false, execStep{argv: []string{"iptables", "-I", "INPUT", "--dport", "80", "-m", "comment", "--comment", "pumba:run1", "-j", "DROP"}})
	client := dockerClient{containerAPI: api, imageAPI: api}

	err := client.IPTablesContainer(context.Background(), &ctr.IPTablesRequest{
		Container: &ctr.Container{ContainerID: "target"},
		CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}, DPorts: []string{"80"},
		Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "run1",
	})

	require.NoError(t, err)
}

func TestIPTablesContainerWithoutCommentMatchFailsNamingCause(t *testing.T) {
	api := NewMockEngine(t)
	expectSidecar(api, "target", "ipt", ctr.SidecarLabels("run1", "target", ctr.SidecarKindIPTables))
	expectExec(api, "ipt", false, execStep{
		argv:     []string{"iptables", "-I", "INPUT", "-m", "comment", "--comment", "pumba:run1", "-j", "DROP"},
		stderr:   "iptables v1.8.10 (legacy): Couldn't load match `comment':No such file or directory\n",
		exitCode: 2,
	})
	client := dockerClient{containerAPI: api, imageAPI: api}

	err := client.IPTablesContainer(context.Background(), &ctr.IPTablesRequest{
		Container: &ctr.Container{ContainerID: "target"},
		CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"},
		Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "run1",
	})

	require.ErrorIs(t, err, tcplan.ErrCommentMatchUnavailable)
	assert.ErrorContains(t, err, "xt_comment")
}

func TestNetemSidecarCarriesLabels(t *testing.T) {
	api := NewMockEngine(t)
	expectSidecar(api, "target", "tc", ctr.SidecarLabels("run1", "target", ctr.SidecarKindNetem))
	api.EXPECT().ContainerExecCreate(mock.Anything, "tc", mock.AnythingOfType("container.ExecOptions")).Return(ctypes.ExecCreateResponse{ID: "e"}, nil).Once()
	api.EXPECT().ContainerExecAttach(mock.Anything, "e", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil).Once()
	api.EXPECT().ContainerExecInspect(mock.Anything, "e").Return(ctypes.ExecInspect{}, nil).Once()
	client := dockerClient{containerAPI: api, imageAPI: api}

	err := client.NetemContainer(context.Background(), &ctr.NetemRequest{
		Container: &ctr.Container{ContainerID: "target"}, Interface: "eth0",
		Command: []string{"delay", "100ms"}, Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "run1",
	})

	require.NoError(t, err)
}

func TestStressContainerConfigCarriesLabels(t *testing.T) {
	for _, inject := range []bool{false, true} {
		req := &ctr.StressRequest{
			Container: &ctr.Container{ContainerID: "target"}, Stressors: []string{"--cpu", "1"},
			Sidecar: ctr.SidecarSpec{Image: "stress-ng"}, InjectCgroup: inject, RunID: "run1",
		}
		config, _ := stressContainerConfig(req, "systemd", "parent.slice", "/parent.slice/docker-target.scope")
		assert.Equal(t, map[string]string{
			ctr.SkipLabel:   "true",
			ctr.RunIDLabel:  "run1",
			ctr.TargetLabel: "target",
			ctr.KindLabel:   ctr.SidecarKindStress,
		}, config.Labels)
	}
}

func TestRunExecRejectsMalformedStream(t *testing.T) {
	api := NewMockEngine(t)
	conn := &mockConn{}
	conn.On("Close").Return(nil)
	api.EXPECT().ContainerExecCreate(mock.Anything, "c", mock.AnythingOfType("container.ExecOptions")).Return(ctypes.ExecCreateResponse{ID: "e"}, nil)
	api.EXPECT().ContainerExecAttach(mock.Anything, "e", ctypes.ExecAttachOptions{}).Return(types.HijackedResponse{
		Conn: conn, Reader: bufio.NewReader(bytes.NewReader([]byte("plain text, not multiplexed"))),
	}, nil)
	api.EXPECT().ContainerExecInspect(mock.Anything, "e").Return(ctypes.ExecInspect{}, nil)
	client := dockerClient{containerAPI: api}

	_, err := client.runExec(context.Background(), "c", []string{"tc", "qdisc", "show"}, false)

	require.ErrorContains(t, err, "demultiplex")
}
