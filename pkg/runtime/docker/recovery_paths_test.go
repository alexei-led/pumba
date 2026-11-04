package docker

import (
	"context"
	"errors"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/tc"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func dockerRecoveryPathRequest() *ctr.RuleRecoveryRequest {
	return &ctr.RuleRecoveryRequest{
		Container: &ctr.Container{ContainerID: "c1"}, RunFilter: "old", InvocationID: "current",
		Rules: []ctr.PumbaRule{{Backend: "iptables-nft", RunID: "old", Chain: "INPUT", Spec: `-A INPUT -m comment --comment "pumba:old" -j DROP`}},
	}
}

func TestDockerPlanRecoveryRules(t *testing.T) {
	client := dockerClient{}
	req := dockerRecoveryPathRequest()
	plan, err := client.PlanRecoveryRules(req)
	require.NoError(t, err)
	want, err := tc.RecoveryDeleteCommand("iptables-nft", "old", "INPUT", `-A INPUT -m comment --comment "pumba:old" -j DROP`)
	require.NoError(t, err)
	require.Equal(t, [][]string{want}, plan.Revert)

	for _, invalid := range []*ctr.RuleRecoveryRequest{nil, {Container: &ctr.Container{ContainerID: "target"}}} {
		_, err := client.PlanRecoveryRules(invalid)
		require.ErrorContains(t, err, "incomplete recovery request")
	}
	invalidRule := dockerRecoveryPathRequest()
	invalidRule.Rules[0].RunID = "different"
	_, err = client.PlanRecoveryRules(invalidRule)
	require.ErrorContains(t, err, "outside run filter")
}

func TestDockerRecoverRulesRejectsBeforeRuntimeAccess(t *testing.T) {
	client := dockerClient{}
	require.ErrorContains(t, client.RecoverRules(context.Background(), &ctr.RuleRecoveryRequest{}), "incomplete recovery request")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, client.RecoverRules(ctx, dockerRecoveryPathRequest()), context.Canceled)
}

func TestDockerRecoverRulesPropagatesInspectionFailures(t *testing.T) {
	t.Run("direct exec", func(t *testing.T) {
		api := NewMockEngine(t)
		expectExec(api, "c1", true, execStep{argv: tc.QdiscShowCommand(), exitCode: 1})
		err := (dockerClient{containerAPI: api}).RecoverRules(context.Background(), dockerRecoveryPathRequest())
		require.ErrorContains(t, err, "failed to list qdiscs")
	})

	t.Run("labeled sidecar and cleanup failure", func(t *testing.T) {
		api := NewMockEngine(t)
		identity := dockerRecoveryPathRequest()
		identity.Sidecar = ctr.SidecarSpec{Image: "nettools"}
		labels := ctr.SidecarLabels(identity.InvocationID, identity.Container.ID(), ctr.SidecarKindInspect)
		expectSidecar(api, identity.Container.ID(), "inspector", labels)
		cleanupErr := errors.New("inspect helper cleanup failed")
		api.EXPECT().ContainerRemove(mock.Anything, "inspector", ctypes.RemoveOptions{Force: true}).Unset()
		api.EXPECT().ContainerRemove(mock.Anything, "inspector", ctypes.RemoveOptions{Force: true}).Return(cleanupErr).Once()
		expectExec(api, "inspector", false, execStep{argv: tc.QdiscShowCommand(), exitCode: 1})
		err := (dockerClient{containerAPI: api, imageAPI: api}).RecoverRules(context.Background(), identity)
		require.ErrorContains(t, err, "failed to list qdiscs")
		require.ErrorIs(t, err, cleanupErr)
	})
}

func TestRemoveRecoverySidecarRejectsIncompleteOrUnavailableMetadata(t *testing.T) {
	identity := ctr.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "run", Kind: ctr.SidecarKindStress}
	inspectErr := errors.New("daemon unavailable")
	cases := []struct {
		name string
		info ctypes.InspectResponse
		err  error
		want string
	}{
		{name: "inspect error", err: inspectErr, want: "daemon unavailable"},
		{name: "missing base", info: ctypes.InspectResponse{Config: &ctypes.Config{}}, want: "incomplete sidecar metadata"},
		{name: "missing config", info: ctypes.InspectResponse{ContainerJSONBase: &ctypes.ContainerJSONBase{ID: identity.ID}}, want: "incomplete sidecar metadata"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := NewMockEngine(t)
			api.EXPECT().ContainerInspect(mock.Anything, identity.ID).Return(tc.info, tc.err).Once()
			err := (dockerClient{containerAPI: api}).RemoveRecoverySidecar(context.Background(), identity)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestRemoveRecoverySidecarReportsVerificationError(t *testing.T) {
	api := NewMockEngine(t)
	identity := ctr.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "run", Kind: ctr.SidecarKindStress}
	info := ctypes.InspectResponse{
		ContainerJSONBase: &ctypes.ContainerJSONBase{ID: identity.ID},
		Config:            &ctypes.Config{Labels: ctr.SidecarLabels(identity.RunID, identity.TargetID, identity.Kind)},
	}
	verifyErr := errors.New("verification unavailable")
	api.EXPECT().ContainerInspect(mock.Anything, identity.ID).Return(info, nil).Once()
	api.EXPECT().ContainerRemove(mock.Anything, identity.ID, ctypes.RemoveOptions{Force: true}).Return(nil).Once()
	api.EXPECT().ContainerInspect(mock.Anything, identity.ID).Return(ctypes.InspectResponse{}, verifyErr).Once()
	err := (dockerClient{containerAPI: api}).RemoveRecoverySidecar(context.Background(), identity)
	require.ErrorIs(t, err, verifyErr)
	require.ErrorContains(t, err, "verify sidecar absence")
}
