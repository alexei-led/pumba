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

func TestConcurrent_TwoNetemDifferentContainers(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	name1 := uniqueName(t, "conc-netem1")
	name2 := uniqueName(t, "conc-netem2")

	id1 := startContainer(t, name1)
	id2 := startContainer(t, name2)

	// Start netem on both containers simultaneously
	pp1 := runPumbaBackground(t,
		"netem", "--duration", "15s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "100",
		name1,
	)
	pp2 := runPumbaBackground(t,
		"netem", "--duration", "15s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "200",
		name2,
	)

	pid1 := containerPID(t, id1)
	pid2 := containerPID(t, id2)

	// Both should have netem applied
	waitForNetem(t, pid1, "eth0", 10*time.Second)
	waitForNetem(t, pid2, "eth0", 10*time.Second)

	tc1 := nsenterTC(t, pid1, "eth0")
	tc2 := nsenterTC(t, pid2, "eth0")

	assert.Contains(t, strings.ToLower(tc1), "delay")
	assert.Contains(t, strings.ToLower(tc2), "delay")

	// Stop both
	pp1.Stop()
	pp2.Stop()

	// Both should be cleaned
	waitForClean(t, pid1, "eth0", 15*time.Second)
	waitForClean(t, pid2, "eth0", 15*time.Second)
}

func TestConcurrent_NetemAndIPTablesSameContainer(t *testing.T) {
	t.Parallel()
	requireNoDinD(t)

	name := uniqueName(t, "conc-mixed")
	id := startContainer(t, name)

	// Start netem delay
	ppNetem := runPumbaBackground(t,
		"netem", "--duration", "15s",
		"--tc-image", nettoolsImg, "--pull-image=false",
		"delay", "--time", "100",
		name,
	)

	pid := containerPID(t, id)
	waitForNetem(t, pid, "eth0", 10*time.Second)

	// Start iptables on same container
	ppIPT := runPumbaBackground(t,
		"iptables", "--duration", "15s",
		"--iptables-image", nettoolsImg, "--pull-image=false",
		"loss", "--mode", "random", "--probability", "0.5",
		name,
	)

	waitForIPTables(t, pid, 10*time.Second)

	// Both should be active
	tc := nsenterTC(t, pid, "eth0")
	ipt := nsenterIPTables(t, pid)
	assert.Contains(t, strings.ToLower(tc), "netem", "netem should be applied")
	assert.Contains(t, strings.ToUpper(ipt), "DROP", "iptables DROP should be applied")

	// Stop both
	ppNetem.Stop()
	ppIPT.Stop()

	// Both should be cleaned
	waitForClean(t, pid, "eth0", 15*time.Second)
	waitForIPTablesClean(t, pid, 15*time.Second)
}

func TestConcurrent_TwoKillLimitOnSamePool(t *testing.T) {
	t.Parallel()

	prefix := uniqueName(t, "conc-kill")
	// Create 4 containers
	var names []string
	for i := range 4 {
		name := fmt.Sprintf("%s-%d", prefix, i)
		names = append(names, name)
		startContainerWithOpts(t, ContainerOpts{
			Name:  name,
			Image: defaultImage,
			Cmd:   []string{"tail", "-f", "/dev/null"},
		})
	}

	// Build regex pattern matching all 4
	pattern := fmt.Sprintf("re2:%s-\\d+", prefix)

	// Kill with limit=1 twice
	_, _, err1 := runPumba(t, "kill", "--limit", "1", pattern)
	require.NoError(t, err1, "first kill should succeed")

	require.Eventually(t, func() bool {
		for _, name := range names {
			if containerStatus(t, name) == "exited" {
				return true
			}
		}
		return false
	}, 10*time.Second, 500*time.Millisecond, "first container should be killed")

	_, _, err2 := runPumba(t, "kill", "--limit", "1", pattern)
	require.NoError(t, err2, "second kill should succeed")

	require.Eventually(t, func() bool {
		exited := 0
		for _, name := range names {
			if containerStatus(t, name) == "exited" {
				exited++
			}
		}
		return exited >= 2
	}, 10*time.Second, 500*time.Millisecond, "two containers should be killed")

	// Count running vs exited
	running := 0
	exited := 0
	for _, name := range names {
		status := containerStatus(t, name)
		switch status {
		case "running":
			running++
		case "exited":
			exited++
		}
	}

	t.Logf("Pool status: %d running, %d exited", running, exited)
	assert.Equal(t, 2, exited, "exactly 2 containers should be killed")
	assert.Equal(t, 2, running, "exactly 2 containers should remain running")
}
