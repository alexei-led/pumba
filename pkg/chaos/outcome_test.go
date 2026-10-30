package chaos_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewOutcome(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name      string
		counts    chaos.Counts
		err       error
		aborted   bool
		status    chaos.RunStatus
		code      chaos.ErrorCode
		phase     chaos.Phase
		message   string
		exitCode  int
		wantNoErr bool
	}{
		{name: "ok", counts: chaos.Counts{Selected: 2, OK: 2}, status: chaos.RunOK, wantNoErr: true},
		{name: "dry-run planned", counts: chaos.Counts{Selected: 1, Planned: 1}, status: chaos.RunOK, wantNoErr: true},
		{name: "no targets", status: chaos.RunNoTargets, code: chaos.CodeNoTargets, phase: chaos.PhaseSelect, message: "no containers found"},
		{
			name: "usage error", err: chaos.NewUsageError(errors.New("unset or invalid duration value")),
			status: chaos.RunUsageError, code: chaos.CodeInvalidArgument, phase: chaos.PhaseUsage,
			message: "unset or invalid duration value", exitCode: 1,
		},
		{
			name: "negative max targets", err: chaos.ErrNegativeMaxTargets,
			status: chaos.RunUsageError, code: chaos.CodeInvalidArgument, phase: chaos.PhaseUsage,
			message: "--max-targets must be non-negative", exitCode: 1,
		},
		{
			name: "runtime unavailable", err: chaos.NewRuntimeUnavailableError(errors.New("could not create Docker client: dial")),
			status: chaos.RunFailed, code: chaos.CodeRuntimeUnavailable, phase: chaos.PhaseRuntime,
			message: "could not create Docker client: dial", exitCode: 1,
		},
		{
			name: "refused", err: fmt.Errorf("running kill: %w", chaos.ErrMaxTargetsExceeded),
			status: chaos.RunRefused, code: chaos.CodeMaxTargetsExceeded, phase: chaos.PhaseSelect,
			message: "running kill: refusing to run", exitCode: 1,
		},
		{
			name: "cleanup failed", counts: chaos.Counts{Selected: 1, CleanupFailed: 1}, err: boom,
			status: chaos.RunCleanupFailed, code: chaos.CodeCleanupFailed, phase: chaos.PhaseCleanup, message: "boom", exitCode: 1,
		},
		{
			name: "cleanup failed under skip-error", counts: chaos.Counts{Selected: 2, OK: 1, CleanupFailed: 1},
			status: chaos.RunCleanupFailed, code: chaos.CodeCleanupFailed, phase: chaos.PhaseCleanup, message: "cleanup failed on 1 target(s)",
		},
		{
			name: "cleanup failure beats timeout", counts: chaos.Counts{Selected: 1, CleanupFailed: 1}, err: chaos.ErrMaxDurationExceeded,
			status: chaos.RunCleanupFailed, code: chaos.CodeCleanupFailed, phase: chaos.PhaseCleanup, message: "--max-duration exceeded", exitCode: 1,
		},
		{
			name: "max duration", counts: chaos.Counts{Selected: 1, OK: 1}, err: chaos.ErrMaxDurationExceeded,
			status: chaos.RunAborted, code: chaos.CodeTimeout, phase: chaos.PhaseRuntime, message: "--max-duration exceeded", exitCode: 1,
		},
		{
			name: "failed", counts: chaos.Counts{Selected: 1, Failed: 1}, err: boom,
			status: chaos.RunFailed, code: chaos.CodeCommandFailed, phase: chaos.PhaseInject, message: "boom", exitCode: 1,
		},
		{
			name: "partial", counts: chaos.Counts{Selected: 3, OK: 1, Failed: 1, Skipped: 1}, err: boom,
			status: chaos.RunPartial, code: chaos.CodeCommandFailed, phase: chaos.PhaseInject, message: "boom", exitCode: 1,
		},
		{
			name: "failed under skip-error", counts: chaos.Counts{Selected: 1, Failed: 1},
			status: chaos.RunFailed, code: chaos.CodeCommandFailed, phase: chaos.PhaseInject, message: "1 target(s) failed",
		},
		{
			name: "failure beats abort", counts: chaos.Counts{Selected: 1, Failed: 1}, err: boom, aborted: true,
			status: chaos.RunFailed, code: chaos.CodeCommandFailed, phase: chaos.PhaseInject, message: "boom", exitCode: 1,
		},
		{
			name: "aborted", counts: chaos.Counts{Selected: 1, OK: 1}, aborted: true,
			status: chaos.RunAborted, code: chaos.CodeCanceled, phase: chaos.PhaseRuntime, message: "run canceled",
		},
		{
			name: "aborted before any target", aborted: true,
			status: chaos.RunAborted, code: chaos.CodeCanceled, phase: chaos.PhaseRuntime, message: "run canceled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chaos.NewOutcome(tt.counts, tt.err, tt.aborted)
			assert.Equal(t, tt.status, got.Status)
			assert.Equal(t, tt.exitCode, got.ExitCode)
			if tt.wantNoErr {
				assert.Nil(t, got.Error)
				return
			}
			require.NotNil(t, got.Error)
			assert.Equal(t, chaos.EventError{Code: tt.code, Phase: tt.phase, Message: tt.message}, *got.Error)
		})
	}
}

