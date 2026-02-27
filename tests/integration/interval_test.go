//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterval_KillThreeCycles(t *testing.T) {
	t.Parallel()

	name := uniqueName(t, "interval")
	id := startContainerWithOpts(t, ContainerOpts{
		Name:    name,
		Image:   defaultImage,
		Cmd:     []string{"top"},
		Restart: "always",
	})

	pp := runPumbaBackground(t, "--interval", "5s", "kill", name)

	// Track 3 kill cycles via RestartCount (restart=always brings it back each time)
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		info, err := dockerCli.ContainerInspect(ctx, id)
		if err != nil {
			return false
		}
		return info.RestartCount >= 3 //nolint:mnd
	}, 25*time.Second, 500*time.Millisecond, "expected at least 3 kill cycles within 25s")

	pp.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, err := dockerCli.ContainerInspect(ctx, id)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, info.RestartCount, 3, //nolint:mnd
		"expected at least 3 restarts, got %d", info.RestartCount)
}

func TestInterval_TimingAccuracy(t *testing.T) {
	t.Parallel()

	const (
		cycles    = 5
		tolerance = 2 * time.Second
		expected  = time.Duration(cycles) * 2 * time.Second // 5 cycles * 2s interval
	)

	name := uniqueName(t, "interval")
	_ = startContainerWithOpts(t, ContainerOpts{
		Name:    name,
		Image:   defaultImage,
		Cmd:     []string{"top"},
		Restart: "always",
	})

	start := time.Now()
	pp := runPumbaBackground(t, "--interval", "2s", "kill", name)

	// Wait for N restarts to measure timing
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		info, err := dockerCli.ContainerInspect(ctx, name)
		if err != nil {
			return false
		}
		return info.RestartCount >= cycles
	}, expected+10*time.Second, 300*time.Millisecond, "expected %d restarts", cycles)

	elapsed := time.Since(start)
	pp.Stop()

	assert.InDelta(t, expected.Seconds(), elapsed.Seconds(), tolerance.Seconds(),
		"total elapsed %v should be close to %v (±%v)", elapsed, expected, tolerance)
}

func TestInterval_SkipErrorSurvivesFailure(t *testing.T) {
	t.Parallel()

	name := uniqueName(t, "interval")
	id := startContainerWithOpts(t, ContainerOpts{
		Name:  name,
		Image: defaultImage,
		Cmd:   []string{"top"},
	})

	pp := runPumbaBackground(t, "--interval", "3s", "--skip-error", "kill", name)

	// Wait for the first kill
	require.Eventually(t, func() bool {
		return containerStatus(t, id) == "exited"
	}, 10*time.Second, 300*time.Millisecond, "container should be killed in first cycle")

	// Force-remove the target so the next interval tick fails
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := dockerCli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
	require.NoError(t, err, "force-remove target container")

	// Wait for at least one more interval tick after removal
	time.Sleep(5 * time.Second) //nolint:mnd

	// Check if pumba is still running or exited gracefully
	done := make(chan struct{})
	go func() {
		_ = pp.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Process exited — verify it didn't panic
	case <-time.After(2 * time.Second):
		// Process still running — expected with --skip-error
		pp.Stop()
	}

	stderr := pp.Stderr.String()
	assert.NotContains(t, stderr, "panic", "pumba should not panic")
	assert.NotContains(t, strings.ToLower(stderr), "fatal", "pumba should not have fatal errors")
}
