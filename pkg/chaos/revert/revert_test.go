package revert

import (
	"context"
	"errors"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type eventLog struct{ events []*chaos.Event }

func (l *eventLog) Emit(e *chaos.Event) { l.events = append(l.events, e) }

func target(id string) *container.Container {
	return &container.Container{ContainerID: id, ContainerName: id, State: container.StateRunning}
}
func artifact(id, parent string) *container.Container {
	return &container.Container{ContainerID: id, ContainerName: "helper-" + id, State: container.StateExited, Labels: container.SidecarLabels("old", parent, container.SidecarKindStress)}
}
func identity(c *container.Container) container.SidecarIdentity {
	return container.SidecarIdentity{ID: c.ID(), TargetID: c.Labels[container.TargetLabel], RunID: "old", Kind: container.SidecarKindStress}
}
func inspectReq(c *container.Container) *container.InspectRequest {
	return &container.InspectRequest{Container: c, RunID: "new"}
}
func recoverReq(c *container.Container, rules []container.PumbaRule) *container.RuleRecoveryRequest {
	return &container.RuleRecoveryRequest{Container: c, RunFilter: "old", InvocationID: "new", Rules: rules}
}
func rule() container.PumbaRule {
	return container.PumbaRule{Backend: "iptables-nft", RunID: "old", Chain: "INPUT", Spec: "-A INPUT -m comment --comment pumba:old -j DROP"}
}
func expectList(t *testing.T, client *container.MockClient, all bool, candidates []*container.Container, gap bool, err error) {
	t.Helper()
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(o container.ListOpts) bool { return o.All == all && len(o.Labels) == 0 && o.OnSkip != nil })).RunAndReturn(
		func(_ context.Context, filter container.FilterFunc, opts container.ListOpts) ([]*container.Container, error) {
			if gap {
				opts.OnSkip("unreadable", errors.New("denied"))
			}
			var found []*container.Container
			for _, c := range candidates {
				if filter(c) {
					found = append(found, c)
				}
			}
			return found, err
		}).Once()
}
func commandFor(t *testing.T, client *container.MockClient, gp *chaos.GlobalParams) (chaos.Command, *eventLog, *chaos.Recorder) {
	t.Helper()
	log := &eventLog{}
	rec := chaos.NewRecorder(log, "test")
	gp.RunID, gp.Action, gp.Events = "new", "revert", rec
	cmd, err := NewCommand(client, gp, Params{RunFilter: "old"})
	require.NoError(t, err)
	gp.Emit(&chaos.Event{Type: chaos.EventRunStart, Start: &chaos.RunStart{}})
	return cmd, log, rec
}

func TestRecoveryInventoriesAllTargetsBeforeMutation(t *testing.T) {
	client := container.NewMockClient(t)
	a, b := target("a"), target("b")
	sc := artifact("sc", "a")
	expectList(t, client, false, []*container.Container{a, b}, false, nil)
	expectList(t, client, true, []*container.Container{sc}, false, nil)
	inspected := 0
	for _, c := range []*container.Container{a, b} {
		client.EXPECT().InspectNetns(mock.Anything, inspectReq(c)).Run(func(context.Context, *container.InspectRequest) { inspected++ }).Return(&container.FaultState{Rules: []container.PumbaRule{rule()}}, nil).Once()
		client.EXPECT().PlanRecoveryRules(recoverReq(c, []container.PumbaRule{rule()})).Return(&container.CommandPlan{Via: container.ViaExec, Revert: [][]string{{"iptables-nft", "-D", "INPUT"}}}, nil).Once()
		client.EXPECT().RecoverRules(mock.Anything, recoverReq(c, []container.PumbaRule{rule()})).Run(func(context.Context, *container.RuleRecoveryRequest) { require.Equal(t, 2, inspected) }).Return(nil).Once()
		client.EXPECT().InspectNetns(mock.Anything, inspectReq(c)).Return(&container.FaultState{}, nil).Once()
	}
	client.EXPECT().RemoveRecoverySidecar(mock.Anything, identity(sc)).Return(nil).Once()
	expectList(t, client, true, nil, false, nil)
	cmd, log, rec := commandFor(t, client, &chaos.GlobalParams{})
	require.NoError(t, cmd.Run(context.Background(), false))
	end, out := rec.RunEnd(nil, false)
	require.Equal(t, chaos.RunOK, out.Status)
	require.Equal(t, 2, end.End.Counts.Selected)
	require.Equal(t, 0, end.End.ArtifactCounts.Selected)
	for _, e := range log.events {
		require.NotEqual(t, chaos.EventTargetApplied, e.Type)
	}
}

