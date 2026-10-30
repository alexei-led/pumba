package chaos_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commandFunc adapts a func to chaos.Command.
type commandFunc func(ctx context.Context) error

func (f commandFunc) Run(ctx context.Context, _ bool) error { return f(ctx) }

func TestRunChaosCommandMaxDurationMidFault(t *testing.T) {
	var cleanedUp atomic.Bool
	cmd := commandFunc(func(ctx context.Context) error {
		<-ctx.Done() // fault active until the budget cancels it
		cleanup := context.WithoutCancel(ctx)
		require.NoError(t, cleanup.Err(), "cleanup context must survive the budget")
		cleanedUp.Store(true)
		return nil
	})
	gp := &chaos.GlobalParams{MaxDuration: 20 * time.Millisecond}

	err := chaos.RunChaosCommand(context.Background(), cmd, gp)

	require.ErrorIs(t, err, chaos.ErrMaxDurationExceeded)
	assert.True(t, cleanedUp.Load())
	o := chaos.NewOutcome(chaos.Counts{Selected: 1, OK: 1}, err, false)
	assert.Equal(t, chaos.RunAborted, o.Status)
	assert.Equal(t, chaos.CodeTimeout, o.Error.Code)
	assert.Equal(t, 1, o.ExitCode)
}

func TestRunChaosCommandMaxDurationStopsIntervalBeforeNextTick(t *testing.T) {
	var runs atomic.Int32
	cmd := commandFunc(func(context.Context) error { runs.Add(1); return nil })
	gp := &chaos.GlobalParams{Interval: time.Hour, MaxDuration: 20 * time.Millisecond}

	start := time.Now()
	err := chaos.RunChaosCommand(context.Background(), cmd, gp)

	require.ErrorIs(t, err, chaos.ErrMaxDurationExceeded)
	assert.Equal(t, int32(1), runs.Load(), "no tick may start after the budget")
	assert.Less(t, time.Since(start), time.Minute)
}

func TestRunChaosCommandMaxDurationNotSkipped(t *testing.T) {
	cmd := commandFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return errors.New("interrupted")
	})
	gp := &chaos.GlobalParams{MaxDuration: 10 * time.Millisecond, SkipErrors: true, Interval: time.Hour}

	err := chaos.RunChaosCommand(context.Background(), cmd, gp)

	require.ErrorIs(t, err, chaos.ErrMaxDurationExceeded)
	assert.ErrorContains(t, err, "interrupted")
}

func TestRunChaosCommandUnlimitedByDefault(t *testing.T) {
	cmd := commandFunc(func(ctx context.Context) error {
		_, hasDeadline := ctx.Deadline()
		assert.False(t, hasDeadline)
		return nil
	})
	require.NoError(t, chaos.RunChaosCommand(context.Background(), cmd, &chaos.GlobalParams{}))
}

func TestRunChaosCommandSignalIsNotATimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := commandFunc(func(ctx context.Context) error {
		cancel()
		<-ctx.Done()
		return nil
	})
	gp := &chaos.GlobalParams{MaxDuration: time.Hour, Interval: time.Hour}
	assert.NoError(t, chaos.RunChaosCommand(ctx, cmd, gp), "SIGINT/SIGTERM keeps exit 0")
}
