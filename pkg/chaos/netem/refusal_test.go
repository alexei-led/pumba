package netem

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type signaledReport struct {
	fakeReport
	applied chan struct{}
}

func (r *signaledReport) Applied() { r.fakeReport.Applied(); close(r.applied) }

// Both targets have identical selectors in the same netns. The first owns
// the active fault; the second's proven refusal must never trigger its cleanup.
func TestRunNetemRefusedSiblingPreservesActiveFault(t *testing.T) {
	client := container.NewMockClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := &container.NetemRequest{Container: &container.Container{ContainerID: "first"}, Interface: "eth0", DPorts: []string{"443"}, Duration: time.Hour}
	second := *first
	second.Container = &container.Container{ContainerID: "sibling"}
	active := true
	client.EXPECT().NetemContainer(ctx, first).Return(nil).Once()
	client.EXPECT().NetemContainer(mock.Anything, &second).Return(fmt.Errorf("startup: %w", container.ErrNetemNotApplied)).Once()
	client.EXPECT().StopNetemContainer(mock.Anything, first).Run(func(context.Context, *container.NetemRequest) { active = false }).Return(nil).Once()
	// Record an erroneous sibling stop instead of failing from a mock goroutine.
	siblingStops := 0
	client.EXPECT().StopNetemContainer(mock.Anything, &second).Run(func(context.Context, *container.NetemRequest) { siblingStops++; active = false }).Return(nil).Maybe()
	report := &signaledReport{applied: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- runNetem(ctx, client, first, report) }()
	<-report.applied
	refusedReport := &fakeReport{}
	err := runNetem(context.Background(), client, &second, refusedReport)
	require.ErrorIs(t, err, container.ErrNetemNotApplied)
	require.Empty(t, refusedReport.calls)
	// Always finish the owner before asserting to avoid leaking a waiter on RED.
	preserved, stops := active, siblingStops
	cancel()
	require.NoError(t, <-done)
	require.True(t, preserved, "refused sibling removed the owner's active fault")
	require.Zero(t, stops)
	require.False(t, active)
	require.Equal(t, []string{"applied", "reverted"}, report.calls)
}

func TestRunNetemUnprovenRefusalStillRollsBack(t *testing.T) {
	for _, message := range []string{
		"partial install failed",
		"failed to read exec output",
		"sidecar cleanup failed",
		"sh -ec script: refusing to replace a foreign or stale root qdisc; exit 78; actual exit code 1",
	} {
		t.Run(message, func(t *testing.T) {
			client := container.NewMockClient(t)
			req := &container.NetemRequest{Container: &container.Container{ContainerID: "target"}, Interface: "eth0"}
			startErr := fmt.Errorf("netem: %s", message)
			client.EXPECT().NetemContainer(mock.Anything, req).Return(startErr).Once()
			client.EXPECT().StopNetemContainer(mock.Anything, req).Return(nil).Once()
			report := &fakeReport{}
			require.ErrorIs(t, runNetem(context.Background(), client, req, report), startErr)
			require.Empty(t, report.calls)
		})
	}
}
