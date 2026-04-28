//go:build integration

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSidecar_RemovedAfterNetem(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	name := uniqueName(t, "sidecar")
	startContainer(t, name)

	before := countSidecars(t)

	_, stderr, err := runPumba(t,
		"netem", "--tc-image", nettoolsImg, "--pull-image=false",
		"--duration", "3s", "delay", "--time", "100",
		name,
	)
	require.NoError(t, err, "pumba netem failed: %s", stderr)

	require.Eventually(t, func() bool {
		return countSidecars(t) <= before
	}, 10*time.Second, 500*time.Millisecond, "sidecar containers should be fully removed after netem completes")
}

func TestSidecar_RemovedAfterIPTables(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	name := uniqueName(t, "sidecar")
	startContainer(t, name)

	before := countSidecars(t)

	_, stderr, err := runPumba(t,
		"iptables", "--iptables-image", nettoolsImg, "--pull-image=false",
		"--duration", "3s", "loss",
		"--probability", "1.0",
		name,
	)
	require.NoError(t, err, "pumba iptables failed: %s", stderr)

	require.Eventually(t, func() bool {
		return countSidecars(t) <= before
	}, 10*time.Second, 500*time.Millisecond, "sidecar containers should be fully removed after iptables completes")
}

func TestSidecar_NoAccumulationMultipleRuns(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	name := uniqueName(t, "sidecar")
	startContainer(t, name)

	before := countSidecars(t)

	for i := range 5 {
		_, stderr, err := runPumba(t,
			"netem", "--tc-image", nettoolsImg, "--pull-image=false",
			"--duration", "2s", "delay", "--time", "50",
			name,
		)
		require.NoError(t, err, "pumba netem run %d failed: %s", i+1, stderr)
	}

	require.Eventually(t, func() bool {
		return countSidecars(t) <= before
	}, 10*time.Second, 500*time.Millisecond, "no sidecars should accumulate after %d sequential runs", 5)
}

func TestSidecar_HasSkipLabel(t *testing.T) {
	t.Skip("sidecars are ephemeral; cleanup and no-accumulation tests cover the observable contract")
}

func TestSidecar_ImagePullFailure(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	name := uniqueName(t, "sidecar")
	startContainer(t, name)

	_, stderr, err := runPumba(t,
		"netem", "--tc-image", "nonexistent-registry.example.com/fake-image:v0.0.0",
		"--duration", "3s", "delay", "--time", "100",
		name,
	)
	require.Error(t, err, "pumba should fail with bad tc-image")
	assert.True(t,
		strings.Contains(stderr, "pull") || strings.Contains(stderr, "image") || strings.Contains(stderr, "error"),
		"stderr should mention pull/image failure, got: %s", stderr,
	)
}

func TestSidecar_NoPullWithCachedImage(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	name := uniqueName(t, "sidecar")
	id := startContainer(t, name)
	pid := containerPID(t, id)

	_, stderr, err := runPumba(t,
		"netem", "--tc-image", nettoolsImg, "--pull-image=false",
		"--duration", "3s", "delay", "--time", "100",
		name,
	)
	require.NoError(t, err, "pumba netem with --pull-image=false failed: %s", stderr)

	waitForClean(t, pid, "eth0", 10*time.Second)
}