func TestNewUsageErrorKeepsMessageAndChain(t *testing.T) {
	cause := errors.New("bad flag")
	err := chaos.NewUsageError(cause)
	assert.EqualError(t, err, "bad flag")
	assert.ErrorIs(t, err, cause)
	assert.NoError(t, chaos.NewUsageError(nil))
}

func TestRecorderCountsAndStampsVersion(t *testing.T) {
	next := &recordSink{}
	rec := chaos.NewRecorder(next, "1.5.0")
	gp := &chaos.GlobalParams{RunID: "r1", Action: "netem delay", Events: rec}
	gp.Emit(&chaos.Event{Type: chaos.EventRunStart, Start: &chaos.RunStart{Runtime: "docker"}})
	for _, s := range []chaos.TargetStatus{chaos.TargetOK, chaos.TargetOK, chaos.TargetFailed, chaos.TargetCleanupFailed, chaos.TargetPlanned, chaos.TargetSkipped} {
		gp.Emit(&chaos.Event{Type: chaos.EventTargetResult, Result: &chaos.TargetResult{Status: s}})
	}
	gp.Emit(&chaos.Event{Type: chaos.EventTargetApplied, Applied: &chaos.TargetApplied{Tick: 1}})

	forwarded := next.all()
	require.Len(t, forwarded, 8, "every event is forwarded")
	assert.Equal(t, "1.5.0", forwarded[0].Start.PumbaVersion)

	end, o := rec.RunEnd(errors.New("boom"), false)
	assert.Equal(t, chaos.Counts{Selected: 6, OK: 2, Failed: 1, CleanupFailed: 1, Planned: 1, Skipped: 1}, end.End.Counts)
	assert.Equal(t, chaos.RunCleanupFailed, o.Status)
	assert.Equal(t, 1, o.ExitCode)
	assert.Equal(t, chaos.EventRunEnd, end.Type)
	assert.Equal(t, chaos.EventSchema, end.Schema)
	assert.Equal(t, "r1", end.RunID)
	assert.Equal(t, "netem delay", end.Action)
	assert.Equal(t, o.Status, end.End.Status)
	assert.Equal(t, o.Error, end.End.Error)
}

func TestRecorderRunEndBeforeRunStart(t *testing.T) {
	rec := chaos.NewRecorder(chaos.NopSink{}, "1.5.0")
	end, o := rec.RunEnd(chaos.NewUsageError(errors.New("bad")), false)
	assert.Equal(t, chaos.RunUsageError, o.Status)
	assert.Empty(t, end.RunID)
	assert.Equal(t, chaos.Counts{}, end.End.Counts)
}

func TestRecorderRunEndReportsTickErrorsSkipErrorSwallowed(t *testing.T) {
	listErr := errors.New("dial unix /var/run/docker.sock: connect: no such file")
	tests := []struct {
		name    string
		run     func(context.Context, *chaos.GlobalParams, container.Lister) error
		listErr error
		code    chaos.ErrorCode
	}{
		{
			name: "listing failure",
			run: func(ctx context.Context, gp *chaos.GlobalParams, l container.Lister) error {
				return chaos.RunOnContainers(ctx, l, gp, 0, false, true,
					func(context.Context, *container.Container, *chaos.Target) error { return nil })
			},
			listErr: listErr,
			code:    chaos.CodeRuntimeUnavailable,
		},
		{
			name: "--target resolution failure",
			run: func(context.Context, *chaos.GlobalParams, container.Lister) error {
				return errors.New("failed to resolve --target: no such container")
			},
			code: chaos.CodeCommandFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := container.NewMockClient(t)
			if tt.listErr != nil {
				client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), container.ListOpts{}).Return(nil, tt.listErr).Once()
			}
			rec := chaos.NewRecorder(chaos.NopSink{}, "1.5.0")
			gp := &chaos.GlobalParams{Names: []string{"c1"}, SkipErrors: true, Events: rec}
			cmd := commandFunc(func(ctx context.Context) error { return tt.run(ctx, gp, client) })

			err := chaos.RunChaosCommand(context.Background(), cmd, gp)
			require.NoError(t, err)

			end, o := rec.RunEnd(err, false)
			assert.Equal(t, 0, o.ExitCode)
			assert.Equal(t, chaos.RunFailed, o.Status)
			require.NotNil(t, end.End.Error)
			assert.Equal(t, tt.code, end.End.Error.Code)
		})
	}
}
