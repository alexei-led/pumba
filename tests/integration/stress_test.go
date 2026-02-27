//go:build integration

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startStressContainer creates an alpine container with stress-ng installed.
func startStressContainer(t *testing.T, name string, opts ...func(*ContainerOpts)) string {
	t.Helper()
	o := ContainerOpts{
		Name:  name,
		Image: defaultImage,
		Cmd:   []string{"sh", "-c", "apk add --no-cache stress-ng >/dev/null 2>&1 && sleep infinity"},
	}
	for _, fn := range opts {
		fn(&o)
	}
	id := startContainerWithOpts(t, o)

	require.Eventually(t, func() bool {
		out := execInContainer(t, id, []string{"which", "stress-ng"})
		return strings.Contains(out, "stress-ng")
	}, 60*time.Second, time.Second, "stress-ng not installed in container %s", name)

	return id
}

func TestStress_CPUUsage(t *testing.T) {
	t.Parallel()

	name := uniqueName(t, "stress")
	id := startStressContainer(t, name)

	_, stderr, err := runPumba(t, "--log-level", "debug",
		"stress", "--duration", "10s",
		"--stressors", "--cpu 1 --timeout 5s",
		name,
	)
	t.Logf("stderr: %s", stderr)
	assert.NoError(t, err, "pumba stress should succeed")

	require.Equal(t, "running", containerStatus(t, id))
}

func TestStress_MemoryStressor(t *testing.T) {
	t.Parallel()

	name := uniqueName(t, "stress")
	id := startStressContainer(t, name)

	_, stderr, err := runPumba(t, "--log-level", "debug",
		"stress", "--duration", "10s",
		"--stressors", "--vm 1 --vm-bytes 64M --timeout 5s",
		name,
	)
	t.Logf("stderr: %s", stderr)
	assert.NoError(t, err, "pumba stress with memory should succeed")

	require.Equal(t, "running", containerStatus(t, id))
}

func TestStress_CleanupNoLeakedProcesses(t *testing.T) {
	t.Parallel()

	name := uniqueName(t, "stress")
	id := startStressContainer(t, name)

	_, stderr, err := runPumba(t, "--log-level", "debug",
		"stress", "--duration", "10s",
		"--stressors", "--cpu 1 --timeout 5s",
		name,
	)
	assert.NoError(t, err, "pumba stress failed: %s", stderr)

	// After pumba exits, verify no stress-ng processes remain
	require.Eventually(t, func() bool {
		ps := execInContainer(t, id, []string{"ps", "aux"})
		return !strings.Contains(ps, "stress-ng")
	}, 15*time.Second, time.Second, "stress-ng processes still running after pumba exited")
}

func TestStress_WithMemoryLimit(t *testing.T) {
	t.Parallel()

	name := uniqueName(t, "stress")
	id := startStressContainer(t, name, func(o *ContainerOpts) {
		o.Memory = 256 * 1024 * 1024 //nolint:mnd // 256MB
	})

	_, stderr, err := runPumba(t, "--log-level", "debug",
		"stress", "--duration", "10s",
		"--stressors", "--vm 1 --vm-bytes 64M --timeout 5s",
		name,
	)
	t.Logf("stderr: %s", stderr)
	assert.NoError(t, err, "stress within memory limits should succeed")

	require.Equal(t, "running", containerStatus(t, id))
}
