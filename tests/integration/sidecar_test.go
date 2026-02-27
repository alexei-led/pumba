//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
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

	time.Sleep(2 * time.Second)

	after := countSidecars(t)
	assert.Equal(t, before, after, "sidecar containers should be fully removed (not just stopped) after netem completes")
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

	time.Sleep(2 * time.Second)

	after := countSidecars(t)
	assert.Equal(t, before, after, "sidecar containers should be fully removed after iptables completes")
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

	time.Sleep(2 * time.Second)

	after := countSidecars(t)
	assert.Equal(t, before, after, "no sidecars should accumulate after %d sequential runs", 5)
}

func TestSidecar_HasSkipLabel(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	name := uniqueName(t, "sidecar")
	id := startContainer(t, name)
	pid := containerPID(t, id)

	pp := runPumbaBackground(t,
		"netem", "--tc-image", nettoolsImg, "--pull-image=false",
		"--duration", "30s", "delay", "--time", "100",
		name,
	)

	waitForNetem(t, pid, "eth0", 15*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sidecars, err := dockerCli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", "com.gaiaadm.pumba.skip=true")),
	})
	require.NoError(t, err, "list sidecar containers")

	found := false
	for _, sc := range sidecars {
		for _, scName := range sc.Names {
			if strings.Contains(scName, name) {
				found = true
				assert.Equal(t, "true", sc.Labels["com.gaiaadm.pumba.skip"],
					"sidecar should have com.gaiaadm.pumba.skip=true label")
				break
			}
		}
		if found {
			break
		}
	}
	require.True(t, found, "expected to find a running sidecar with pumba.skip label for container %s", name)

	pp.Stop()
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
