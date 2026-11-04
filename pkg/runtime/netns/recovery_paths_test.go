package netns

import (
	"context"
	"errors"
	"strings"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/require"
)

func TestRecoveryPlanRejectsIncompleteOrUnauthorizedSnapshots(t *testing.T) {
	rule := recoveryRule("iptables-nft")
	for _, tc := range []struct {
		name string
		req  *ctr.RuleRecoveryRequest
		want string
	}{
		{name: "nil request", want: "incomplete recovery request"},
		{name: "missing target", req: &ctr.RuleRecoveryRequest{RunFilter: "old"}, want: "incomplete recovery request"},
		{name: "missing run filter", req: &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "target"}}, want: "incomplete recovery request"},
		{name: "rule outside filter", req: &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "target"}, RunFilter: "other", Rules: []ctr.PumbaRule{rule}}, want: "outside run filter"},
		{name: "unknown backend", req: &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "target"}, RunFilter: "old", Rules: []ctr.PumbaRule{{Backend: "iptables", RunID: "old", Chain: "INPUT", Spec: rule.Spec}}}, want: "unknown recovery backend"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RecoveryPlan(tc.req)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestRecoveryPlanBoundsRenderedOccurrencesAndRecoverRulesStopsOnAdmissionErrors(t *testing.T) {
	rule := recoveryRule("iptables-nft")
	long := rule
	long.Spec = "-A INPUT -s " + strings.Repeat("a", 220) + " -m comment --comment pumba:old -j DROP"
	req := &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "target"}, RunFilter: "old", Rules: make([]ctr.PumbaRule, maxRecoveryRules)}
	for i := range req.Rules {
		req.Rules[i] = long
	}
	_, err := RecoveryPlan(req)
	require.ErrorContains(t, err, "bytes")

	nearLimit := rule
	nearLimit.Spec = "-A INPUT -s " + strings.Repeat("a", maxRecoveryBytes-60) + " -m comment --comment pumba:old -j DROP"
	_, err = RecoveryPlan(&ctr.RuleRecoveryRequest{Container: req.Container, RunFilter: "old", Rules: []ctr.PumbaRule{nearLimit}})
	require.ErrorContains(t, err, "bytes")

	req.Rules = []ctr.PumbaRule{rule}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = RecoverRules(ctx, req, func(context.Context, []string) (string, string, error) {
		t.Fatal("canceled admission must not inspect or mutate")
		return "", "", nil
	})
	require.ErrorIs(t, err, context.Canceled)

	err = RecoverRules(context.Background(), req, func(_ context.Context, argv []string) (string, string, error) {
		if argv[0] == "tc" {
			return "", "", errors.New("cannot inspect")
		}
		return "", "", nil
	})
	require.ErrorContains(t, err, "failed to list qdiscs")
}
