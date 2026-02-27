//go:build integration

package integration

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	containerdNamespace     = "moby"
	containerdAlpineImage   = "docker.io/library/alpine:latest"
	containerdNetshootImage = "docker.io/nicolaka/netshoot:latest"
)

// ctrRun creates and starts a containerd container in the given namespace.
// Returns the container ID (same as name for ctr).
func ctrRun(t *testing.T, namespace, name, image string, args ...string) string {
	t.Helper()
	cmdArgs := []string{"ctr", "-n", namespace, "run", "-d"}
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, image, name)
	// Use "top" as PID 1 — it handles signals properly unlike sleep
	cmdArgs = append(cmdArgs, "top")

	out, err := exec.Command("sudo", cmdArgs...).CombinedOutput()
	require.NoError(t, err, "ctr run %s: %s", name, string(out))

	t.Cleanup(func() {
		ctrRemove(t, namespace, name)
	})

	// Wait for RUNNING state
	require.Eventually(t, func() bool {
		return ctrTaskStatus(t, namespace, name) == "RUNNING"
	}, 10*time.Second, 200*time.Millisecond, "container %s not RUNNING", name)

	return name
}

// ctrRunSh creates a containerd container that runs a shell command.
func ctrRunSh(t *testing.T, namespace, name, image, shellCmd string, extraArgs ...string) string {
	t.Helper()
	cmdArgs := []string{"ctr", "-n", namespace, "run", "-d"}
	cmdArgs = append(cmdArgs, extraArgs...)
	cmdArgs = append(cmdArgs, image, name, "sh", "-c", shellCmd)

	out, err := exec.Command("sudo", cmdArgs...).CombinedOutput()
	require.NoError(t, err, "ctr run %s: %s", name, string(out))

	t.Cleanup(func() {
		ctrRemove(t, namespace, name)
	})

	require.Eventually(t, func() bool {
		return ctrTaskStatus(t, namespace, name) == "RUNNING"
	}, 10*time.Second, 200*time.Millisecond, "container %s not RUNNING", name)

	return name
}

// ctrTaskStatus returns the task status (RUNNING, STOPPED, etc.) for a containerd container.
func ctrTaskStatus(t *testing.T, namespace, name string) string {
	t.Helper()
	out, err := exec.Command("sudo", "ctr", "-n", namespace, "t", "ls").CombinedOutput()
	if err != nil {
		return "NOT_FOUND"
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == name { //nolint:mnd
			return fields[2]
		}
	}
	return "NOT_FOUND"
}

// ctrRemove kills the task and removes the containerd container.
func ctrRemove(t *testing.T, namespace, name string) {
	t.Helper()
	_ = exec.Command("sudo", "ctr", "-n", namespace, "t", "kill", "-s", "SIGKILL", name).Run()
	// Poll until task stops instead of sleeping
	for i := range 10 {
		s := ctrTaskStatus(t, namespace, name)
		if s == "STOPPED" || s == "NOT_FOUND" {
			break
		}
		if i == 9 {
			t.Logf("warning: task %s did not stop after 5s", name)
		}
		time.Sleep(500 * time.Millisecond) //nolint:mnd
	}
	_ = exec.Command("sudo", "ctr", "-n", namespace, "c", "rm", name).Run()
}

// ctrPullImage ensures an image is available in the given containerd namespace.
func ctrPullImage(t *testing.T, namespace, image string) {
	t.Helper()
	// Check if already present
	err := exec.Command("sudo", "ctr", "-n", namespace, "i", "check", image).Run()
	if err == nil {
		return
	}
	out, err := exec.Command("sudo", "ctr", "-n", namespace, "i", "pull", image).CombinedOutput()
	require.NoError(t, err, "ctr pull %s: %s", image, string(out))
}

func TestContainerd_NonExistentNamespace(t *testing.T) {
	requireContainerd(t)
	t.Parallel()

	name := uniqueName(t, "ctrd")

	_, stderr, err := runPumba(t,
		"--runtime", "containerd",
		"--containerd-namespace", "bogus",
		"--log-level", "debug",
		"kill", name,
	)

	// Pumba should exit cleanly (no matching containers), but log the namespace issue
	// or fail if the namespace doesn't exist at all
	t.Logf("stderr: %s", stderr)
	if err != nil {
		// Error path: pumba failed to connect or list in bogus namespace
		assert.Contains(t, stderr, "bogus", "error should mention the namespace")
	}
	// Either way, pumba should not panic
}

func TestContainerd_PureContainerKill(t *testing.T) {
	requireContainerd(t)
	t.Parallel()

	name := uniqueName(t, "ctrd")
	ctrPullImage(t, containerdNamespace, containerdAlpineImage)
	ctrRun(t, containerdNamespace, name, containerdAlpineImage)

	require.Equal(t, "RUNNING", ctrTaskStatus(t, containerdNamespace, name))

	_, stderr, err := runPumba(t,
		"--runtime", "containerd",
		"--containerd-namespace", containerdNamespace,
		"--log-level", "debug",
		"kill", name,
	)
	t.Logf("stderr: %s", stderr)
	require.NoError(t, err, "pumba kill should succeed")

	// Container task should be STOPPED or removed
	require.Eventually(t, func() bool {
		status := ctrTaskStatus(t, containerdNamespace, name)
		return status == "STOPPED" || status == "NOT_FOUND"
	}, 10*time.Second, 500*time.Millisecond, "container %s should be stopped after kill", name)
}

