package tc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartupRefusalExitStatusBeforeMutation(t *testing.T) {
	for _, mode := range []string{"unscoped", "OR", "AND"} {
		for _, state := range []string{"qdisc netem 1: root", "qdisc netem 504d: root", "qdisc prio 504d: root"} {
			t.Run(mode+"/"+state, func(t *testing.T) {
				req := &NetemRequest{Interface: "eth0", Command: []string{"delay", "100ms"}, MatchAll: mode == "AND"}
				if mode != "unscoped" {
					req.DPorts = []string{"443"}
				}
				log, err := runScript(t, startArgs(t, req), state)
				var exited *exec.ExitError
				require.ErrorAs(t, err, &exited)
				require.Equal(t, NetemRefusedExitCode, exited.ExitCode())
				require.Empty(t, log)
			})
		}
	}
}

func TestStartupSubcommandCannotImpersonateRefusal(t *testing.T) {
	for _, mode := range []string{"unscoped", "OR", "AND"} {
		for _, step := range []string{"show", "root", "child", "filter"} {
			if mode == "unscoped" && (step == "child" || step == "filter") {
				continue
			}
			t.Run(mode+"/"+step, func(t *testing.T) {
				dir := t.TempDir()
				originalPath := os.Getenv("PATH")
				script := `#!/bin/sh
case "$TC_FAIL:$*" in
 show:qdisc\ show*|root:qdisc\ add*root*|child:qdisc\ add*parent*|filter:filter\ add*) exit "$REFUSAL_CODE" ;;
esac
if [ "$1 $2" = 'qdisc show' ]; then printf 'qdisc noqueue 0: root\n'; fi
`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "tc"), []byte(script), 0o755))
				t.Setenv("PATH", dir+string(os.PathListSeparator)+originalPath)
				t.Setenv("TC_FAIL", step)
				t.Setenv("REFUSAL_CODE", strconv.Itoa(NetemRefusedExitCode))
				req := &NetemRequest{Interface: "eth0", Command: []string{"delay", "100ms"}, MatchAll: mode == "AND"}
				if mode != "unscoped" {
					req.DPorts = []string{"443"}
				}
				err := exec.Command("sh", startArgs(t, req)...).Run()
				var exited *exec.ExitError
				require.ErrorAs(t, err, &exited)
				require.Equal(t, 1, exited.ExitCode(), "a tc failure is not proven pre-mutation refusal")
			})
		}
	}
}

func TestStartupMissingVerifierRefusesBeforeMutation(t *testing.T) {
	shell, err := exec.LookPath("sh")
	require.NoError(t, err)
	t.Setenv("PATH", t.TempDir())
	args := startArgs(t, &NetemRequest{Interface: "eth0", DPorts: []string{"443"}, MatchAll: true})
	err = exec.Command(shell, args...).Run()
	var exited *exec.ExitError
	require.ErrorAs(t, err, &exited)
	require.Equal(t, NetemRefusedExitCode, exited.ExitCode())
}
