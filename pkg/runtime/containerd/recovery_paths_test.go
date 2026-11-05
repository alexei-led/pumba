package containerd

import (
	"context"
	"errors"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/tc"
	containers "github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func namespaceName(ctx context.Context) string {
	name, _ := namespaces.Namespace(ctx)
	return name
}

func recoveryPathRequest(image string) *ctr.RuleRecoveryRequest {
	return &ctr.RuleRecoveryRequest{
		Container: testContainer("c1"), RunFilter: "old", InvocationID: "current",
		Sidecar: ctr.SidecarSpec{Image: image},
		Rules:   []ctr.PumbaRule{{Backend: "iptables-nft", RunID: "old", Chain: "INPUT", Spec: `-A INPUT -m comment --comment "pumba:old" -j DROP`}},
	}
}

func TestContainerdPlanRecoveryRules(t *testing.T) {
	client := &containerdClient{}
	plan, err := client.PlanRecoveryRules(recoveryPathRequest("nettools"))
	require.NoError(t, err)
	want, err := tc.RecoveryDeleteCommand("iptables-nft", "old", "INPUT", `-A INPUT -m comment --comment "pumba:old" -j DROP`)
	require.NoError(t, err)
	require.Equal(t, [][]string{want}, plan.Revert)
	require.Equal(t, "nettools", plan.Image)

	for _, req := range []*ctr.RuleRecoveryRequest{nil, {Container: testContainer("c1")}} {
		_, err := client.PlanRecoveryRules(req)
		require.ErrorContains(t, err, "incomplete recovery request")
	}
	invalidRule := recoveryPathRequest("")
	invalidRule.Rules[0].RunID = "different"
	_, err = client.PlanRecoveryRules(invalidRule)
	require.ErrorContains(t, err, "outside run filter")
}

func TestContainerdRecoverRulesRejectsBeforeRuntimeAccess(t *testing.T) {
	api := NewMockapiClient(t)
	client := newTestClient(api)
	require.ErrorContains(t, client.RecoverRules(context.Background(), &ctr.RuleRecoveryRequest{}), "incomplete recovery request")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, client.RecoverRules(ctx, recoveryPathRequest("")), context.Canceled)
}

func TestContainerdRecoverRulesPropagatesDirectInspectionFailure(t *testing.T) {
	api := NewMockapiClient(t)
	task := newRunningTask(t)
	setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))
	inspectErr := errors.New("target inspection failed")
	task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).
		Run(func(ctx context.Context, _ string, _ *specs.Process, _ cio.Creator) {
			require.Equal(t, "test-ns", namespaceName(ctx))
		}).Return(nil, inspectErr).Once()

	err := newTestClient(api).RecoverRules(context.Background(), recoveryPathRequest(""))
	require.ErrorIs(t, err, inspectErr)
}

func TestContainerdRecoverRulesSidecarInspectionAndCleanupFailures(t *testing.T) {
	labels := ctr.SidecarLabels("current", "c1", ctr.SidecarKindInspect)
	api, task, sidecar := setupNetSidecarContainer(t, labels)
	inspectErr, cleanupErr := errors.New("sidecar inspection failed"), errors.New("sidecar snapshot cleanup failed")
	task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).
		Run(func(ctx context.Context, _ string, _ *specs.Process, _ cio.Creator) {
			require.Equal(t, "test-ns", namespaceName(ctx))
			require.NoError(t, ctx.Err())
		}).Return(nil, inspectErr).Once()
	sidecar.EXPECT().Delete(mock.Anything, mock.Anything).Unset()
	sidecar.EXPECT().Delete(mock.Anything, mock.Anything).Return(cleanupErr).Once()

	req := recoveryPathRequest("nettools")
	err := newTestClient(api).RecoverRules(context.Background(), req)
	require.ErrorIs(t, err, inspectErr)
	require.ErrorIs(t, err, cleanupErr)
}

func TestRemoveRecoverySidecarReturnsLoadAndInfoFailures(t *testing.T) {
	identity := ctr.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "run", Kind: ctr.SidecarKindStress}
	loadErr := errors.New("container service unavailable")
	infoErr := errors.New("metadata unavailable")
	t.Run("load error", func(t *testing.T) {
		api := NewMockapiClient(t)
		api.EXPECT().LoadContainer(mock.Anything, identity.ID).Return(nil, loadErr).Once()
		err := (&containerdClient{client: api}).RemoveRecoverySidecar(context.Background(), identity)
		require.ErrorIs(t, err, loadErr)
	})
	t.Run("info error", func(t *testing.T) {
		api := NewMockapiClient(t)
		cntr := NewMockContainer(t)
		api.EXPECT().LoadContainer(mock.Anything, identity.ID).Return(cntr, nil).Once()
		cntr.EXPECT().Info(mock.Anything).Return(containers.Container{}, infoErr).Once()
		err := (&containerdClient{client: api}).RemoveRecoverySidecar(context.Background(), identity)
		require.ErrorIs(t, err, infoErr)
	})
}