func TestRecoveryOrphansAndSelectors(t *testing.T) {
	for _, tt := range []struct {
		name          string
		names         []string
		pattern       string
		max           int
		wantArtifacts int
		wantErr       string
	}{
		{name: "orphan-only", wantArtifacts: 2, wantErr: "namespace coverage unavailable"},
		{name: "exact original ID", names: []string{"gone"}, wantArtifacts: 2, wantErr: "not found"},
		{name: "no name inference", names: []string{"helper-s1"}, wantErr: "not found"},
		{name: "no prefix inference", names: []string{"gon"}, wantErr: "not found"},
		{name: "no regex inference", pattern: ".*"},
		{name: "one cap for two artifacts", max: 1, wantArtifacts: 2, wantErr: "namespace coverage unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			s1, s2 := artifact("s1", "gone"), artifact("s2", "gone")
			expectList(t, client, false, nil, false, nil)
			expectList(t, client, true, []*container.Container{s1, s2}, false, nil)
			if tt.wantArtifacts > 0 {
				for _, sc := range []*container.Container{s1, s2} {
					client.EXPECT().RemoveRecoverySidecar(mock.Anything, identity(sc)).Return(nil).Once()
				}
			}
			expectList(t, client, true, nil, false, nil)
			cmd, log, rec := commandFor(t, client, &chaos.GlobalParams{Names: tt.names, Pattern: tt.pattern, MaxTargets: tt.max})
			err := cmd.Run(context.Background(), false)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			end, out := rec.RunEnd(err, false)
			require.Zero(t, end.End.Counts.Selected)
			require.Equal(t, tt.wantArtifacts, end.End.ArtifactCounts.Selected)
			if tt.wantArtifacts > 0 {
				require.Equal(t, chaos.RunPartial, out.Status)
				require.Equal(t, 1, out.ExitCode)
			}
			for _, e := range log.events {
				require.NotEqual(t, chaos.EventTargetApplied, e.Type)
				require.NotEqual(t, chaos.EventTargetResult, e.Type)
			}
		})
	}
}

func TestRecoveryRefusesIncompleteInventoryBeforeInspection(t *testing.T) {
	for _, tt := range []struct {
		name                   string
		gap, listFail, invalid bool
		cap                    int
	}{
		{name: "combined cap", cap: 1}, {name: "listing gap", gap: true}, {name: "failed listing", listFail: true}, {name: "invalid labels", invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			sc := artifact("sc", "gone")
			if tt.invalid {
				sc.Labels[container.SkipLabel] = "false"
			}
			var listErr error
			if tt.listFail {
				listErr = errors.New("listing failed")
			}
			expectList(t, client, false, []*container.Container{target("live")}, tt.gap, listErr)
			expectList(t, client, true, []*container.Container{sc}, false, nil)
			cmd, _, _ := commandFor(t, client, &chaos.GlobalParams{MaxTargets: tt.cap})
			require.Error(t, cmd.Run(context.Background(), false))
		})
	}
}

