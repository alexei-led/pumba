package podman

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSocket_ExplicitReachable(t *testing.T) {
	path := makeSocketFile(t)
	uri, source, err := resolveSocket(path)
	require.NoError(t, err)
	require.Equal(t, "unix://"+path, uri)
	require.Equal(t, "flag:--podman-socket", source)
}

func TestResolveSocket_ExplicitURIPassedThrough(t *testing.T) {
	path := makeSocketFile(t)
	explicit := "unix://" + path
	uri, source, err := resolveSocket(explicit)
	require.NoError(t, err)
	require.Equal(t, explicit, uri)
	require.Equal(t, "flag:--podman-socket", source)
}

func TestResolveSocket_ExplicitUnreachable(t *testing.T) {
	bogus := "/tmp/does-not-exist-podman-sock-xyz-abc"
	_, _, err := resolveSocket(bogus)
	require.Error(t, err)
	require.Contains(t, err.Error(), bogus)
	require.Contains(t, err.Error(), "explicit socket")
}

func TestResolveSocket_FirstReachableWins(t *testing.T) {
	winner := makeSocketFile(t)
	restore := swapCandidates(t, []func() (string, string){
		func() (string, string) { return "/tmp/missing-podman-a-xyz", "test:a" },
		func() (string, string) { return winner, "test:winner" },
		func() (string, string) { return "/tmp/missing-podman-b-xyz", "test:b" },
	})
	defer restore()

	uri, source, err := resolveSocket("")
	require.NoError(t, err)
	require.Equal(t, "unix://"+winner, uri)
	require.Equal(t, "test:winner", source)
}

func TestResolveSocket_EnvWinsOverDefaults(t *testing.T) {
	winner := makeSocketFile(t)
	t.Setenv("PUMBA_TEST_CONTAINER_HOST", winner)

	restore := swapCandidates(t, []func() (string, string){
		envCandidate("PUMBA_TEST_CONTAINER_HOST"),
		func() (string, string) { return "/tmp/missing-podman-default-xyz", "default" },
	})
	defer restore()

	uri, source, err := resolveSocket("")
	require.NoError(t, err)
	require.Equal(t, "unix://"+winner, uri)
	require.Equal(t, "env:PUMBA_TEST_CONTAINER_HOST", source)
}

func TestResolveSocket_AllCandidatesUnreachable(t *testing.T) {
	restore := swapCandidates(t, []func() (string, string){
		func() (string, string) { return "/tmp/missing-podman-a", "test:a" },
		func() (string, string) { return "/tmp/missing-podman-b", "test:b" },
	})
	defer restore()

	_, _, err := resolveSocket("")
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "test:a")
	require.Contains(t, msg, "/tmp/missing-podman-a")
	require.Contains(t, msg, "test:b")
	require.Contains(t, msg, "/tmp/missing-podman-b")
}

func TestResolveSocket_AllCandidatesProduceEmpty(t *testing.T) {
	restore := swapCandidates(t, []func() (string, string){
		func() (string, string) { return "", "" },
		func() (string, string) { return "", "" },
	})
	defer restore()

	_, _, err := resolveSocket("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no candidates produced a value")
}

func TestEnvCandidate_Set(t *testing.T) {
	t.Setenv("PUMBA_TEST_PODMAN_ENV", "/tmp/foo")
	uri, src := envCandidate("PUMBA_TEST_PODMAN_ENV")()
	require.Equal(t, "/tmp/foo", uri)
	require.Equal(t, "env:PUMBA_TEST_PODMAN_ENV", src)
}

func TestEnvCandidate_WhitespaceIsEmpty(t *testing.T) {
	t.Setenv("PUMBA_TEST_PODMAN_ENV_WS", "   ")
	uri, src := envCandidate("PUMBA_TEST_PODMAN_ENV_WS")()
	require.Empty(t, uri)
	require.Empty(t, src)
}

func TestStaticCandidate(t *testing.T) {
	uri, src := staticCandidate("/foo", "label")()
	require.Equal(t, "/foo", uri)
	require.Equal(t, "label", src)
}

