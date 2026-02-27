//go:build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkVerify_DelayIncreasesRTT(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	// Start two netshoot containers on default bridge
	srvName := uniqueName(t, "netverify-srv")
	clientName := uniqueName(t, "netverify-cli")

	srvID := startContainerWithOpts(t, ContainerOpts{
		Name:  srvName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})
	clientID := startContainerWithOpts(t, ContainerOpts{
		Name:  clientName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})

	srvIP := containerIP(t, srvID)

	// Measure baseline RTT
	baseline := pingBetween(t, clientID, srvIP, 3)
	require.Greater(t, baseline.Received, 0, "baseline ping should succeed")
	t.Logf("Baseline RTT: %v", baseline.AvgRTT)

	// Apply 200ms delay to server container
	pp := runPumbaBackground(t,
		"netem", "--duration", "20s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "200",
		srvName,
	)

	pid := containerPID(t, srvID)
	waitForNetem(t, pid, "eth0", 10*time.Second)

	// Measure RTT with delay
	delayed := pingBetween(t, clientID, srvIP, 3)
	require.Greater(t, delayed.Received, 0, "delayed ping should succeed")
	t.Logf("Delayed RTT: %v", delayed.AvgRTT)

	// Verify RTT increased by at least 150ms (allowing for baseline jitter)
	delta := delayed.AvgRTT - baseline.AvgRTT
	assert.Greater(t, delta, 150*time.Millisecond,
		"RTT should increase by ~200ms, got delta=%v (baseline=%v, delayed=%v)",
		delta, baseline.AvgRTT, delayed.AvgRTT)

	pp.Stop()
}

func TestNetworkVerify_Loss100PercentBlocksPing(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	srvName := uniqueName(t, "netverify-loss-srv")
	clientName := uniqueName(t, "netverify-loss-cli")

	srvID := startContainerWithOpts(t, ContainerOpts{
		Name:  srvName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})
	clientID := startContainerWithOpts(t, ContainerOpts{
		Name:  clientName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})

	srvIP := containerIP(t, srvID)

	// Apply 100% packet loss
	pp := runPumbaBackground(t,
		"netem", "--duration", "20s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"loss", "--percent", "100",
		srvName,
	)

	pid := containerPID(t, srvID)
	waitForNetem(t, pid, "eth0", 10*time.Second)

	// Ping should fail completely
	result := pingBetween(t, clientID, srvIP, 5)
	assert.Equal(t, 0, result.Received,
		"with 100%% loss, no packets should be received; got %d/%d",
		result.Received, result.Transmitted)

	pp.Stop()
}

func TestNetworkVerify_IPTablesBlocksTraffic(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	srvName := uniqueName(t, "netverify-ipt-srv")
	clientName := uniqueName(t, "netverify-ipt-cli")

	srvID := startContainerWithOpts(t, ContainerOpts{
		Name:  srvName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})
	clientID := startContainerWithOpts(t, ContainerOpts{
		Name:  clientName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})

	srvIP := containerIP(t, srvID)

	// Verify baseline connectivity
	baseline := pingBetween(t, clientID, srvIP, 2)
	require.Greater(t, baseline.Received, 0, "baseline ping should succeed")

	// Apply iptables loss probability 1.0
	pp := runPumbaBackground(t,
		"iptables", "--duration", "20s",
		"--iptables-image", nettoolsImg, "--pull-image=false",
		"loss", "--mode", "random", "--probability", "1.0",
		srvName,
	)

	pid := containerPID(t, srvID)
	waitForIPTables(t, pid, 10*time.Second)

	// Ping should fail
	result := pingBetween(t, clientID, srvIP, 5)
	assert.Equal(t, 0, result.Received,
		"with iptables DROP, no packets should be received")

	pp.Stop()
}

func TestNetworkVerify_DelayRemovedRTTReturns(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	srvName := uniqueName(t, "netverify-recover-srv")
	clientName := uniqueName(t, "netverify-recover-cli")

	srvID := startContainerWithOpts(t, ContainerOpts{
		Name:  srvName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})
	clientID := startContainerWithOpts(t, ContainerOpts{
		Name:  clientName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})

	srvIP := containerIP(t, srvID)

	// Baseline
	baseline := pingBetween(t, clientID, srvIP, 3)
	require.Greater(t, baseline.Received, 0)
	t.Logf("Baseline RTT: %v", baseline.AvgRTT)

	// Apply short delay
	_, stderr, err := runPumba(t,
		"netem", "--duration", "5s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "200",
		srvName,
	)
	require.NoError(t, err, "pumba netem failed: %s", stderr)

	// After pumba exits, verify RTT returned to baseline
	pid := containerPID(t, srvID)
	waitForClean(t, pid, "eth0", 10*time.Second)

	recovered := pingBetween(t, clientID, srvIP, 3)
	require.Greater(t, recovered.Received, 0)
	t.Logf("Recovered RTT: %v", recovered.AvgRTT)

	// Recovered RTT should be close to baseline (within 50ms)
	assert.Less(t, recovered.AvgRTT, baseline.AvgRTT+50*time.Millisecond,
		"after netem removal, RTT should return to baseline")
}

