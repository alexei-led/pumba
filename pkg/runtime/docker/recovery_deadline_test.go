package docker

import (
	"context"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/tc"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRecoveryKeepsSharedDeleteVerificationDeadline(t *testing.T) {
	for _, image := range []string{"", "nettools"} {
		t.Run(image, func(t *testing.T) {
			api := NewMockEngine(t)
			id, privileged := "target", true
			if image != "" {
				id, privileged = "inspector", false
				expectSidecar(api, "target", id, ctr.SidecarLabels("current", "target", ctr.SidecarKindInspect))
			}
			rule := ctr.PumbaRule{Backend: "iptables-nft", RunID: "old", Chain: "INPUT", Spec: "-A INPUT -m comment --comment \"pumba:old\" -j DROP"}
			req := &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "target"}, RunFilter: "old", InvocationID: "current", Sidecar: ctr.SidecarSpec{Image: image}, Rules: []ctr.PumbaRule{rule}}
			expectExec(api, id, privileged,
				execStep{argv: tc.QdiscShowCommand()},
				execStep{argv: tc.IPTablesVersionCommand(), stdout: "iptables v1.8.13 (nf_tables)\n"},
				execStep{argv: tc.IPTablesNFTShowCommand(), stdout: rule.Spec + "\n"},
			)
			command, err := tc.RecoveryDeleteCommand(rule.Backend, rule.RunID, rule.Chain, rule.Spec)
			require.NoError(t, err)
			var deadline time.Time
			expectRecoveryDeadlineExec(t, api, id, privileged, execStep{argv: command}, &deadline, true)
			for _, step := range []execStep{
				{argv: tc.QdiscShowCommand()},
				{argv: tc.IPTablesVersionCommand(), stdout: "iptables v1.8.13 (nf_tables)\n"},
				{argv: tc.IPTablesNFTShowCommand()},
			} {
				expectRecoveryDeadlineExec(t, api, id, privileged, step, &deadline, false)
			}
			client := dockerClient{containerAPI: api, imageAPI: api}
			require.NoError(t, client.RecoverRules(context.Background(), req))
			require.False(t, deadline.IsZero())
		})
	}
}

func expectRecoveryDeadlineExec(t *testing.T, api *mocks.APIClient, id string, privileged bool, step execStep, deadline *time.Time, first bool) {
	t.Helper()
	check := func(ctx context.Context) {
		actual, ok := ctx.Deadline()
		require.True(t, ok)
		if first && deadline.IsZero() {
			*deadline = actual
			require.LessOrEqual(t, time.Until(actual), recoveryTimeout, "runtime extended the recovery operation deadline")
		} else {
			require.Equal(t, *deadline, actual, "verification must share the admitted delete deadline")
		}
	}
	execID := "bounded-" + step.argv[0]
	api.EXPECT().ContainerExecCreate(mock.Anything, id, ctypes.ExecOptions{
		AttachStdout: true, AttachStderr: true, Privileged: privileged, Cmd: step.argv,
	}).Run(func(ctx context.Context, _ string, _ ctypes.ExecOptions) { check(ctx) }).
		Return(ctypes.ExecCreateResponse{ID: execID}, nil).Once()
	api.EXPECT().ContainerExecAttach(mock.Anything, execID, ctypes.ExecAttachOptions{}).
		Run(func(ctx context.Context, _ string, _ ctypes.ExecAttachOptions) { check(ctx) }).
		Return(execAttachOutput(step.stdout, step.stderr), nil).Once()
	api.EXPECT().ContainerExecInspect(mock.Anything, execID).
		Run(func(ctx context.Context, _ string) { check(ctx) }).
		Return(ctypes.ExecInspect{}, nil).Once()
}
