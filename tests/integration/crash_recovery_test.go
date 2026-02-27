//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrashRecovery_SIGKILLDuringNetem(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)
	name := uniqueName(t, "crash")
	id := startContainer(t, name)
	pid := containerPID(t, id)

	pp := runPumbaBackground(t, "--log-level", "debug",
		"netem", "--duration", "60s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "100", name)

	waitForNetem(t, pid, "eth0", 30*time.Second)

	// SIGKILL pumba — no cleanup handler runs
	require.NoError(t, pp.Signal("KILL"))
	_ = pp.Wait()

	// Verify netem rules are LEAKED (documents ungraceful shutdown behavior)
	tcOut := nsenterTC(t, pid, "eth0")
	assert.Contains(t, strings.ToLower(tcOut), "netem",
		"expected leaked netem rules after SIGKILL, got: %s", tcOut)
}

func TestCrashRecovery_SIGTERMDuringNetem(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)
	name := uniqueName(t, "crash")
	id := startContainer(t, name)
	pid := containerPID(t, id)

	sidecarsBefore := countSidecars(t)

	pp := runPumbaBackground(t, "--log-level", "debug",
		"netem", "--duration", "60s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "100", name)

	waitForNetem(t, pid, "eth0", 30*time.Second)

	// SIGTERM pumba — should trigger graceful cleanup
	require.NoError(t, pp.Signal("TERM"))
	_ = pp.Wait()

	// Verify tc rules removed
	waitForClean(t, pid, "eth0", 15*time.Second)

	// Verify sidecar removed
	require.Eventually(t, func() bool {
		return countSidecars(t) <= sidecarsBefore
	}, 15*time.Second, 500*time.Millisecond, "sidecar not removed after SIGTERM")
}

func TestCrashRecovery_SIGTERMDuringPause(t *testing.T) {
	t.Parallel()
	name := uniqueName(t, "crash")
	id := startContainer(t, name)

	pp := runPumbaBackground(t, "--log-level", "debug",
		"pause", "--duration", "60s", name)

	// Wait until container is paused
	require.Eventually(t, func() bool {
		return containerStatus(t, id) == "paused"
	}, 15*time.Second, 200*time.Millisecond, "container not paused")

	// SIGTERM pumba — should unpause the container
	require.NoError(t, pp.Signal("TERM"))
	_ = pp.Wait()

	// Verify container returned to running
	require.Eventually(t, func() bool {
		return containerStatus(t, id) == "running"
	}, 15*time.Second, 200*time.Millisecond, "container not running after SIGTERM")
}

func TestCrashRecovery_SIGKILLDuringPause(t *testing.T) {
	t.Parallel()
	name := uniqueName(t, "crash")
	id := startContainer(t, name)

	pp := runPumbaBackground(t, "--log-level", "debug",
		"pause", "--duration", "60s", name)

	// Wait until container is paused
	require.Eventually(t, func() bool {
		return containerStatus(t, id) == "paused"
	}, 15*time.Second, 200*time.Millisecond, "container not paused")

	// SIGKILL pumba — no cleanup handler runs
	require.NoError(t, pp.Signal("KILL"))
	_ = pp.Wait()

	// Verify container stays PAUSED (documents ungraceful shutdown behavior)
	time.Sleep(2 * time.Second)
	status := containerStatus(t, id)
	assert.Equal(t, "paused", status,
		"expected container to stay paused after SIGKILL, got: %s", status)

	// Manual cleanup: unpause so removeContainer in t.Cleanup works
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = dockerCli.ContainerUnpause(ctx, id)
}

func TestCrashRecovery_SIGTERMDuringIPTables(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)
	name := uniqueName(t, "crash")
	id := startContainer(t, name)
	pid := containerPID(t, id)

	pp := runPumbaBackground(t, "--log-level", "debug",
		"iptables", "--duration", "60s",
		"--iptables-image", nettoolsImg, "--pull-image=false",
		"loss", "--probability", "1.0", name)

	waitForIPTables(t, pid, 30*time.Second)

	// SIGTERM pumba — should clean up iptables rules
	require.NoError(t, pp.Signal("TERM"))
	_ = pp.Wait()

	// Verify iptables rules removed
	waitForIPTablesClean(t, pid, 15*time.Second)
}

