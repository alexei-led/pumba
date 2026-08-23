package tc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartScopedInstallsOnlyPumbaTopology(t *testing.T) {
	script := Start(&NetemRequest{
		Interface: "eth0",
		Command:   []string{"delay", "100ms"},
		IPs:       []string{"10.0.0.1/32"},
	})[1]

	assert.Contains(t, script, "tc qdisc show dev 'eth0'")
	assert.Contains(t, script, "root handle 504d: prio bands 3 priomap 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1")
	assert.Contains(t, script, "parent 504d:3 handle 5050: netem 'delay' '100ms'")
	assert.Contains(t, script, "flowid 504d:3")
	assert.NotContains(t, script, "qdisc replace")
}

func TestStartRejectsForeignRootBeforeMutation(t *testing.T) {
	log, err := runScript(t, Start(&NetemRequest{
		Interface: "eth0",
		Command:   []string{"loss", "10%"},
	}), "qdisc netem 1: root refcnt 2")

	require.Error(t, err)
	assert.Empty(t, log)
}

func TestStopScopedVerifiesAndRemovesOnlyChildren(t *testing.T) {
	state := strings.Join([]string{
		"qdisc prio 504d: root refcnt 2 bands 3 priomap",
		"qdisc sfq 504e: parent 504d:1 limit 127p",
		"qdisc sfq 504f: parent 504d:2 limit 127p",
		"qdisc netem 5050: parent 504d:3 limit 1000",
	}, "\n")
	log, err := runScript(t, Stop(&NetemRequest{Interface: "eth0", IPs: []string{"10.0.0.1/32"}}), state)

	require.NoError(t, err)
	assert.Contains(t, log, "filter del dev eth0 parent 504d: protocol ip prio 1")
	assert.Contains(t, log, "qdisc del dev eth0 parent 504d:1 handle 504e:")
	assert.Contains(t, log, "qdisc del dev eth0 parent 504d:3 handle 5050:")
	assert.NotContains(t, log, "qdisc del dev eth0 root")
}

func TestStopMissingStateIsSuccess(t *testing.T) {
	log, err := runScript(t, Stop(&NetemRequest{Interface: "eth0"}), "qdisc noqueue 0: root refcnt 2")

	require.NoError(t, err)
	assert.Empty(t, log)
}

func TestStopUnscopedRejectsForeignRoot(t *testing.T) {
	log, err := runScript(t, Stop(&NetemRequest{Interface: "eth0"}), "qdisc prio 1: root refcnt 2")

	require.Error(t, err)
	assert.Empty(t, log)
}

func TestStopUnscopedRemovesVerifiedPumbaRoot(t *testing.T) {
	log, err := runScript(t, Stop(&NetemRequest{Interface: "eth0"}), "qdisc netem 504d: root refcnt 2")

	require.NoError(t, err)
	assert.Equal(t, "qdisc del dev eth0 root handle 504d:\n", log)
}

func runScript(t *testing.T, args []string, state string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands")
	tcPath := filepath.Join(dir, "tc")
	script := `#!/bin/sh
if [ "$1" = "qdisc" ] && [ "$2" = "show" ]; then
  printf '%s\n' "$TC_STATE"
  exit 0
fi
if [ "$1" = "filter" ] && [ "$2" = "show" ]; then
  printf '%s\n' 'filter protocol ip pref 1 u32 chain 0 fh 800: ht divisor 1 flowid 504d:3'
  exit 0
fi
printf '%s\n' "$*" >> "$TC_LOG"
`
	require.NoError(t, os.WriteFile(tcPath, []byte(script), 0o755))

	cmd := []string{"sh"}
	cmd = append(cmd, args...)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TC_STATE", state)
	t.Setenv("TC_LOG", logPath)
	process := exec.Command(cmd[0], cmd[1:]...)
	err := process.Run()
	output, readErr := os.ReadFile(logPath)
	if os.IsNotExist(readErr) {
		return "", err
	}
	require.NoError(t, readErr)
	return string(output), err
}
