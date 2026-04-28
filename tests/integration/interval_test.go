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

func intervalMarkerCount(t *testing.T, id string) int {
	t.Helper()
	out := execInContainer(t, id, []string{"sh", "-c", "cat /tmp/pumba_interval 2>/dev/null || true"})
	return strings.Count(out, "x")
}

func TestInterval_KillThreeCycles(t *testing.T) {
	t.Parallel()

	name := uniqueName(t, "interval")
	id := startContainerWithOpts(t, ContainerOpts{
		Name:  name,
		Image: defaultImage,
		Cmd:   []string{"top"},
	})

	pp := runPumbaBackground(t, "--interval", "2s",
		"exec", "--command", "sh", "--args", "-c", "--args", "printf x >> /tmp/pumba_interval", name)

	require.Eventually(t, func() bool {
		return intervalMarkerCount(t, id) >= 3 //nolint:mnd
	}, 8*time.Second, 300*time.Millisecond, "expected at least 3 interval cycles")

	pp.Stop()
	assert.GreaterOrEqual(t, intervalMarkerCount(t, id), 3) //nolint:mnd
}

func TestInterval_TimingAccuracy(t *testing.T) {
	t.Parallel()

	const (
		cycles    = 5
		tolerance = 2 * time.Second
		expected  = time.Duration(cycles-1) * 2 * time.Second
	)

	name := uniqueName(t, "interval")
	id := startContainerWithOpts(t, ContainerOpts{
		Name:  name,
		Image: defaultImage,
		Cmd:   []string{"top"},
	})

	start := time.Now()
	pp := runPumbaBackground(t, "--interval", "2s",
		"exec", "--command", "sh", "--args", "-c", "--args", "printf x >> /tmp/pumba_interval", name)

	require.Eventually(t, func() bool {
		return intervalMarkerCount(t, id) >= cycles
	}, expected+6*time.Second, 300*time.Millisecond, "expected %d interval cycles", cycles)

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
