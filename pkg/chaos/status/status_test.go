package status_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/status"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const invocation = "inv"

var (
	tcImage = container.SidecarSpec{Image: "nettools", Pull: true}

	ruleX     = container.PumbaRule{RunID: "X", Chain: "INPUT", Spec: "-A INPUT -i eth0 -m comment --comment pumba:X -j DROP"}
	qdisc     = container.PumbaQdisc{Device: "eth0", Kind: "netem", Ownership: container.QdiscPumbaOwned}
	oddQdisc  = container.PumbaQdisc{Device: "eth0", Kind: "htb", Ownership: container.QdiscSuspicious}
	cleanNets = &container.FaultState{}
)

func running(id, name string) *container.Container {
	return &container.Container{ContainerID: id, ContainerName: name, State: container.StateRunning, Labels: map[string]string{}}
}

func sidecar(id, kind, target, runID, state string) *container.Container {
	return &container.Container{
		ContainerID:   id,
		ContainerName: "pumba-" + id,
		State:         state,
		Labels:        container.SidecarLabels(runID, target, kind),
	}
}

var (
	web = running("web-id", "web")
	db  = running("db-id", "db")
)

// recording captures what the Recorder forwards: events and status reports.
type recording struct {
	mu      sync.Mutex
	results []*chaos.TargetResult
	reports []*chaos.StatusReport
}

func (r *recording) Emit(ev *chaos.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ev.Type == chaos.EventTargetResult {
		r.results = append(r.results, ev.Result)
	}
}

func (r *recording) RecordStatus(rep *chaos.StatusReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports = append(r.reports, rep)
}

// expectListings serves both listings (sidecars, then targets) from world,
// applying the caller's filter like a runtime does. gaps are reported through
// OnSkip.
func expectListings(client *container.MockClient, world []*container.Container, gaps ...string) {
	serve := func(_ context.Context, fn container.FilterFunc, opts container.ListOpts) ([]*container.Container, error) {
		for _, id := range gaps {
			if opts.OnSkip != nil {
				opts.OnSkip(id, errors.New("task status: unavailable"))
			}
		}
		var out []*container.Container
		for _, c := range world {
			if (opts.All || c.State == container.StateRunning) && fn(c) {
				out = append(out, c)
			}
		}
		return out, nil
	}
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"),
		mock.MatchedBy(func(o container.ListOpts) bool { return o.All && o.OnSkip != nil && o.Labels == nil })).
		RunAndReturn(serve).Once()
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
		RunAndReturn(serve).Once()
}

func expectInspect(client *container.MockClient, c *container.Container, fs *container.FaultState, err error) {
	client.EXPECT().InspectNetns(mock.Anything, &container.InspectRequest{Container: c, Sidecar: tcImage, RunID: invocation}).
		Return(fs, err).Once()
}

type inspection struct {
	target *container.Container
	state  *container.FaultState
	err    error
}

