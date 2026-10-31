package containerd

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// labelsOpt matches the WithContainerLabels option by applying it to an empty
// container record.
func labelsOpt(want map[string]string) any {
	return mock.MatchedBy(func(opt containerd.NewContainerOpts) bool {
		var c containers.Container
		return opt(context.Background(), nil, &c) == nil && assert.ObjectsAreEqual(want, c.Labels)
	})
}

func execArgs(argv ...string) any {
	return mock.MatchedBy(func(p *specs.Process) bool { return assert.ObjectsAreEqual(argv, p.Args) })
}

// setupNetSidecar wires a running target "c1" and a network sidecar created
// with labels, and expects the sidecar to be cleaned up. It returns the
// sidecar task for Exec expectations.
func setupNetSidecar(t *testing.T, labels map[string]string) (*MockapiClient, *MockTask) {
	t.Helper()
	api, task, _ := setupNetSidecarContainer(t, labels)
	return api, task
}

func setupNetSidecarContainer(t *testing.T, labels map[string]string) (*MockapiClient, *MockTask, *MockContainer) {
	t.Helper()
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, newRunningTaskWithPID(t, 42)))
	api.EXPECT().GetImage(mock.Anything, "nettools").Return(NewMockImage(t), nil)

	sidecarTask := NewMockTask(t)
	sidecarTask.EXPECT().Start(mock.Anything).Return(nil)
	exitCh := make(chan containerd.ExitStatus, 1)
	exitCh <- *containerd.NewExitStatus(137, time.Now(), nil)
	sidecarTask.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	sidecarTask.EXPECT().Kill(mock.Anything, mock.Anything).Return(nil)
	sidecarTask.EXPECT().Delete(mock.Anything).Return(nil, nil)

	sidecar := NewMockContainer(t)
	sidecar.EXPECT().NewTask(mock.Anything, mock.Anything).Return(sidecarTask, nil)
	sidecar.EXPECT().Task(mock.Anything, mock.Anything).Return(sidecarTask, nil)
	sidecar.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil).Once()

	api.EXPECT().NewContainer(mock.Anything,
		mock.MatchedBy(func(id string) bool { return strings.HasPrefix(id, "pumba-sidecar-") }),
		mock.Anything, mock.Anything, mock.Anything, labelsOpt(labels),
	).Return(sidecar, nil)
	return api, sidecarTask, sidecar
}

func TestInspectNetnsInLabeledSidecar(t *testing.T) {
	api, task := setupNetSidecar(t, ctr.SidecarLabels("status1", "c1", ctr.SidecarKindInspect))
	task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).Return(newSuccessProcess(t), nil).Once()
	task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("iptables", "-S"), mock.Anything).Return(newSuccessProcess(t), nil).Once()

	state, err := newTestClient(api).InspectNetns(context.Background(), &ctr.InspectRequest{
		Container: testContainer("c1"), Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "status1",
	})

	require.NoError(t, err)
	assert.Equal(t, &ctr.FaultState{}, state)
}

func TestInspectNetnsTCFailureIsAnError(t *testing.T) {
	api, task := setupNetSidecar(t, ctr.SidecarLabels("", "c1", ctr.SidecarKindInspect))
	task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).Return(newFailProcess(t, 2), nil).Once()

	state, err := newTestClient(api).InspectNetns(context.Background(), &ctr.InspectRequest{
		Container: testContainer("c1"), Sidecar: ctr.SidecarSpec{Image: "nettools"},
	})

	require.ErrorContains(t, err, "failed to list qdiscs")
	assert.ErrorContains(t, err, "exited with code 2")
	assert.Nil(t, state)
}

func TestInspectNetnsInTargetWithoutIPTablesIsAnError(t *testing.T) {
	task := newRunningTask(t)
	task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).Return(newSuccessProcess(t), nil).Once()
	task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("iptables", "-S"), mock.Anything).Return(newFailProcess(t, 127), nil).Once()
	api := NewMockapiClient(t)
	api.EXPECT().LoadContainer(mock.Anything, "c1").Return(newMockContainer(t, "c1", "nginx", nil, task), nil).Twice()

	state, err := newTestClient(api).InspectNetns(context.Background(), &ctr.InspectRequest{Container: testContainer("c1")})

	require.ErrorContains(t, err, "failed to list iptables rules")
	assert.ErrorContains(t, err, "exited with code 127")
	assert.Nil(t, state)
}

func TestInspectNetnsIncompleteOutputIsAnError(t *testing.T) {
	for _, sidecar := range []bool{false, true} {
		t.Run(map[bool]string{false: "target", true: "sidecar"}[sidecar], func(t *testing.T) {
			t.Parallel()
			var api *MockapiClient
			var task *MockTask
			req := &ctr.InspectRequest{Container: testContainer("c1")}
			if sidecar {
				api, task = setupNetSidecar(t, ctr.SidecarLabels("", "c1", ctr.SidecarKindInspect))
				req.Sidecar.Image = "nettools"
			} else {
				api = NewMockapiClient(t)
				task = newRunningTask(t)
				setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))
			}
			task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).
				Return(newSuccessProcess(t), nil).Once()
			proc := newSuccessProcess(t)
			proc.EXPECT().IO().Unset()
			proc.EXPECT().IO().Return(newHeldIO()).Once()
			task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("iptables", "-S"), mock.Anything).
				Return(proc, nil).Once()

			state, err := newTestClient(api).InspectNetns(context.Background(), req)

			require.ErrorContains(t, err, "incomplete exec output")
			assert.Nil(t, state, "a parseable prefix is not evidence of clean state")
		})
	}
}

