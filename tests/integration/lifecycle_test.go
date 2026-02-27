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

func TestLifecycle_KillWithRestartPolicy(t *testing.T) {
	t.Parallel()
	name := uniqueName(t, "lifecycle")
	id := startContainerWithOpts(t, ContainerOpts{
		Name:    name,
		Image:   defaultImage,
		Cmd:     []string{"top"},
		Restart: "always",
	})

	pidBefore := containerPID(t, id)

	_, stderr, err := runPumba(t, "--log-level", "debug", "kill", "--signal", "SIGTERM", name)
	assert.NoError(t, err, "pumba kill should exit cleanly, stderr: %s", stderr)

	// Container should auto-restart thanks to restart=always policy
	require.Eventually(t, func() bool {
		return containerStatus(t, id) == "running"
	}, 15*time.Second, 500*time.Millisecond,
		"container with restart=always should restart after kill")

	// PID should change after restart
	pidAfter := containerPID(t, id)
	assert.NotEqual(t, pidBefore, pidAfter, "container PID should change after restart")
}

func TestLifecycle_ShortLivedContainerDuringNetem(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)
	name := uniqueName(t, "lifecycle")

	// Container exits after 2 seconds — well before the 10s netem duration
	id := startContainerWithOpts(t, ContainerOpts{
		Name:  name,
		Image: defaultImage,
		Cmd:   []string{"sleep", "2"},
	})

	pp := runPumbaBackground(t, "--log-level", "debug",
		"netem", "--duration", "10s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "100", name)

	// Wait for container to exit naturally
	require.Eventually(t, func() bool {
		status := containerStatus(t, id)
		return status == "exited" || status == "not_found"
	}, 10*time.Second, 500*time.Millisecond, "short-lived container should exit")

	// Pumba should handle gracefully and exit (not hang or crash)
	done := make(chan error, 1)
	go func() {
		done <- pp.Wait()
	}()

	select {
	case <-done:
		assert.NotContains(t, pp.Stderr.String(), "panic",
			"pumba should not panic when container exits during netem")
	case <-time.After(30 * time.Second):
		t.Fatal("pumba did not exit within 30s after short-lived container died")
	}
}

func TestLifecycle_PauseAlreadyPaused(t *testing.T) {
	t.Parallel()
	name := uniqueName(t, "lifecycle")
	id := startContainer(t, name)

	// Manually pause the container first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := dockerCli.ContainerPause(ctx, id)
	require.NoError(t, err, "manual pause should succeed")

	// Ensure we unpause on cleanup so removeContainer works
	t.Cleanup(func() {
		_ = dockerCli.ContainerUnpause(context.Background(), id)
	})

	require.Equal(t, "paused", containerStatus(t, id))

	// Run pumba pause on an already-paused container
	_, stderr, err := runPumba(t, "--log-level", "debug", "pause", "--duration", "3s", name)

	// Pumba may error or handle idempotently — either way, it should not crash
	if err != nil {
		assert.NotContains(t, strings.ToLower(stderr), "panic",
			"pumba should not panic on already-paused container")
	}

	// After pumba returns, container might be paused or running (if pumba unpaused it)
	time.Sleep(4 * time.Second)
	status := containerStatus(t, id)
	assert.Contains(t, []string{"paused", "running"}, status,
		"container should be paused or running, got: %s", status)
}

func TestLifecycle_RmWithVolumes(t *testing.T) {
	t.Parallel()
	name := uniqueName(t, "lifecycle")

	// Create container with anonymous volume via ContainerCreate directly
	// to capture the volume name before pumba removes it
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := dockerCli.ContainerCreate(ctx,
		&container.Config{
			Image:   defaultImage,
			Cmd:     []string{"sleep", "infinity"},
			Volumes: map[string]struct{}{"/data": {}},
		},
		&container.HostConfig{}, nil, nil, name)
	require.NoError(t, err)

	err = dockerCli.ContainerStart(ctx, resp.ID, container.StartOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = dockerCli.ContainerRemove(context.Background(), resp.ID,
			container.RemoveOptions{Force: true, RemoveVolumes: true})
	})

	// Find the anonymous volume
	info, err := dockerCli.ContainerInspect(ctx, resp.ID)
	require.NoError(t, err)
	require.NotEmpty(t, info.Mounts, "container should have mounts")

	var volName string
	for _, m := range info.Mounts {
		if m.Destination == "/data" {
			volName = m.Name
			break
		}
	}
	require.NotEmpty(t, volName, "should find anonymous volume for /data")

	// Verify volume exists before removal
	_, err = dockerCli.VolumeInspect(ctx, volName)
	require.NoError(t, err, "volume should exist before rm")

	// pumba rm (--volumes defaults to true via BoolTFlag)
	_, stderr, err := runPumba(t, "--log-level", "debug", "rm", name)
	assert.NoError(t, err, "pumba rm should succeed, stderr: %s", stderr)

	// Container should be removed
	require.Eventually(t, func() bool {
		return containerStatus(t, resp.ID) == "not_found"
	}, 10*time.Second, 500*time.Millisecond, "container should be removed")

	// Volume should be deleted
	volCtx, volCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer volCancel()
	_, err = dockerCli.VolumeInspect(volCtx, volName)
	assert.Error(t, err, "anonymous volume should be deleted after rm")
}

func TestLifecycle_StopWithHealthCheck(t *testing.T) {
	t.Parallel()
	name := uniqueName(t, "lifecycle")

	// Create container with healthcheck directly — need HealthConfig which ContainerOpts doesn't support
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := dockerCli.ContainerCreate(ctx,
		&container.Config{
			Image: defaultImage,
			Cmd:   []string{"top"},
			Healthcheck: &container.HealthConfig{
				Test:     []string{"CMD", "true"},
				Interval: 1 * time.Second,
				Timeout:  1 * time.Second,
				Retries:  3,
			},
		},
		&container.HostConfig{}, nil, nil, name)
	require.NoError(t, err)
	t.Cleanup(func() {
		removeContainer(t, resp.ID)
	})

	err = dockerCli.ContainerStart(ctx, resp.ID, container.StartOptions{})
	require.NoError(t, err)

	// Wait for healthy status
	require.Eventually(t, func() bool {
		info, inspectErr := dockerCli.ContainerInspect(context.Background(), resp.ID)
		if inspectErr != nil || info.State.Health == nil {
			return false
		}
		return info.State.Health.Status == "healthy"
	}, 15*time.Second, 500*time.Millisecond, "container should become healthy")

	// pumba stop --restart --duration 5s: stops the container, then restarts it after 5s
	_, stderr, err := runPumba(t, "--log-level", "debug",
		"stop", "--restart", "--duration", "5s", name)
	assert.NoError(t, err, "pumba stop --restart should succeed, stderr: %s", stderr)

	// Container should be running again after pumba restarts it
	require.Eventually(t, func() bool {
		return containerStatus(t, resp.ID) == "running"
	}, 15*time.Second, 500*time.Millisecond, "container should be running after restart")

	// Health check should pass again
	require.Eventually(t, func() bool {
		info, inspectErr := dockerCli.ContainerInspect(context.Background(), resp.ID)
		if inspectErr != nil || info.State.Health == nil {
			return false
		}
		return info.State.Health.Status == "healthy"
	}, 15*time.Second, 500*time.Millisecond, "container health check should pass after restart")
}