func TestRecoveryDryRun(t *testing.T) {
	client := container.NewMockClient(t)
	a := target("a")
	sc, orphan := artifact("sc", "a"), artifact("orphan", "gone")
	expectList(t, client, false, []*container.Container{a}, false, nil)
	expectList(t, client, true, []*container.Container{sc, orphan}, false, nil)
	client.EXPECT().InspectNetns(mock.Anything, inspectReq(a)).Return(&container.FaultState{Rules: []container.PumbaRule{rule()}}, nil).Once()
	plan := &container.CommandPlan{Via: container.ViaExec, Revert: [][]string{{"iptables-nft", "-t", "filter", "-D", "INPUT"}}}
	client.EXPECT().PlanRecoveryRules(recoverReq(a, []container.PumbaRule{rule()})).Return(plan, nil).Once()
	cmd, log, rec := commandFor(t, client, &chaos.GlobalParams{DryRun: true})
	err := cmd.Run(context.Background(), false)
	require.ErrorContains(t, err, "namespace coverage unavailable")
	require.Len(t, log.events, 3)
	result := log.events[1].Result
	require.Equal(t, chaos.TargetPlanned, result.Status)
	require.Equal(t, plan.Revert, result.Recovery.Rules.Revert)
	require.Equal(t, []container.SidecarIdentity{identity(sc)}, result.Recovery.Sidecars)
	end, _ := rec.RunEnd(err, false)
	require.Equal(t, 1, end.End.Counts.Planned)
	require.Equal(t, 1, end.End.ArtifactCounts.Planned)
}

func TestRecoveryEmptyIsIdempotentAndRandomRejected(t *testing.T) {
	client := container.NewMockClient(t)
	expectList(t, client, false, nil, false, nil)
	expectList(t, client, true, nil, false, nil)
	expectList(t, client, true, nil, false, nil)
	cmd, _, rec := commandFor(t, client, &chaos.GlobalParams{})
	require.NoError(t, cmd.Run(context.Background(), false))
	_, out := rec.RunEnd(nil, false)
	require.Equal(t, chaos.RunOK, out.Status)
	require.ErrorContains(t, cmd.Run(context.Background(), true), "random")
}

func TestRecoveryUninspectableTargetStillRemovesSidecar(t *testing.T) {
	client := container.NewMockClient(t)
	a := target("a")
	sc := artifact("sc", "a")
	expectList(t, client, false, []*container.Container{a}, false, nil)
	expectList(t, client, true, []*container.Container{sc}, false, nil)
	client.EXPECT().InspectNetns(mock.Anything, inspectReq(a)).Return(nil, errors.New("target stopped")).Twice()
	client.EXPECT().RemoveRecoverySidecar(mock.Anything, identity(sc)).Return(nil).Once()
	expectList(t, client, true, nil, false, nil)
	cmd, _, _ := commandFor(t, client, &chaos.GlobalParams{})
	require.ErrorContains(t, cmd.Run(context.Background(), false), "unresolved coverage")
}

