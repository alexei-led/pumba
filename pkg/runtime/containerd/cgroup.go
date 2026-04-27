package containerd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// cgroupReader reads the cgroup file for a process. Overrideable in tests.
var cgroupReader = func(pid uint32) ([]byte, error) {
	return os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
}

// isSystemdCgroup returns true if the cgroup parent path uses the systemd driver.
// Mirrors containerd CRI's heuristic: systemd cgroup paths end with ".slice".
func isSystemdCgroup(cgroupParent string) bool {
	return strings.HasSuffix(path.Base(cgroupParent), ".slice")
}

// cgroupChildPath constructs a child cgroup path under the given parent.
// For systemd drivers (parent ends with ".slice"), it uses the systemd slice format:
//
//	"<slice>:pumba:<sidecarID>" → runc creates /<slice>/pumba-<sidecarID>.scope
//
// For cgroupfs drivers, it uses a simple path join: "<parent>/<sidecarID>".
func cgroupChildPath(cgroupParent, sidecarID string) string {
	if isSystemdCgroup(cgroupParent) {
		return strings.Join([]string{path.Base(cgroupParent), "pumba", sidecarID}, ":")
	}
	return filepath.Join(cgroupParent, sidecarID)
}

// resolveCgroupPath parses /proc/<pid>/cgroup and returns the target's cgroup path
// and its parent directory. On cgroups v2, expects "0::<path>". On cgroups v1,
// falls back to the "memory" subsystem path.
func resolveCgroupPath(pid uint32) (cgroupPath, cgroupParent string, err error) {
	data, err := cgroupReader(pid)
	if err != nil {
		return "", "", fmt.Errorf("failed to read cgroup for PID %d: %w", pid, err)
	}
	var v1MemoryPath string
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		const cgroupFields = 3
		parts := strings.SplitN(line, ":", cgroupFields)
		if len(parts) != 3 || parts[2] == "/" {
			continue
		}
		// cgroups v2: hierarchy 0, empty subsystem list
		if parts[0] == "0" && parts[1] == "" {
			cgroupPath = parts[2]
			break
		}
		// cgroups v1: look for memory subsystem as representative
		if v1MemoryPath == "" && strings.Contains(parts[1], "memory") {
			v1MemoryPath = parts[2]
		}
	}
	if cgroupPath == "" {
		cgroupPath = v1MemoryPath
	}
	if cgroupPath == "" {
		return "", "", fmt.Errorf("could not parse cgroup path for PID %d (content length: %d bytes)", pid, len(data))
	}
	cgroupParent = "/"
	if lastSlash := strings.LastIndex(cgroupPath, "/"); lastSlash > 0 {
		cgroupParent = cgroupPath[:lastSlash]
	}
	return cgroupPath, cgroupParent, nil
}