func TestStatusVerdicts(t *testing.T) {
	tests := []struct {
		name        string
		names       []string
		runFilter   string
		noCheck     bool
		world       []*container.Container
		gaps        []string
		inspections []inspection
		wantVerdict chaos.Verdict
		wantStatus  chaos.RunStatus
		wantExit    int
		wantErr     bool
		verify      func(t *testing.T, rep *chaos.StatusReport, results []*chaos.TargetResult)
	}{
		{
			name:        "no artifacts is clean",
			world:       []*container.Container{web},
			inspections: []inspection{{web, cleanNets, nil}},
			wantVerdict: chaos.VerdictClean, wantStatus: chaos.RunClean, wantExit: 0,
			verify: func(t *testing.T, rep *chaos.StatusReport, results []*chaos.TargetResult) {
				require.Len(t, results, 1)
				assert.Equal(t, chaos.TargetOK, results[0].Status)
				assert.Equal(t, &chaos.TargetState{Rules: []chaos.StateRule{}, Qdiscs: []chaos.StateQdisc{}, Sidecars: []chaos.StateSidecar{}}, results[0].State)
				assert.Empty(t, rep.Coverage.Reasons)
			},
		},
		{
			name:        "tagged rule of the filtered run is dirty",
			runFilter:   "X",
			world:       []*container.Container{web},
			inspections: []inspection{{web, &container.FaultState{Rules: []container.PumbaRule{ruleX}, Qdiscs: []container.PumbaQdisc{qdisc}}, nil}},
			wantVerdict: chaos.VerdictDirty, wantStatus: chaos.RunDirty, wantExit: 1,
			verify: func(t *testing.T, _ *chaos.StatusReport, results []*chaos.TargetResult) {
				assert.Equal(t, []chaos.StateRule{{RunID: "X", Chain: "INPUT", Spec: ruleX.Spec}}, results[0].State.Rules)
			},
		},
		{
			name:        "another run sees only the unattributed qdisc",
			runFilter:   "Y",
			world:       []*container.Container{web},
			inspections: []inspection{{web, &container.FaultState{Rules: []container.PumbaRule{ruleX}, Qdiscs: []container.PumbaQdisc{qdisc}}, nil}},
			wantVerdict: chaos.VerdictUnknown, wantStatus: chaos.RunUnknown, wantExit: 1,
			verify: func(t *testing.T, rep *chaos.StatusReport, results []*chaos.TargetResult) {
				assert.Empty(t, results[0].State.Rules)
				assert.Equal(t, []chaos.StateQdisc{{Device: "eth0", Kind: "netem", Ownership: "pumba-owned"}}, results[0].State.Qdiscs)
				assert.Equal(t, []string{"web: pumba qdisc on eth0 not attributable to a run"}, rep.Coverage.Reasons)
			},
		},
		{
			name:        "pumba qdisc without run filter is dirty",
			world:       []*container.Container{web},
			inspections: []inspection{{web, &container.FaultState{Qdiscs: []container.PumbaQdisc{qdisc}}, nil}},
			wantVerdict: chaos.VerdictDirty, wantStatus: chaos.RunDirty, wantExit: 1,
		},
		{
			name:        "suspicious qdisc is unknown",
			world:       []*container.Container{web},
			inspections: []inspection{{web, &container.FaultState{Qdiscs: []container.PumbaQdisc{oddQdisc}}, nil}},
			wantVerdict: chaos.VerdictUnknown, wantStatus: chaos.RunUnknown, wantExit: 1,
		},
		{
			name:        "uninspected target is unknown",
			world:       []*container.Container{web, db},
			inspections: []inspection{{web, cleanNets, nil}, {db, nil, errors.New("tc: not found")}},
			wantVerdict: chaos.VerdictUnknown, wantStatus: chaos.RunUnknown, wantExit: 1, wantErr: true,
			verify: func(t *testing.T, rep *chaos.StatusReport, results []*chaos.TargetResult) {
				failed := resultFor(t, results, "db-id")
				assert.Equal(t, chaos.TargetFailed, failed.Status)
				assert.Equal(t, chaos.PhaseInspect, failed.Error.Phase)
				assert.Equal(t, chaos.CodeCommandFailed, failed.Error.Code)
				assert.Equal(t, []string{"db: inspection failed"}, rep.Coverage.Reasons)
			},
		},
		{
			name:        "leftover sidecar of a target is dirty",
			world:       []*container.Container{web, sidecar("s1", container.SidecarKindNetem, "web-id", "X", container.StateExited)},
			inspections: []inspection{{web, cleanNets, nil}},
			wantVerdict: chaos.VerdictDirty, wantStatus: chaos.RunDirty, wantExit: 1,
			verify: func(t *testing.T, _ *chaos.StatusReport, results []*chaos.TargetResult) {
				x := "X"
				assert.Equal(t, []chaos.StateSidecar{{ID: "s1", Name: "pumba-s1", Kind: "netem", State: "exited", TargetID: "web-id", RunID: &x}}, results[0].State.Sidecars)
			},
		},
		{
			name:        "sidecar of another run is filtered out",
			runFilter:   "Y",
			world:       []*container.Container{web, sidecar("s1", container.SidecarKindNetem, "web-id", "X", container.StateRunning)},
			inspections: []inspection{{web, cleanNets, nil}},
			wantVerdict: chaos.VerdictClean, wantStatus: chaos.RunClean, wantExit: 0,
		},
		{
			name:        "inspect sidecars are not findings",
			world:       []*container.Container{web, sidecar("i1", container.SidecarKindInspect, "web-id", "Z", container.StateRunning)},
			inspections: []inspection{{web, cleanNets, nil}},
			wantVerdict: chaos.VerdictClean, wantStatus: chaos.RunClean, wantExit: 0,
		},
		{
			name:        "orphan sidecar is reported without a selection",
			world:       []*container.Container{web, sidecar("s2", container.SidecarKindStress, "gone-id", "", container.StateRunning)},
			inspections: []inspection{{web, cleanNets, nil}},
			wantVerdict: chaos.VerdictDirty, wantStatus: chaos.RunDirty, wantExit: 1,
			verify: func(t *testing.T, rep *chaos.StatusReport, _ []*chaos.TargetResult) {
				assert.Equal(t, []chaos.StateSidecar{{ID: "s2", Name: "pumba-s2", Kind: "stress", State: "running", TargetID: "gone-id"}}, rep.Coverage.OrphanSidecars)
			},
		},
		{
			name:        "orphan sidecar is not reported with a selection",
			names:       []string{"web"},
			world:       []*container.Container{web, sidecar("s2", container.SidecarKindStress, "gone-id", "", container.StateRunning)},
			inspections: []inspection{{web, cleanNets, nil}},
			wantVerdict: chaos.VerdictClean, wantStatus: chaos.RunClean, wantExit: 0,
			verify: func(t *testing.T, rep *chaos.StatusReport, _ []*chaos.TargetResult) {
				assert.Empty(t, rep.Coverage.OrphanSidecars)
			},
		},
		{
			name:        "missing named target is unknown",
			names:       []string{"web", "ghost"},
			world:       []*container.Container{web},
			inspections: []inspection{{web, cleanNets, nil}},
			wantVerdict: chaos.VerdictUnknown, wantStatus: chaos.RunUnknown, wantExit: 1,
			verify: func(t *testing.T, rep *chaos.StatusReport, _ []*chaos.TargetResult) {
				assert.Equal(t, []string{"ghost"}, rep.Coverage.MissingTargets)
			},
		},
		{
			name:        "empty selection is unknown",
			wantVerdict: chaos.VerdictUnknown, wantStatus: chaos.RunUnknown, wantExit: 1,
			verify: func(t *testing.T, rep *chaos.StatusReport, results []*chaos.TargetResult) {
				assert.Empty(t, results)
				assert.Equal(t, []string{"no targets selected"}, rep.Coverage.Reasons)
			},
		},
		{
			name:        "listing gap is unknown",
			world:       []*container.Container{web},
			gaps:        []string{"broken-id"},
			inspections: []inspection{{web, cleanNets, nil}},
			wantVerdict: chaos.VerdictUnknown, wantStatus: chaos.RunUnknown, wantExit: 1,
			verify: func(t *testing.T, rep *chaos.StatusReport, _ []*chaos.TargetResult) {
				assert.Equal(t, []chaos.ListingGap{{ID: "broken-id", Error: "task status: unavailable"}}, rep.Coverage.ListingGaps)
			},
		},
		{
			name:        "dirty wins over unknown",
			names:       []string{"web", "ghost"},
			world:       []*container.Container{web},
			inspections: []inspection{{web, &container.FaultState{Rules: []container.PumbaRule{ruleX}}, nil}},
			wantVerdict: chaos.VerdictDirty, wantStatus: chaos.RunDirty, wantExit: 1,
		},
		{
			name:        "without --check the verdict does not decide the outcome",
			noCheck:     true,
			world:       []*container.Container{web},
			inspections: []inspection{{web, &container.FaultState{Rules: []container.PumbaRule{ruleX}}, nil}},
			wantVerdict: chaos.VerdictDirty, wantStatus: chaos.RunOK, wantExit: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			expectListings(client, tt.world, tt.gaps...)
			for _, in := range tt.inspections {
				expectInspect(client, in.target, in.state, in.err)
			}
			rec := &recording{}
			recorder := chaos.NewRecorder(rec, "test")
			gp := &chaos.GlobalParams{Names: tt.names, RunID: invocation, Events: recorder}
			cmd := status.NewCommand(client, gp, status.Params{RunFilter: tt.runFilter, Check: !tt.noCheck, Sidecar: tcImage})

			err := cmd.Run(context.Background(), false)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, rec.reports, 1)
			rep := rec.reports[0]
			assert.Equal(t, tt.wantVerdict, rep.Coverage.Verdict)
			end, outcome := recorder.RunEnd(err, false)
			assert.Equal(t, tt.wantStatus, end.End.Status)
			assert.Equal(t, tt.wantExit, outcome.ExitCode)
			assert.Same(t, rep.Coverage, end.End.Coverage)
			if tt.verify != nil {
				tt.verify(t, rep, rec.results)
			}
		})
	}
}

