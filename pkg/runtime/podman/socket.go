// Package podman implements the container.Client interface for the Podman
// runtime via Podman's Docker-compatible API socket.
package podman

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultSystemSocket   = "/run/podman/podman.sock"
	defaultSocketScheme   = "unix"
	tcpScheme             = "tcp"
	sshScheme             = "ssh"
	probeTimeout          = time.Second
	machineInspectTimeout = 3 * time.Second
)

// sshUnsupportedHint explains why ssh:// podman endpoints are rejected. The
// shared Docker HTTP client has no SSH connhelper wired in, so accepting an
// ssh:// URI here would only defer the failure to /info bootstrap with a less
// useful diagnostic. Run pumba on the same host as the podman socket instead,
// or expose a tcp:// listener.
const sshUnsupportedHint = "ssh:// podman endpoints are not supported; run pumba on the podman host or expose tcp:// instead"

// candidateFuncs is the ordered list of podman socket candidates tried when no
// explicit socket is set. Each function returns a (rawURI, sourceLabel) pair
// or ("", "") to skip. Tests swap this variable to inject custom candidates.
var candidateFuncs = []func() (string, string){
	envCandidate("CONTAINER_HOST"),
	envCandidate("PODMAN_SOCK"),
	machineInspectCandidate,
	staticCandidate(defaultSystemSocket, "default:"+defaultSystemSocket),
	xdgRuntimeCandidate,
}

// execLookPath and execCommand are indirections so tests can stub the
// `podman machine inspect` invocation without running real subprocesses.
var (
	execLookPath = exec.LookPath
	execCommand  = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).Output()
	}
)

// resolveSocket returns the first reachable podman socket URI along with a
// short source label suitable for diagnostic output. An explicit value wins
// outright — unreachable explicit URIs do not fall back to auto-discovery.
func resolveSocket(explicit string) (string, string, error) {
	if explicit != "" {
		uri, err := probeCandidate(explicit)
		if err != nil {
			return "", "", fmt.Errorf("podman runtime: explicit socket %q unreachable: %w", explicit, err)
		}
		return uri, "flag:--podman-socket", nil
	}
	var tried []string
	for _, fn := range candidateFuncs {
		raw, src := fn()
		if raw == "" {
			continue
		}
		uri, err := probeCandidate(raw)
		if err == nil {
			return uri, src, nil
		}
		tried = append(tried, fmt.Sprintf("%s=%s (%v)", src, raw, err))
	}
	if len(tried) == 0 {
		return "", "", errors.New("podman runtime: no reachable socket found (no candidates produced a value)")
	}
	return "", "", fmt.Errorf("podman runtime: no reachable socket found (tried: %s)", strings.Join(tried, "; "))
}

// envCandidate returns a candidate backed by the named environment variable.
func envCandidate(name string) func() (string, string) {
	return func() (string, string) {
		v := strings.TrimSpace(os.Getenv(name))
		if v == "" {
			return "", ""
		}
		return v, "env:" + name
	}
}

// staticCandidate returns a candidate that always produces the given value.
func staticCandidate(value, source string) func() (string, string) {
	return func() (string, string) { return value, source }
}

// xdgRuntimeCandidate resolves $XDG_RUNTIME_DIR/podman/podman.sock. Returns
// empty when the env var isn't set.
func xdgRuntimeCandidate() (string, string) {
	dir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR"))
	if dir == "" {
		return "", ""
	}
	return filepath.Join(dir, "podman", "podman.sock"), "env:XDG_RUNTIME_DIR/podman/podman.sock"
}

// machineInspectCandidate asks `podman machine inspect` for the active
// socket. Returns ("", "") when `podman` isn't on $PATH or the command fails
// — callers fall through to the next candidate.
//
// `podman machine inspect` without an explicit machine name renders the
// --format template once per configured machine, so the output can be
// multi-line when several machines exist. Return the first non-empty path
// rather than the trimmed blob (which would leave an embedded newline and
// silently fall through to the next candidate). Users with multiple machines
// who want a specific one should set --podman-socket explicitly.
func machineInspectCandidate() (string, string) {
	if _, err := execLookPath("podman"); err != nil {
		return "", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), machineInspectTimeout)
	defer cancel()
	out, err := execCommand(ctx, "podman", "machine", "inspect", "--format", "{{.ConnectionInfo.PodmanSocket.Path}}")
	if err != nil {
		return "", ""
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		path := strings.TrimSpace(line)
		if path != "" {
			return path, "podman machine inspect"
		}
	}
	return "", ""
}

// probeCandidate normalises a raw socket value to a Docker-SDK-compatible URI
// and verifies reachability.
func probeCandidate(raw string) (string, error) {
	uri, err := normalise(raw)
	if err != nil {
		return "", err
	}
	scheme, rest := splitScheme(uri)
	switch scheme {
	case defaultSocketScheme:
		if _, statErr := os.Stat(rest); statErr != nil {
			return "", statErr
		}
	case tcpScheme:
		if rest == "" {
			return "", fmt.Errorf("malformed tcp URI %q: missing host:port", uri)
		}
		d := net.Dialer{Timeout: probeTimeout}
		conn, dialErr := d.Dial(tcpScheme, rest)
		if dialErr != nil {
			return "", dialErr
		}
		_ = conn.Close()
	case sshScheme:
		return "", errors.New(sshUnsupportedHint)
	default:
		return "", fmt.Errorf("unsupported socket scheme %q", scheme)
	}
	return uri, nil
}

// normalise converts a bare filesystem path into a `unix://` URI; values that
// already declare a scheme pass through unchanged.
func normalise(raw string) (string, error) {
	if strings.Contains(raw, "://") {
		return raw, nil
	}
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("podman socket %q must be absolute or a URI", raw)
	}
	return defaultSocketScheme + "://" + raw, nil
}

// splitScheme returns (scheme, remainder) for a URI. For unix URIs the
// remainder is the filesystem path; for tcp the remainder is `host:port`.
func splitScheme(uri string) (string, string) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", uri
	}
	switch u.Scheme {
	case defaultSocketScheme:
		return defaultSocketScheme, u.Path
	case tcpScheme:
		return tcpScheme, u.Host
	}
	return u.Scheme, uri
}