func TestNetemSidecarCarriesLabels(t *testing.T) {
	api, task := setupNetSidecar(t, ctr.SidecarLabels("run1", "c1", ctr.SidecarKindNetem))
	task.EXPECT().Exec(mock.Anything, mock.Anything, mock.MatchedBy(func(p *specs.Process) bool { return p.Args[0] == "sh" }), mock.Anything).Return(newSuccessProcess(t), nil).Once()

	err := newTestClient(api).NetemContainer(context.Background(), &ctr.NetemRequest{
		Container: testContainer("c1"), Interface: "eth0", Command: []string{"delay", "100ms"},
		Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "run1",
	})

	require.NoError(t, err)
}

func TestIPTablesSidecarTagsRulesAndCarriesLabels(t *testing.T) {
	api, task := setupNetSidecar(t, ctr.SidecarLabels("run1", "c1", ctr.SidecarKindIPTables))
	task.EXPECT().Exec(mock.Anything, mock.Anything,
		execArgs("iptables", "-I", "INPUT", "-s", "10.0.0.1/32", "-m", "comment", "--comment", "pumba:run1", "-j", "DROP"),
		mock.Anything).Return(newSuccessProcess(t), nil).Once()

	err := newTestClient(api).IPTablesContainer(context.Background(), &ctr.IPTablesRequest{
		Container: testContainer("c1"), CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"},
		SrcIPs:  []*net.IPNet{{IP: net.ParseIP("10.0.0.1").To4(), Mask: net.CIDRMask(32, 32)}},
		Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunID: "run1",
	})

	require.NoError(t, err)
}

func TestStressSidecarCarriesLabels(t *testing.T) { //nolint:paralleltest // mutates package-level cgroupReader
	api := setupStressTarget(t, 42, "0::/system.slice/docker-c1.scope\n")
	api.EXPECT().GetImage(mock.Anything, "stress-ng").Return(NewMockImage(t), nil)
	api.EXPECT().NewContainer(mock.Anything, stressPrefix(), mock.Anything, mock.Anything, mock.Anything,
		labelsOpt(ctr.SidecarLabels("run1", "c1", ctr.SidecarKindStress)),
	).Return(nil, assert.AnError)

	req := stressReq(testContainer("c1"), []string{"--cpu", "1"}, "stress-ng", false, time.Second, false, false)
	req.RunID = "run1"
	_, err := newTestClient(api).StressContainer(context.Background(), req)

	require.ErrorIs(t, err, assert.AnError)
}

func TestInspectNetnsExecCleanupFailure(t *testing.T) {
	t.Parallel()
	for _, sidecar := range []bool{false, true} {
		t.Run(map[bool]string{false: "target", true: "sidecar"}[sidecar], func(t *testing.T) {
			t.Parallel()
			var api *MockapiClient
			var task *MockTask
			req := &ctr.InspectRequest{Container: testContainer("c1")}
			if sidecar {
				api, task = setupNetSidecar(t, ctr.SidecarLabels("", "c1", ctr.SidecarKindInspect))
				req.Sidecar.Image = "nettools"
			} else {
				api, task = NewMockapiClient(t), newRunningTask(t)
				setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))
			}
			proc := newSuccessProcess(t)
			proc.EXPECT().Delete(mock.Anything).Unset()
			proc.EXPECT().Delete(mock.Anything).Return(nil, assert.AnError).Once()
			task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).Return(proc, nil).Once()

			state, err := newTestClient(api).InspectNetns(context.Background(), req)

			require.ErrorIs(t, err, assert.AnError)
			assert.Nil(t, state)
		})
	}
}

func TestInspectNetnsCancellation(t *testing.T) {
	t.Parallel()
	for _, sidecar := range []bool{false, true} {
		t.Run(map[bool]string{false: "target", true: "sidecar"}[sidecar], func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var api *MockapiClient
			var task *MockTask
			req := &ctr.InspectRequest{Container: testContainer("c1")}
			if sidecar {
				api, task = setupNetSidecar(t, ctr.SidecarLabels("", "c1", ctr.SidecarKindInspect))
				req.Sidecar.Image = "nettools"
			} else {
				api, task = NewMockapiClient(t), newRunningTask(t)
				setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))
			}
			proc := NewMockProcess(t)
			proc.EXPECT().Wait(mock.Anything).Return(make(<-chan containerd.ExitStatus), nil).Once()
			proc.EXPECT().Start(mock.Anything).Run(func(context.Context) { cancel() }).Return(nil).Once()
			proc.EXPECT().Delete(mock.Anything).Return(nil, nil).Once()
			task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).Return(proc, nil).Once()

			state, err := newTestClient(api).InspectNetns(ctx, req)

			require.ErrorIs(t, err, context.Canceled)
			assert.Nil(t, state)
		})
	}
}
