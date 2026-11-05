package podman

import (
	"context"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRecoveryRootlessGuards(t *testing.T) {
	delegate := ctr.NewMockClient(t)
	client := &podmanClient{Client: delegate, rootless: true}
	req := &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "t"}, RunFilter: "old"}
	_, err := client.PlanRecoveryRules(req)
	require.ErrorContains(t, err, "rootless")
	require.ErrorContains(t, client.RecoverRules(context.Background(), req), "rootless")
}

func TestRecoveryRootfulDelegates(t *testing.T) {
	delegate := ctr.NewMockClient(t)
	client := &podmanClient{Client: delegate}
	req := &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "t"}, RunFilter: "old"}
	plan := &ctr.CommandPlan{Via: ctr.ViaExec}
	delegate.EXPECT().PlanRecoveryRules(req).Return(plan, nil).Once()
	delegate.EXPECT().RecoverRules(mock.Anything, req).Return(nil).Once()
	got, err := client.PlanRecoveryRules(req)
	require.NoError(t, err)
	require.Same(t, plan, got)
	require.NoError(t, client.RecoverRules(context.Background(), req))
}