func TestContainerd_PureContainerPause(t *testing.T) {
	requireContainerd(t)
	t.Parallel()

	name := uniqueName(t, "ctrd")
	ctrPullImage(t, containerdNamespace, containerdAlpineImage)
	ctrRun(t, containerdNamespace, name, containerdAlpineImage)

	require.Equal(t, "RUNNING", ctrTaskStatus(t, containerdNamespace, name))

	// Pause with a short duration so pumba unpauses automatically
	_, stderr, err := runPumba(t,
		"--runtime", "containerd",
		"--containerd-namespace", containerdNamespace,
		"--log-level", "debug",
		"pause", "--duration", "3s", name,
	)
	t.Logf("stderr: %s", stderr)
	require.NoError(t, err, "pumba pause should succeed")

	// After pumba exits (duration expired), container should be RUNNING again
	require.Eventually(t, func() bool {
		return ctrTaskStatus(t, containerdNamespace, name) == "RUNNING"
	}, 10*time.Second, 500*time.Millisecond, "container %s should be running after unpause", name)
}

func TestContainerd_NetemWithSidecarCleanup(t *testing.T) {
	requireContainerd(t)
	t.Parallel()

	name := uniqueName(t, "ctrd")
	ctrPullImage(t, containerdNamespace, containerdNetshootImage)

	// Create container with a dummy network interface for netem testing
	ctrRunSh(t, containerdNamespace, name, containerdNetshootImage,
		"ip link add dummy0 type dummy && ip link set dummy0 up && sleep infinity",
		"--privileged",
	)

	// Poll until the dummy0 interface is ready
	require.Eventually(t, func() bool {
		out, err := exec.Command("sudo", "ctr", "-n", containerdNamespace,
			"t", "exec", "--exec-id", fmt.Sprintf("wait-iface-%d", time.Now().UnixNano()),
			name, "ip", "link", "show", "dummy0").CombinedOutput()
		return err == nil && strings.Contains(string(out), "dummy0")
	}, 10*time.Second, 200*time.Millisecond, "dummy0 interface not ready")

	// Run pumba netem in background with short duration
	pp := runPumbaBackground(t,
		"--runtime", "containerd",
		"--containerd-namespace", containerdNamespace,
		"--log-level", "debug",
		"netem",
		"--interface", "dummy0",
		"--duration", "5s",
		"delay", "--time", "100",
		name,
	)

	// Wait for netem to be applied — check via ctr exec
	require.Eventually(t, func() bool {
		out, err := exec.Command("sudo", "ctr", "-n", containerdNamespace,
			"t", "exec", "--exec-id", fmt.Sprintf("check-tc-%d", time.Now().UnixNano()),
			name, "tc", "qdisc", "show", "dev", "dummy0").CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(strings.ToLower(string(out)), "netem")
	}, 10*time.Second, 500*time.Millisecond, "netem rules should be applied")

	// Verify delay value
	out, err := exec.Command("sudo", "ctr", "-n", containerdNamespace,
		"t", "exec", "--exec-id", fmt.Sprintf("verify-tc-%d", time.Now().UnixNano()),
		name, "tc", "qdisc", "show", "dev", "dummy0").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(out), "delay 100", "should show 100ms delay")

	// Wait for pumba to finish (duration 5s) — netem should be cleaned up
	if err := pp.Wait(); err != nil {
		t.Logf("pumba exited with error: %v", err)
	}

	// Verify netem rules are removed after pumba exits
	require.Eventually(t, func() bool {
		out, err := exec.Command("sudo", "ctr", "-n", containerdNamespace,
			"t", "exec", "--exec-id", fmt.Sprintf("clean-tc-%d", time.Now().UnixNano()),
			name, "tc", "qdisc", "show", "dev", "dummy0").CombinedOutput()
		if err != nil {
			return true // container gone = clean
		}
		return !strings.Contains(strings.ToLower(string(out)), "netem")
	}, 10*time.Second, 500*time.Millisecond, "netem rules should be cleaned up after pumba exits")
}

func TestContainerd_CustomSocketBadPath(t *testing.T) {
	requireContainerd(t)
	t.Parallel()

	name := uniqueName(t, "ctrd")

	_, stderr, err := runPumba(t,
		"--runtime", "containerd",
		"--containerd-socket", "/nonexistent/path/containerd.sock",
		"--containerd-namespace", containerdNamespace,
		"--log-level", "debug",
		"kill", name,
	)

	require.Error(t, err, "pumba should fail with bad socket path")
	t.Logf("stderr: %s", stderr)

	// The error should mention connection failure or the bad path
	stderrLower := strings.ToLower(stderr)
	pathMentioned := strings.Contains(stderrLower, "nonexistent") ||
		strings.Contains(stderrLower, "connect") ||
		strings.Contains(stderrLower, "no such file") ||
		strings.Contains(stderrLower, "socket")
	assert.True(t, pathMentioned, "error should reference the bad socket path or connection failure, got: %s", stderr)
}
