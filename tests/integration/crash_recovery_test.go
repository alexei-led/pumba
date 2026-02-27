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

	// Pumba should exit cleanly (or with a logged error, not hang)
	done := make(chan error, 1)
	go func() {
		done <- pp.Wait()
	}()

	select {
	case <-done:
		// pumba exited — success regardless of exit code
	case <-time.After(30 * time.Second):
		t.Fatal("pumba did not exit within 30s after target was killed")
	}
}

func TestCrashRecovery_TargetOOMDuringNetem(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)
	name := uniqueName(t, "crash")

	const memLimit = 64 * 1024 * 1024 // 64 MB
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

	// Trigger OOM by allocating more memory than the limit inside the container
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	execCfg := container.ExecOptions{
		Cmd: []string{"sh", "-c", "dd if=/dev/zero of=/dev/null bs=128M"},
	}
	execResp, err := dockerCli.ContainerExecCreate(ctx, id, execCfg)
	if err == nil {
		_ = dockerCli.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{})
	}

	// Wait for the container to exit (OOM-killed)
	require.Eventually(t, func() bool {
		status := containerStatus(t, id)
		return status == "exited" || status == "not_found"
	}, 30*time.Second, 500*time.Millisecond, "container not OOM-killed")

	// Pumba should exit
	done := make(chan error, 1)
	go func() {
		done <- pp.Wait()
	}()

	select {
	case <-done:
		// pumba exited
	case <-time.After(30 * time.Second):
		t.Fatal("pumba did not exit within 30s after target OOM")
	}

	// Verify sidecar cleaned up
	require.Eventually(t, func() bool {
		return countSidecars(t) <= sidecarsBefore
	}, 15*time.Second, 500*time.Millisecond, "sidecar not cleaned up after target OOM")
}