func resultFor(t *testing.T, results []*chaos.TargetResult, id string) *chaos.TargetResult {
	t.Helper()
	for _, r := range results {
		if r.Target.ID == id {
			return r
		}
	}
	require.Failf(t, "no result", "target %s", id)
	return nil
}

func TestStatusIgnoresDryRunAndRandom(t *testing.T) {
	client := container.NewMockClient(t)
	expectListings(client, []*container.Container{web, db})
	expectInspect(client, web, cleanNets, nil)
	expectInspect(client, db, cleanNets, nil)
	rec := &recording{}
	gp := &chaos.GlobalParams{RunID: invocation, DryRun: true, Random: true, Events: rec}
	cmd := status.NewCommand(client, gp, status.Params{Sidecar: tcImage})

	require.NoError(t, cmd.Run(context.Background(), true))

	require.Len(t, rec.results, 2, "--random must not narrow the inspection")
	for _, r := range rec.results {
		assert.Equal(t, chaos.TargetOK, r.Status, "--dry-run must not turn results into plans")
	}
	assert.False(t, gp.DryRun)
	assert.False(t, gp.Random)
}

func TestStatusMaxTargetsRefusesBeforeInspecting(t *testing.T) {
	client := container.NewMockClient(t)
	expectListings(client, []*container.Container{web, db})
	rec := &recording{}
	gp := &chaos.GlobalParams{RunID: invocation, MaxTargets: 1, Events: rec}
	cmd := status.NewCommand(client, gp, status.Params{Check: true, Sidecar: tcImage})

	err := cmd.Run(context.Background(), false)

	require.ErrorIs(t, err, chaos.ErrMaxTargetsExceeded)
	assert.Empty(t, rec.reports)
}