func TestNetworkVerify_IPTablesPortFilter(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	srvName := uniqueName(t, "netverify-port-srv")
	clientName := uniqueName(t, "netverify-port-cli")

	srvID := startContainerWithOpts(t, ContainerOpts{
		Name:  srvName,
		Image: netshootImage,
		Cmd:   []string{"sh", "-c", "nc -lk -p 80 -e echo 'HTTP OK' & nc -lk -p 8080 -e echo 'ALT OK' & sleep infinity"},
	})
	clientID := startContainerWithOpts(t, ContainerOpts{
		Name:  clientName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})

	srvIP := containerIP(t, srvID)
	require.Eventually(t, func() bool {
		out := execInContainer(t, clientID, []string{"sh", "-c",
			fmt.Sprintf("nc -z -w1 %s 80 2>/dev/null && echo ok", srvIP)})
		return strings.Contains(out, "ok")
	}, 5*time.Second, 200*time.Millisecond, "nc listeners not ready")

	// Apply iptables loss only on port 80
	pp := runPumbaBackground(t,
		"iptables", "--duration", "20s",
		"--iptables-image", nettoolsImg, "--pull-image=false",
		"--protocol", "tcp", "--dst-port", "80",
		"loss", "--mode", "random", "--probability", "1.0",
		srvName,
	)

	pid := containerPID(t, srvID)
	waitForIPTables(t, pid, 10*time.Second)

	// Port 80 should be blocked — nc connection should fail/timeout
	out80 := execInContainer(t, clientID, []string{"sh", "-c",
		fmt.Sprintf("timeout 3 nc -w 2 %s 80 || echo 'CONN_FAILED'", srvIP)})
	assert.Contains(t, out80, "CONN_FAILED",
		"port 80 should be blocked by iptables")

	// Port 8080 should still work
	out8080 := execInContainer(t, clientID, []string{"sh", "-c",
		fmt.Sprintf("echo test | nc -w 2 %s 8080", srvIP)})
	t.Logf("Port 8080 response: %s", out8080)
	assert.NotContains(t, out8080, "CONN_FAILED",
		"port 8080 should not be blocked by iptables targeting port 80 only")

	pp.Stop()
}

func TestNetworkVerify_NetemTargetIP(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	// Three containers: source, target (delayed), bystander (not delayed)
	srcName := uniqueName(t, "netverify-tgt-src")
	tgtName := uniqueName(t, "netverify-tgt-tgt")
	byName := uniqueName(t, "netverify-tgt-by")

	srcID := startContainerWithOpts(t, ContainerOpts{
		Name:  srcName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})
	tgtID := startContainerWithOpts(t, ContainerOpts{
		Name:  tgtName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})
	_ = startContainerWithOpts(t, ContainerOpts{
		Name:  byName,
		Image: netshootImage,
		Cmd:   []string{"sleep", "infinity"},
	})

	tgtIP := containerIP(t, tgtID)

	// Apply delay with --target filter to source container
	pp := runPumbaBackground(t,
		"netem", "--duration", "20s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"--target", tgtIP,
		"delay", "--time", "200",
		srcName,
	)

	pid := containerPID(t, srcID)
	waitForNetem(t, pid, "eth0", 10*time.Second)

	// Ping to target should be delayed
	delayed := pingBetween(t, srcID, tgtIP, 3)
	require.Greater(t, delayed.Received, 0)
	t.Logf("Delayed ping to target: %v", delayed.AvgRTT)

	// RTT to target should show significant delay
	assert.Greater(t, delayed.AvgRTT, 150*time.Millisecond,
		"ping to target IP should be delayed by ~200ms")

	pp.Stop()

	// Verify cleanup
	waitForClean(t, pid, "eth0", 10*time.Second)

	// After cleanup, ping should be fast again
	clean := pingBetween(t, srcID, tgtIP, 3)
	require.Greater(t, clean.Received, 0, "ping should succeed after netem cleanup")
	assert.Less(t, clean.AvgRTT, 50*time.Millisecond,
		"after netem cleanup, RTT should return to normal")
}
