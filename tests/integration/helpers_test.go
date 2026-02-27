//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/require"
)

// uniqueName returns a test-unique container name to avoid collisions in parallel tests.
func uniqueName(t *testing.T, base string) string {
	t.Helper()
	return fmt.Sprintf("inttest-%s-%s-%d", base, t.Name(), rand.IntN(100000)) //nolint:mnd
}

// ContainerOpts configures a test container.
type ContainerOpts struct {
	Name       string
	Image      string
	Cmd        []string
	Labels     map[string]string
	Restart    string // restart policy name
	Memory     int64  // memory limit in bytes
	Networks   []string
	Volumes    map[string]struct{}
	Privileged bool
}

// startContainer creates and starts a simple alpine container.
func startContainer(t *testing.T, name string) string {
	t.Helper()
	return startContainerWithOpts(t, ContainerOpts{
		Name:  name,
		Image: defaultImage,
		Cmd:   []string{"sleep", "infinity"},
	})
}

// startContainerWithOpts creates and starts a container with full options.
func startContainerWithOpts(t *testing.T, opts ContainerOpts) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if opts.Image == "" {
		opts.Image = defaultImage
	}
	if len(opts.Cmd) == 0 {
		opts.Cmd = []string{"sleep", "infinity"}
	}

	config := &container.Config{
		Image:  opts.Image,
		Cmd:    opts.Cmd,
		Labels: opts.Labels,
	}
	if opts.Volumes != nil {
		config.Volumes = opts.Volumes
	}

	hostConfig := &container.HostConfig{
		Privileged: opts.Privileged,
	}
	if opts.Memory > 0 {
		hostConfig.Resources.Memory = opts.Memory
	}
	if opts.Restart != "" {
		hostConfig.RestartPolicy.Name = container.RestartPolicyMode(opts.Restart)
	}

	netConfig := &network.NetworkingConfig{}

	resp, err := dockerCli.ContainerCreate(ctx, config, hostConfig, netConfig, nil, opts.Name)
	require.NoError(t, err, "create container %s", opts.Name)

	err = dockerCli.ContainerStart(ctx, resp.ID, container.StartOptions{})
	require.NoError(t, err, "start container %s", opts.Name)

	t.Cleanup(func() {
		removeContainer(t, resp.ID)
	})

	// Wait for running state
	require.Eventually(t, func() bool {
		return containerStatus(t, resp.ID) == "running"
	}, 10*time.Second, 200*time.Millisecond, "container %s not running", opts.Name)

	return resp.ID
}

// removeContainer forcefully removes a container.
func removeContainer(t *testing.T, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = dockerCli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true, RemoveVolumes: true})
}

// containerStatus returns the container status string (running, exited, paused, etc).
func containerStatus(t *testing.T, id string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, err := dockerCli.ContainerInspect(ctx, id)
	if err != nil {
		return "not_found"
	}
	return info.State.Status
}

// containerPID returns the PID of a running container.
func containerPID(t *testing.T, id string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, err := dockerCli.ContainerInspect(ctx, id)
	require.NoError(t, err, "inspect container %s", id)
	require.NotZero(t, info.State.Pid, "container %s has no PID (not running?)", id)
	return info.State.Pid
}

// containerIP returns the IP address of a container on the default bridge network.
func containerIP(t *testing.T, id string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, err := dockerCli.ContainerInspect(ctx, id)
	require.NoError(t, err)
	ip := info.NetworkSettings.IPAddress
	require.NotEmpty(t, ip, "container %s has no IP", id)
	return ip
}

// runPumba runs pumba synchronously and returns stdout, stderr.
func runPumba(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return runPumbaCtx(ctx, t, args...)
}

// runPumbaCtx runs pumba with an explicit context.
func runPumbaCtx(ctx context.Context, t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, pumba, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// PumbaProcess represents a background pumba process.
type PumbaProcess struct {
	Cmd    *exec.Cmd
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
	cancel context.CancelFunc
}

// runPumbaBackground starts pumba in the background with context cancellation.
func runPumbaBackground(t *testing.T, args ...string) *PumbaProcess {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var outBuf, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, pumba, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	require.NoError(t, cmd.Start(), "start pumba background: %v", args)

	pp := &PumbaProcess{
		Cmd:    cmd,
		Stdout: &outBuf,
		Stderr: &errBuf,
		cancel: cancel,
	}

	t.Cleanup(func() {
		pp.Stop()
	})

	return pp
}

// Stop cancels the context (sending SIGKILL) and waits for exit.
func (p *PumbaProcess) Stop() {
	if p.Cmd.Process == nil {
		return
	}
	p.cancel()
	_ = p.Cmd.Wait()
}

// Signal sends a specific signal to the pumba process.
func (p *PumbaProcess) Signal(sig string) error {
	if p.Cmd.Process == nil {
		return fmt.Errorf("process not started")
	}
	//nolint:gosec // signal name is controlled by tests
	return exec.Command("kill", "-"+sig, strconv.Itoa(p.Cmd.Process.Pid)).Run()
}

// Wait waits for the pumba process to exit and returns the error.
func (p *PumbaProcess) Wait() error {
	return p.Cmd.Wait()
}

// nsenterTC runs `tc qdisc show dev <iface>` inside the container's network namespace.
func nsenterTC(t *testing.T, pid int, iface string) string {
	t.Helper()
	out, err := exec.Command("nsenter", "-t", strconv.Itoa(pid), "-n",
		"tc", "qdisc", "show", "dev", iface).CombinedOutput()
	require.NoError(t, err, "nsenter tc qdisc show: %s", string(out))
	return string(out)
}

// nsenterIPTables runs `iptables -L INPUT -n -v` inside the container's network namespace.
func nsenterIPTables(t *testing.T, pid int) string {
	t.Helper()
	out, err := exec.Command("nsenter", "-t", strconv.Itoa(pid), "-n",
		"iptables", "-L", "INPUT", "-n", "-v").CombinedOutput()
	require.NoError(t, err, "nsenter iptables: %s", string(out))
	return string(out)
}

// waitForNetem polls until netem rules appear on the interface.
func waitForNetem(t *testing.T, pid int, iface string, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		out, err := exec.Command("nsenter", "-t", strconv.Itoa(pid), "-n",
			"tc", "qdisc", "show", "dev", iface).CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(strings.ToLower(string(out)), "netem")
	}, timeout, 200*time.Millisecond, "netem rules not applied within %v", timeout)
}

