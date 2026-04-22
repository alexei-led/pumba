package podman

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Canonical cgroup driver names returned by ParseProc1Cgroup.
const (
	driverSystemd  = "systemd"
	driverCgroupfs = "cgroupfs"
)

// Known trailing sub-cgroup segments that should be skipped when locating the
// container's canonical scope. Podman's libpod init process creates a child
// `container` cgroup; systemd running inside a container creates `init.scope`.
const (
	initScopeSegment = "init.scope"
	containerSegment = "container"
)

// cgroupReader reads /proc/<pid>/cgroup from the host filesystem. Overrideable
// in tests. Accepts a Go int (Docker SDK exposes `State.Pid` as int) rather
// than the uint32 used by the containerd runtime.
var cgroupReader = func(pid int) ([]byte, error) {
	return os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
}

// Sentinel errors returned by ParseProc1Cgroup. Exposed for tests and for
// callers that want to distinguish "my read site is wrong" (cgroupns) from
// "this payload is junk".
var (
	errEmptyCgroup         = errors.New("podman cgroup: empty contents")
	errMalformedCgroup     = errors.New("podman cgroup: malformed contents")
	errNoCgroupLine        = errors.New("podman cgroup: no v2 unified or v1 systemd line")
	errPrivateCgroupnsView = errors.New(
		"podman cgroup: private-cgroupns view (caller must read /proc/<pid>/cgroup from the host)",
	)
)

// ParseProc1Cgroup parses /proc/<pid>/cgroup contents (either cgroups v1 or
// v2 format) and returns the container's canonical cgroup location as a
// (driver, fullPath, parent, leaf) tuple.
//
// Selection: prefer the v2 unified line ("0::/..."); fall back to the v1
// systemd hierarchy line ("N:name=systemd:/..."); error otherwise.
//
// Truncation: walk path segments right-to-left and take the prefix up to and
// including the rightmost `.scope` segment, ignoring the conventional
// systemd-in-container `init.scope` and Podman's libpod `container` init
// sub-cgroup. If no `.scope` segment is present, fall back to the rightmost
// `.slice` segment; if neither is present, use the raw path as-is (plain
// cgroupfs hierarchies like `/libpod/<id>`).
//
// Driver: "systemd" when fullPath contains `.slice` or `.scope`; otherwise
// "cgroupfs".
//
// Returns errPrivateCgroupnsView when the v2 path is "/" or "/container" —
// those are artifacts of reading from inside a container running under the
// default cgroupns=private mode. The caller has read from the wrong side of
// the namespace boundary and must retry against the host PID's /proc entry.
//
// Typical inputs observed on a rootful `podman machine` (Podman 4.9+, cgroups
// v2, systemd driver):
//
//	0::/machine.slice/libpod-<64-hex-id>.scope
//	0::/machine.slice/libpod-<64-hex-id>.scope/container     (Podman init sub-cgroup)
//
// which both truncate to `/machine.slice/libpod-<id>.scope`.
func ParseProc1Cgroup(contents string) (driver, fullPath, parent, leaf string, err error) {
	trimmed := strings.TrimSpace(contents)
	if trimmed == "" {
		return "", "", "", "", errEmptyCgroup
	}

	rawPath, selectErr := selectCgroupPath(trimmed)
	if selectErr != nil {
		return "", "", "", "", selectErr
	}

	rawPath = normaliseCgroupPath(rawPath)
	if rawPath == "/" || rawPath == "/"+containerSegment {
		return "", "", "", "", fmt.Errorf("%w (saw %q)", errPrivateCgroupnsView, rawPath)
	}

	fullPath = truncateToContainerScope(rawPath)
	driver = driverForPath(fullPath)
	parent, leaf = splitParentLeaf(fullPath)
	return driver, fullPath, parent, leaf, nil
}

// selectCgroupPath returns the path portion of the preferred line from
// /proc/<pid>/cgroup contents. Picks the v2 unified line if present, else the
// v1 `name=systemd` hierarchy, else errors.
func selectCgroupPath(contents string) (string, error) {
	const expectedFields = 3
	var (
		v2Path, v1SystemdPath string
		sawStructuredLine     bool
	)
	for line := range strings.SplitSeq(contents, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", expectedFields)
		if len(parts) != expectedFields {
			continue
		}
		sawStructuredLine = true
		if parts[0] == "0" && parts[1] == "" {
			if v2Path == "" {
				v2Path = parts[2]
			}
			continue
		}
		if v1SystemdPath == "" && hasSubsystem(parts[1], "name=systemd") {
			v1SystemdPath = parts[2]
		}
	}
	if !sawStructuredLine {
		return "", errMalformedCgroup
	}
	switch {
	case v2Path != "":
		return v2Path, nil
	case v1SystemdPath != "":
		return v1SystemdPath, nil
	}
	return "", errNoCgroupLine
}

// hasSubsystem reports whether subsystems (the comma-separated controller
// list from a cgroup line's second field) contains name as a full entry.
func hasSubsystem(subsystems, name string) bool {
	for s := range strings.SplitSeq(subsystems, ",") {
		if s == name {
			return true
		}
	}
	return false
}

// normaliseCgroupPath strips trailing slashes from p while preserving a lone
// "/" for downstream private-cgroupns detection.
func normaliseCgroupPath(p string) string {
	if p == "/" {
		return p
	}
	return strings.TrimRight(p, "/")
}

// truncateToContainerScope returns the prefix of p up to and including the
// rightmost container-scope segment. See ParseProc1Cgroup for the truncation
// rules in prose.
func truncateToContainerScope(p string) string {
	segments := strings.Split(p, "/")
	if idx := lastIndexEndingIn(segments, ".scope", true); idx >= 0 {
		return strings.Join(segments[:idx+1], "/")
	}
	if idx := lastIndexEndingIn(segments, ".slice", false); idx >= 0 {
		return strings.Join(segments[:idx+1], "/")
	}
	return p
}

// lastIndexEndingIn returns the index of the rightmost non-empty segment that
// ends with suffix. When skipInitScope is true, segments exactly equal to
// "init.scope" are treated as sub-cgroup noise and ignored during the scan.
func lastIndexEndingIn(segments []string, suffix string, skipInitScope bool) int {
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		if seg == "" {
			continue
		}
		if skipInitScope && seg == initScopeSegment {
			continue
		}
		if strings.HasSuffix(seg, suffix) {
			return i
		}
	}
	return -1
}

// driverForPath returns "systemd" when p contains `.slice` or `.scope`; else
// "cgroupfs".
func driverForPath(p string) string {
	if strings.Contains(p, ".slice") || strings.Contains(p, ".scope") {
		return driverSystemd
	}
	return driverCgroupfs
}

// splitParentLeaf returns (parent, leaf) for an absolute cgroup path. For
// single-segment paths (e.g. "/libpod-abc.scope"), parent collapses to "/"
// to match Docker SDK expectations around cgroup parent strings.
func splitParentLeaf(p string) (string, string) {
	i := strings.LastIndex(p, "/")
	switch {
	case i < 0:
		return "/", p
	case i == 0:
		return "/", p[1:]
	}
	return p[:i], p[i+1:]
}