func TestStatusListingFailureRecordsUnknown(t *testing.T) {
	socketClosed := errors.New("socket closed")
	tests := []struct {
		name    string
		expect  func(client *container.MockClient)
		wantErr string
		code    chaos.ErrorCode
	}{
		{
			name: "sidecar listing",
			expect: func(client *container.MockClient) {
				client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"),
					mock.MatchedBy(func(o container.ListOpts) bool { return o.All })).
					Return(nil, socketClosed).Once()
			},
			wantErr: "listing pumba sidecars: socket closed",
			code:    chaos.CodeRuntimeUnavailable,
		},
		{
			name: "target listing",
			expect: func(client *container.MockClient) {
				client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"),
					mock.MatchedBy(func(o container.ListOpts) bool { return o.All })).
					Return(nil, nil).Once()
				client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).
					Return(nil, socketClosed).Once()
			},
			wantErr: "selecting targets: ",
			code:    chaos.CodeRuntimeUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			tt.expect(client)
			sink := &recording{}
			rec := chaos.NewRecorder(sink, "1.5.0")
			gp := &chaos.GlobalParams{Names: []string{"web"}, SkipErrors: true, Events: rec}
			cmd := status.NewCommand(client, gp, status.Params{Check: true, RunFilter: "X"})

			err := chaos.RunChaosCommand(context.Background(), cmd, gp)
			require.NoError(t, err, "--skip-error swallows the listing error")

			require.Len(t, sink.reports, 1)
			cov := sink.reports[0].Coverage
			assert.Equal(t, chaos.VerdictUnknown, cov.Verdict)
			require.Len(t, cov.Reasons, 1)
			assert.Contains(t, cov.Reasons[0], "listing failed: "+tt.wantErr)
			assert.Equal(t, "X", cov.RunFilter)

			_, o := rec.RunEnd(err, false)
			assert.Equal(t, chaos.RunUnknown, o.Status)
			assert.Equal(t, 1, o.ExitCode, "--check must not pass a run that inspected nothing")
			require.NotNil(t, o.Error)
			assert.Equal(t, tt.code, o.Error.Code)
		})
	}
}