// waitForClean polls until netem rules are removed from the interface.
func waitForClean(t *testing.T, pid int, iface string, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		out, err := exec.Command("nsenter", "-t", strconv.Itoa(pid), "-n",
			"tc", "qdisc", "show", "dev", iface).CombinedOutput()
		if err != nil {
			return true // container gone = clean
		}
		return !strings.Contains(strings.ToLower(string(out)), "netem")
	}, timeout, 200*time.Millisecond, "netem rules still present after %v", timeout)
}

// waitForIPTables polls until iptables DROP rules appear.
func waitForIPTables(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		out, err := exec.Command("nsenter", "-t", strconv.Itoa(pid), "-n",
			"iptables", "-L", "INPUT", "-n", "-v").CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(strings.ToUpper(string(out)), "DROP")
	}, timeout, 200*time.Millisecond, "iptables DROP rules not applied within %v", timeout)
}

// waitForIPTablesClean polls until iptables DROP rules are removed.
func waitForIPTablesClean(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		out, err := exec.Command("nsenter", "-t", strconv.Itoa(pid), "-n",
			"iptables", "-L", "INPUT", "-n", "-v").CombinedOutput()
		if err != nil {
			return true // container gone = clean
		}
		return !strings.Contains(strings.ToUpper(string(out)), "DROP")
	}, timeout, 200*time.Millisecond, "iptables DROP rules still present after %v", timeout)
}

// countSidecars counts containers with the pumba.skip label.
func countSidecars(t *testing.T) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	containers, err := dockerCli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", "com.gaiaadm.pumba.skip=true")),
	})
	require.NoError(t, err, "list sidecar containers")
	return len(containers)
}

// PingResult holds parsed ping output.
type PingResult struct {
	Transmitted int
	Received    int
	AvgRTT      time.Duration
	Raw         string
}

// pingBetween runs ping from one container to an IP.
func pingBetween(t *testing.T, fromID, toIP string, count int) PingResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(count+10)*time.Second) //nolint:mnd
	defer cancel()

	execConfig := container.ExecOptions{
		Cmd:          []string{"ping", "-c", strconv.Itoa(count), "-W", "2", toIP},
		AttachStdout: true,
		AttachStderr: true,
	}
	execResp, err := dockerCli.ContainerExecCreate(ctx, fromID, execConfig)
	require.NoError(t, err)

	attachResp, err := dockerCli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	require.NoError(t, err)
	defer attachResp.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(attachResp.Reader)
	raw := buf.String()

	result := PingResult{Raw: raw}
	parsePingOutput(&result, raw)
	return result
}

// parsePingOutput extracts stats from ping output.
func parsePingOutput(r *PingResult, raw string) {
	// Match: "X packets transmitted, Y received" or "Y packets received"
	re := regexp.MustCompile(`(\d+) packets transmitted, (\d+)`)
	if m := re.FindStringSubmatch(raw); len(m) == 3 { //nolint:mnd
		r.Transmitted, _ = strconv.Atoi(m[1])
		r.Received, _ = strconv.Atoi(m[2])
	}

	// Match avg RTT: "min/avg/max/mdev = X/Y/Z/W ms"
	reRTT := regexp.MustCompile(`= [\d.]+/([\d.]+)/[\d.]+/[\d.]+ ms`)
	if m := reRTT.FindStringSubmatch(raw); len(m) == 2 { //nolint:mnd
		if avg, err := strconv.ParseFloat(m[1], 64); err == nil {
			r.AvgRTT = time.Duration(avg * float64(time.Millisecond))
		}
	}
}

// execInContainer runs a command in a container and returns the output.
func execInContainer(t *testing.T, containerID string, cmd []string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	execResp, err := dockerCli.ContainerExecCreate(ctx, containerID, execConfig)
	require.NoError(t, err)

	attachResp, err := dockerCli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	require.NoError(t, err)
	defer attachResp.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(attachResp.Reader)
	return buf.String()
}

// requireDinD skips the test if running inside Docker-in-Docker.
func requireNoDinD(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("/.dockerenv"); err == nil {
		t.Skip("sidecar tests not supported in Docker-in-Docker")
	}
	// Also check for .dockerenv file
	if _, err := exec.Command("test", "-f", "/.dockerenv").Output(); err == nil {
		t.Skip("sidecar tests not supported in Docker-in-Docker")
	}
}

// requireContainerd skips the test if containerd socket is not available.
func requireContainerd(t *testing.T) {
	t.Helper()
	if _, err := exec.Command("test", "-S", "/run/containerd/containerd.sock").Output(); err != nil {
		t.Skip("containerd socket not available")
	}
}
