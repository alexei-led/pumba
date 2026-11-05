package chaos_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPreparedValidatesDistinctOriginalTargets(t *testing.T) {
	artifact := container.SidecarIdentity{ID: "sidecar-1", TargetID: "target-1", RunID: "old", Kind: container.SidecarKindStress}
	for _, tt := range []struct {
		name    string
		work    chaos.PreparedWork
		max     int
		wantErr string
	}{
		{name: "target and artifact count once", work: chaos.PreparedWork{Targets: makeContainers("target-1"), Orphans: []container.SidecarIdentity{artifact}}, max: 1},
		{name: "multiple artifacts share one target", work: chaos.PreparedWork{Orphans: []container.SidecarIdentity{artifact, {ID: "sidecar-2", TargetID: "target-1", RunID: "old", Kind: container.SidecarKindNetem}}}, max: 1},
		{name: "distinct orphan exceeds cap", work: chaos.PreparedWork{Orphans: []container.SidecarIdentity{artifact, {ID: "sidecar-2", TargetID: "target-2", RunID: "old", Kind: container.SidecarKindNetem}}}, max: 1, wantErr: chaos.ErrMaxTargetsExceeded.Error()},
		{name: "invalid target rejected", work: chaos.PreparedWork{Targets: []*container.Container{nil}}, max: 2, wantErr: "invalid prepared target"},
		{name: "duplicate target rejected", work: chaos.PreparedWork{Targets: append(makeContainers("target-1"), makeContainers("target-1")...)}, max: 2, wantErr: "duplicate prepared target"},
		{name: "invalid artifact rejected", work: chaos.PreparedWork{Orphans: []container.SidecarIdentity{{ID: "sidecar", RunID: "old", Kind: container.SidecarKindStress}}}, max: 2, wantErr: "invalid recovery sidecar identity"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := chaos.CheckPrepared(&chaos.GlobalParams{MaxTargets: tt.max}, tt.work)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestRunPreparedReportsAdmittedAndSkippedRecoveryWork(t *testing.T) {
	boom := errors.New("cleanup failed")
	artifact := container.SidecarIdentity{ID: "sidecar-1", TargetID: "orphan-1", RunID: "old", Kind: container.SidecarKindStress}
	for _, tc := range []struct {
		name          string
		cancel        bool
		artifactErr   error
		want          []chaos.TargetStatus
		wantErr       bool
		wantCallbacks bool
	}{
		{name: "cleanup failure is reported and independent work continues", artifactErr: boom, want: []chaos.TargetStatus{chaos.TargetOK, chaos.TargetCleanupFailed}, wantErr: true, wantCallbacks: true},
		{name: "cancellation skips all work", cancel: true, want: []chaos.TargetStatus{chaos.TargetSkipped, chaos.TargetSkipped}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &recordSink{}
			gp := &chaos.GlobalParams{RunID: "run", Action: "revert", Events: sink}
			ctx := context.Background()
			if tc.cancel {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			var targets, artifacts int
			err := chaos.RunPrepared(ctx, gp, chaos.PreparedWork{Targets: makeContainers("target"), Orphans: []container.SidecarIdentity{artifact}},
				func(context.Context, *container.Container, *chaos.Target) error { targets++; return nil },
				func(context.Context, container.SidecarIdentity) error { artifacts++; return tc.artifactErr })
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			wantCalls := 0
			if tc.wantCallbacks {
				wantCalls = 1
			}
			assert.Equal(t, wantCalls, targets)
			assert.Equal(t, wantCalls, artifacts)
			var statuses []chaos.TargetStatus
			var eventTypes []chaos.EventType
			for _, ev := range sink.all() {
				eventTypes = append(eventTypes, ev.Type)
				if ev.Type == chaos.EventTargetResult {
					statuses = append(statuses, ev.Result.Status)
				}
				if ev.Type == chaos.EventArtifactResult {
					statuses = append(statuses, ev.Artifact.Status)
				}
			}
			assert.Equal(t, tc.want, statuses)
			assert.NotContains(t, eventTypes, chaos.EventTargetApplied)
		})
	}
}

func TestRunPreparedMarksSuccessfulDryRunArtifactsPlanned(t *testing.T) {
	sink := &recordSink{}
	gp := &chaos.GlobalParams{DryRun: true, Events: sink}
	artifact := container.SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "old", Kind: container.SidecarKindStress}
	err := chaos.RunPrepared(context.Background(), gp, chaos.PreparedWork{Orphans: []container.SidecarIdentity{artifact}}, nil,
		func(context.Context, container.SidecarIdentity) error { return nil })
	require.NoError(t, err)
	events := sink.ofType(chaos.EventArtifactResult)
	require.Len(t, events, 1)
	assert.Equal(t, chaos.TargetPlanned, events[0].Artifact.Status)
}

func TestRunPreparedRejectsCapBeforeInvokingCallbacks(t *testing.T) {
	gp := &chaos.GlobalParams{MaxTargets: 1}
	work := chaos.PreparedWork{Orphans: []container.SidecarIdentity{
		{ID: "one", TargetID: "target-1", RunID: "old", Kind: container.SidecarKindStress},
		{ID: "two", TargetID: "target-2", RunID: "old", Kind: container.SidecarKindStress},
	}}
	err := chaos.RunPrepared(context.Background(), gp, work,
		func(context.Context, *container.Container, *chaos.Target) error {
			t.Fatal("target callback must not run")
			return nil
		},
		func(context.Context, container.SidecarIdentity) error {
			t.Fatal("artifact callback must not run")
			return nil
		})
	require.ErrorIs(t, err, chaos.ErrMaxTargetsExceeded)
}