func TestCrashRecovery_SIGTERMDuringStress(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)
	name := uniqueName(t, "crash")
	_ = startContainer(t, name)

	sidecarsBefore := countSidecars(t)

	pp := runPumbaBackground(t, "--log-level", "debug",
		"stress", "--duration", "60s",
		"--stressors=--cpu 1 --timeout 55s", name)

	// Wait for stress sidecar to appear
	require.Eventually(t, func() bool {
		return countSidecars(t) > sidecarsBefore
	}, 30*time.Second, 500*time.Millisecond, "stress sidecar not created")

	// SIGTERM pumba — should remove sidecar
	require.NoError(t, pp.Signal("TERM"))
	_ = pp.Wait()

	// Verify sidecar removed
	require.Eventually(t, func() bool {
		return countSidecars(t) <= sidecarsBefore
	}, 15*time.Second, 500*time.Millisecond, "stress sidecar not removed after SIGTERM")
}

func TestCrashRecovery_TargetDiesDuringNetem(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)
	name := uniqueName(t, "crash")
	id := startContainer(t, name)
	pid := containerPID(t, id)

	pp := runPumbaBackground(t, "--log-level", "debug",
		"netem", "--duration", "60s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "100", name)

	waitForNetem(t, pid, "eth0", 30*time.Second)

	// Kill the target container while pumba is running
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := dockerCli.ContainerKill(ctx, id, "KILL")
	require.NoError(t, err, "kill target container")

	// Pumba may or may not exit on its own when target dies mid-netem.
	// Give it a chance, then stop it to avoid hanging the test.
	done := make(chan error, 1)
	go func() { done <- pp.Wait() }()
	select {
	case <-done:
		t.Log("pumba exited after target was killed")
	case <-time.After(15 * time.Second):
		t.Log("pumba did not exit after target death, stopping it")
		pp.Stop()
	}
}

func TestCrashRecovery_TargetOOMDuringNetem(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)
	requireCgroupV2(t)
	name := uniqueName(t, "crash")

	// Start container with 32MB memory limit and simple PID 1.
	// memory.oom.group=1 ensures the OOM killer targets all processes in the
	// cgroup (including PID 1), so the container actually dies.
	const memLimit = 32 * 1024 * 1024 // 32 MB
	id := startContainerWithOpts(t, ContainerOpts{
		Name:   name,
		Image:  defaultImage,
		Cmd:    []string{"sleep", "infinity"},
		Memory: memLimit,
	})
	pid := containerPID(t, id)

	sidecarsBefore := countSidecars(t)

	pp := runPumbaBackground(t, "--log-level", "debug",
		"netem", "--duration", "60s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "100", name)

	waitForNetem(t, pid, "eth0", 30*time.Second)

	// Enable OOM group kill so the entire cgroup (including PID 1) is killed
	cgDir := containerCgroupDir(t, id)
	enableOOMGroup(t, cgDir)

	// Trigger real OOM by filling /dev/shm (backed by memory, counts against cgroup limit)
	triggerOOM(t, id)

	// Wait for the container to be killed by the OOM killer
	require.Eventually(t, func() bool {
		status := containerStatus(t, id)
		return status == "exited" || status == "not_found"
	}, 30*time.Second, 500*time.Millisecond, "container not OOM-killed")

	// Verify this was a real OOM kill, not just a signal
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, err := dockerCli.ContainerInspect(ctx, id)
	if err == nil {
		assert.True(t, info.State.OOMKilled, "expected container to be OOM-killed")
	}

	// Pumba may or may not exit on its own when target dies mid-netem.
	done := make(chan error, 1)
	go func() { done <- pp.Wait() }()
	select {
	case <-done:
		t.Log("pumba exited after target was OOM-killed")
	case <-time.After(15 * time.Second):
		t.Log("pumba did not exit after target OOM death, stopping it")
		pp.Stop()
	}

	// Check sidecar cleanup — documents current behavior on OOM
	time.Sleep(2 * time.Second)
	sidecarsAfter := countSidecars(t)
	if sidecarsAfter > sidecarsBefore {
		t.Logf("sidecar leaked after OOM death (before=%d, after=%d)", sidecarsBefore, sidecarsAfter)
	}
}