func TestXDGRuntimeCandidate_Set(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	uri, src := xdgRuntimeCandidate()
	require.Equal(t, "/run/user/1000/podman/podman.sock", uri)
	require.Contains(t, src, "XDG_RUNTIME_DIR")
}

func TestXDGRuntimeCandidate_Empty(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	uri, src := xdgRuntimeCandidate()
	require.Empty(t, uri)
	require.Empty(t, src)
}

func TestMachineInspectCandidate_PodmanMissing(t *testing.T) {
	restore := stubLookPath(func(string) (string, error) { return "", exec.ErrNotFound })
	defer restore()

	uri, src := machineInspectCandidate()
	require.Empty(t, uri)
	require.Empty(t, src)
}

func TestMachineInspectCandidate_Found(t *testing.T) {
	defer stubLookPath(func(string) (string, error) { return "/usr/bin/podman", nil })()
	defer stubExec(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("/run/user/501/podman/podman.sock\n"), nil
	})()

	uri, src := machineInspectCandidate()
	require.Equal(t, "/run/user/501/podman/podman.sock", uri)
	require.Equal(t, "podman machine inspect", src)
}

func TestMachineInspectCandidate_EmptyOutput(t *testing.T) {
	defer stubLookPath(func(string) (string, error) { return "/usr/bin/podman", nil })()
	defer stubExec(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("   \n"), nil
	})()

	uri, src := machineInspectCandidate()
	require.Empty(t, uri)
	require.Empty(t, src)
}

func TestMachineInspectCandidate_CommandError(t *testing.T) {
	defer stubLookPath(func(string) (string, error) { return "/usr/bin/podman", nil })()
	defer stubExec(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, errors.New("boom")
	})()

	uri, src := machineInspectCandidate()
	require.Empty(t, uri)
	require.Empty(t, src)
}

func TestProbeCandidate_UnixReachable(t *testing.T) {
	path := makeSocketFile(t)
	uri, err := probeCandidate(path)
	require.NoError(t, err)
	require.Equal(t, "unix://"+path, uri)
}

func TestProbeCandidate_UnixMissing(t *testing.T) {
	_, err := probeCandidate("/tmp/no-such-podman-sock-xyz")
	require.Error(t, err)
}

func TestProbeCandidate_TCPReachable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	uri, err := probeCandidate("tcp://" + l.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "tcp://"+l.Addr().String(), uri)
}

func TestProbeCandidate_TCPUnreachable(t *testing.T) {
	_, err := probeCandidate("tcp://127.0.0.1:1")
	require.Error(t, err)
}

func TestProbeCandidate_SSHRejected(t *testing.T) {
	_, err := probeCandidate("ssh://user@host/run/podman/podman.sock")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ssh://")
	require.Contains(t, err.Error(), "not supported")
}

func TestProbeCandidate_UnsupportedScheme(t *testing.T) {
	_, err := probeCandidate("http://127.0.0.1:2375")
	require.Error(t, err)
}

func TestProbeCandidate_RelativePath(t *testing.T) {
	_, err := probeCandidate("relative/path")
	require.Error(t, err)
}

func TestSplitScheme(t *testing.T) {
	cases := []struct {
		in, scheme, rest string
	}{
		{"unix:///run/podman/podman.sock", "unix", "/run/podman/podman.sock"},
		{"tcp://127.0.0.1:2375", "tcp", "127.0.0.1:2375"},
		{"ssh://user@host/sock", "ssh", "ssh://user@host/sock"},
	}
	for _, tc := range cases {
		scheme, rest := splitScheme(tc.in)
		require.Equal(t, tc.scheme, scheme, tc.in)
		require.Equal(t, tc.rest, rest, tc.in)
	}
}

// --- helpers ---

func makeSocketFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sock")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	return path
}

func swapCandidates(t *testing.T, fns []func() (string, string)) func() {
	t.Helper()
	orig := candidateFuncs
	candidateFuncs = fns
	return func() { candidateFuncs = orig }
}

func stubLookPath(fn func(string) (string, error)) func() {
	orig := execLookPath
	execLookPath = fn
	return func() { execLookPath = orig }
}

func stubExec(fn func(context.Context, string, ...string) ([]byte, error)) func() {
	orig := execCommand
	execCommand = fn
	return func() { execCommand = orig }
}