func TestRecoveryCanceledAfterFirstArtifact(t *testing.T) {
	client := container.NewMockClient(t)
	s1, s2 := artifact("s1", "gone"), artifact("s2", "gone")
	expectList(t, client, false, nil, false, nil)
	expectList(t, client, true, []*container.Container{s1, s2}, false, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.EXPECT().RemoveRecoverySidecar(mock.Anything, identity(s1)).Run(func(context.Context, container.SidecarIdentity) { cancel() }).Return(nil).Once()
	cmd, _, rec := commandFor(t, client, &chaos.GlobalParams{})
	err := cmd.Run(ctx, false)
	require.ErrorIs(t, err, context.Canceled)
	end, _ := rec.RunEnd(err, true)
	require.Equal(t, 1, end.End.ArtifactCounts.OK)
	require.Equal(t, 1, end.End.ArtifactCounts.Skipped)
}

func TestRecoveryPlanFailureStillAttemptsEveryPreflight(t *testing.T) {
	client := container.NewMockClient(t)
	a, b := target("a"), target("b")
	expectList(t, client, false, []*container.Container{a, b}, false, nil)
	expectList(t, client, true, nil, false, nil)
	for _, c := range []*container.Container{a, b} {
		client.EXPECT().InspectNetns(mock.Anything, inspectReq(c)).Return(&container.FaultState{Rules: []container.PumbaRule{rule()}}, nil).Once()
		client.EXPECT().PlanRecoveryRules(recoverReq(c, []container.PumbaRule{rule()})).Return(nil, errors.New("unsafe spec")).Once()
	}
	cmd, _, _ := commandFor(t, client, &chaos.GlobalParams{})
	require.ErrorContains(t, cmd.Run(context.Background(), false), "unsafe spec")
}

func TestRecoveryContinuesIndependentFailures(t *testing.T) {
	client := container.NewMockClient(t)
	a, b := target("a"), target("b")
	expectList(t, client, false, []*container.Container{a, b}, false, nil)
	expectList(t, client, true, nil, false, nil)
	for _, c := range []*container.Container{a, b} {
		client.EXPECT().InspectNetns(mock.Anything, inspectReq(c)).Return(&container.FaultState{}, nil).Twice()
		client.EXPECT().PlanRecoveryRules(recoverReq(c, nil)).Return(&container.CommandPlan{}, nil).Once()
		var failure error
		if c == a {
			failure = errors.New("cleanup failed")
		}
		client.EXPECT().RecoverRules(mock.Anything, recoverReq(c, nil)).Return(failure).Once()
	}
	expectList(t, client, true, nil, false, nil)
	cmd, _, rec := commandFor(t, client, &chaos.GlobalParams{})
	err := cmd.Run(context.Background(), false)
	require.ErrorContains(t, err, "cleanup failed")
	end, out := rec.RunEnd(err, false)
	require.Equal(t, 1, end.End.Counts.CleanupFailed)
	require.Equal(t, 1, end.End.Counts.OK)
	require.Equal(t, chaos.RunCleanupFailed, out.Status)
}

func TestRecoveryVerificationAndQdiscFailures(t *testing.T) {
	for _, tt := range []struct {
		name           string
		initial, final *container.FaultState
		finalErr       error
		relist         []*container.Container
		gap            bool
		wantErr        string
	}{
		{name: "qdisc unresolved", initial: &container.FaultState{Qdiscs: []container.PumbaQdisc{{Device: "lo", Ownership: container.QdiscSuspicious}}}, final: &container.FaultState{}, wantErr: "qdiscs are not attributable"},
		{name: "remaining rule", initial: &container.FaultState{}, final: &container.FaultState{Rules: []container.PumbaRule{rule()}}, wantErr: "remaining rules"},
		{name: "target vanished", initial: &container.FaultState{}, finalErr: errors.New("target disappeared"), wantErr: "target disappeared"},
		{name: "new sidecar", initial: &container.FaultState{}, final: &container.FaultState{}, relist: []*container.Container{artifact("new-sc", "a")}, wantErr: "remains or appeared"},
		{name: "relist gap", initial: &container.FaultState{}, final: &container.FaultState{}, gap: true, wantErr: "verification gap"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			a := target("a")
			expectList(t, client, false, []*container.Container{a}, false, nil)
			expectList(t, client, true, nil, false, nil)
			client.EXPECT().InspectNetns(mock.Anything, inspectReq(a)).Return(tt.initial, nil).Once()
			client.EXPECT().PlanRecoveryRules(recoverReq(a, nil)).Return(&container.CommandPlan{}, nil).Once()
			client.EXPECT().RecoverRules(mock.Anything, recoverReq(a, nil)).Return(nil).Once()
			client.EXPECT().InspectNetns(mock.Anything, inspectReq(a)).Return(tt.final, tt.finalErr).Once()
			expectList(t, client, true, tt.relist, tt.gap, nil)
			cmd, _, rec := commandFor(t, client, &chaos.GlobalParams{})
			err := cmd.Run(context.Background(), false)
			require.ErrorContains(t, err, tt.wantErr)
			_, out := rec.RunEnd(err, false)
			require.Equal(t, 1, out.ExitCode)
		})
	}
}

func TestRecoveryRejectsCancellationBeforeInventory(t *testing.T) {
	client := container.NewMockClient(t)
	cmd, _, _ := commandFor(t, client, &chaos.GlobalParams{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, cmd.Run(ctx, false), context.Canceled)
}
