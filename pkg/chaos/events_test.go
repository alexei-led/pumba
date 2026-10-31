package chaos_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordSink keeps every emitted event in order.
type recordSink struct {
	mu     sync.Mutex
	events []chaos.Event
}

func (s *recordSink) Emit(ev *chaos.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, *ev)
}

func (s *recordSink) all() []chaos.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]chaos.Event(nil), s.events...)
}

func (s *recordSink) ofType(typ chaos.EventType) []chaos.Event {
	var out []chaos.Event
	for _, ev := range s.all() {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func TestEventJSONContract(t *testing.T) {
	at := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	header := chaos.Event{Schema: chaos.EventSchema, RunID: "3f9c2a7b1d04", Time: at, Action: "netem delay"}
	target := chaos.TargetRef{ID: "a1b2", Name: "web-1"}
	with := func(f func(*chaos.Event)) chaos.Event {
		ev := header
		f(&ev)
		return ev
	}
	const prefix = `{"schema":"pumba.event/v1","type":`
	const ids = `"run_id":"3f9c2a7b1d04","time":"2026-10-29T10:00:00Z","action":"netem delay"`
	tests := []struct {
		name string
		ev   chaos.Event
		want string
	}{
		{
			name: "run.start",
			ev: with(func(e *chaos.Event) {
				e.Type = chaos.EventRunStart
				e.Start = &chaos.RunStart{
					PumbaVersion: "1.5.0", Runtime: "docker", Interval: "0s",
					Selection: chaos.Selection{Names: []string{"web-1"}, Labels: []string{}, MaxTargets: 2},
					Params:    map[string]any{"duration": "30s", "interface": "eth0", "time": 100, "jitter": 10},
				}
			}),
			want: prefix + `"run.start",` + ids + `,"pumba_version":"1.5.0","runtime":"docker","dry_run":false,"interval":"0s",` +
				`"selection":{"names":["web-1"],"labels":[],"random":false,"max_targets":2},` +
				`"params":{"duration":"30s","interface":"eth0","jitter":10,"time":100}}`,
		},
		{
			name: "target.applied",
			ev: with(func(e *chaos.Event) {
				e.Type = chaos.EventTargetApplied
				e.Applied = &chaos.TargetApplied{Tick: 1, Target: target}
			}),
			want: prefix + `"target.applied",` + ids + `,"tick":1,"target":{"id":"a1b2","name":"web-1"}}`,
		},
		{
			name: "target.result ok",
			ev: with(func(e *chaos.Event) {
				e.Type = chaos.EventTargetResult
				e.Result = &chaos.TargetResult{Tick: 1, Target: target, Started: at, Ended: at.Add(time.Second), Status: chaos.TargetOK, Cleanup: &chaos.Cleanup{OK: true}}
			}),
			want: prefix + `"target.result",` + ids + `,"tick":1,"target":{"id":"a1b2","name":"web-1"},` +
				`"started":"2026-10-29T10:00:00Z","ended":"2026-10-29T10:00:01Z","status":"ok","error":null,"cleanup":{"ok":true}}`,
		},
		{
			name: "target.result cleanup failed",
			ev: with(func(e *chaos.Event) {
				e.Type = chaos.EventTargetResult
				e.Result = &chaos.TargetResult{
					Tick: 2, Target: target, Started: at, Ended: at, Status: chaos.TargetCleanupFailed,
					Error:   &chaos.EventError{Code: chaos.CodeCleanupFailed, Phase: chaos.PhaseCleanup, Message: "stopping netem: boom"},
					Cleanup: &chaos.Cleanup{OK: false, Error: "boom"},
				}
			}),
			want: prefix + `"target.result",` + ids + `,"tick":2,"target":{"id":"a1b2","name":"web-1"},` +
				`"started":"2026-10-29T10:00:00Z","ended":"2026-10-29T10:00:00Z","status":"cleanup_failed",` +
				`"error":{"code":"cleanup_failed","phase":"cleanup","message":"stopping netem: boom"},"cleanup":{"ok":false,"error":"boom"}}`,
		},
		{
			name: "target.result without cleanup",
			ev: with(func(e *chaos.Event) {
				e.Type = chaos.EventTargetResult
				e.Result = &chaos.TargetResult{Tick: 1, Target: target, Started: at, Ended: at, Status: chaos.TargetSkipped}
			}),
			want: prefix + `"target.result",` + ids + `,"tick":1,"target":{"id":"a1b2","name":"web-1"},` +
				`"started":"2026-10-29T10:00:00Z","ended":"2026-10-29T10:00:00Z","status":"skipped","error":null,"cleanup":null}`,
		},
		{
			name: "target.result with status state",
			ev: with(func(e *chaos.Event) {
				e.Type = chaos.EventTargetResult
				e.Result = &chaos.TargetResult{Tick: 1, Target: target, Started: at, Ended: at, Status: chaos.TargetOK, State: &chaos.TargetState{
					Rules:    []chaos.StateRule{{RunID: "r1", Chain: "INPUT", Spec: "-A INPUT -j DROP"}},
					Qdiscs:   []chaos.StateQdisc{{Device: "eth0", Kind: "netem", Ownership: "pumba-owned"}},
					Sidecars: []chaos.StateSidecar{{ID: "s1", Name: "pumba-s1", Kind: "netem", State: "exited", TargetID: "a1b2"}},
				}}
			}),
			want: prefix + `"target.result",` + ids + `,"tick":1,"target":{"id":"a1b2","name":"web-1"},` +
				`"started":"2026-10-29T10:00:00Z","ended":"2026-10-29T10:00:00Z","status":"ok","error":null,"cleanup":null,` +
				`"state":{"rules":[{"run_id":"r1","chain":"INPUT","spec":"-A INPUT -j DROP"}],` +
				`"qdiscs":[{"device":"eth0","kind":"netem","ownership":"pumba-owned","run_id":null}],` +
				`"sidecars":[{"id":"s1","name":"pumba-s1","kind":"netem","state":"exited","target_id":"a1b2","run_id":null}]}}`,
		},
		{
			name: "run.end with status coverage",
			ev: with(func(e *chaos.Event) {
				e.Type = chaos.EventRunEnd
				e.End = &chaos.RunEnd{Status: chaos.RunUnknown, Coverage: &chaos.StatusCoverage{
					Verdict:        chaos.VerdictUnknown,
					Reasons:        []string{"ghost: not found or not running"},
					RunFilter:      "r1",
					OrphanSidecars: []chaos.StateSidecar{},
					MissingTargets: []string{"ghost"},
					ListingGaps:    []chaos.ListingGap{},
				}}
			}),
			want: prefix + `"run.end",` + ids + `,"status":"unknown",` +
				`"counts":{"selected":0,"ok":0,"failed":0,"cleanup_failed":0,"planned":0,"skipped":0},"error":null,"events_dropped":0,` +
				`"coverage":{"verdict":"unknown","reasons":["ghost: not found or not running"],"run_filter":"r1",` +
				`"orphan_sidecars":[],"missing_targets":["ghost"],"listing_gaps":[]}}`,
		},
		{
			name: "run.end",
			ev: with(func(e *chaos.Event) {
				e.Type = chaos.EventRunEnd
				e.End = &chaos.RunEnd{Status: chaos.RunOK, Counts: chaos.Counts{Selected: 1, OK: 1}}
			}),
			want: prefix + `"run.end",` + ids + `,"status":"ok",` +
				`"counts":{"selected":1,"ok":1,"failed":0,"cleanup_failed":0,"planned":0,"skipped":0},"error":null,"events_dropped":0}`,
		},
		{
			name: "run.end refused",
			ev: with(func(e *chaos.Event) {
				e.Type = chaos.EventRunEnd
				e.End = &chaos.RunEnd{
					Status:        chaos.RunRefused,
					Error:         &chaos.EventError{Code: chaos.CodeMaxTargetsExceeded, Phase: chaos.PhaseSelect, Message: "refusing to run"},
					EventsDropped: 3,
				}
			}),
			want: prefix + `"run.end",` + ids + `,"status":"refused",` +
				`"counts":{"selected":0,"ok":0,"failed":0,"cleanup_failed":0,"planned":0,"skipped":0},` +
				`"error":{"code":"max_targets_exceeded","phase":"select","message":"refusing to run"},"events_dropped":3}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.ev)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestGlobalParamsEmitStampsHeader(t *testing.T) {
	sink := &recordSink{}
	gp := &chaos.GlobalParams{RunID: "run-1", Action: "kill", Events: sink}
	gp.Emit(&chaos.Event{Type: chaos.EventTargetApplied, Applied: &chaos.TargetApplied{Tick: 1}})

	events := sink.all()
	require.Len(t, events, 1)
	assert.Equal(t, chaos.EventSchema, events[0].Schema)
	assert.Equal(t, "run-1", events[0].RunID)
	assert.Equal(t, "kill", events[0].Action)
	assert.False(t, events[0].Time.IsZero())
}

func TestGlobalParamsEmitWithoutSinkDiscards(t *testing.T) {
	gp := &chaos.GlobalParams{}
	assert.NotPanics(t, func() { gp.Emit(&chaos.Event{Type: chaos.EventRunStart}) })
}
