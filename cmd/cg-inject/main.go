// cg-inject is a minimal binary that moves itself into a target container's cgroup
// and then exec's stress-ng (or any other command). This enables same-cgroup stress
// testing where stress-ng shares resource accounting with the target container.
//
// Usage:
//
//	cg-inject --target-id <containerID> [--cgroup-driver auto|cgroupfs|systemd] -- /stress-ng <args...>
//	cg-inject --cgroup-path <path> -- /stress-ng <args...>
package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"syscall"
)

// cgroupVersion represents the cgroup hierarchy version.
type cgroupVersion int

const (
	cgroupV1 cgroupVersion = 1
	cgroupV2 cgroupVersion = 2
)

// cgroupDriver represents the Docker cgroup driver.
type cgroupDriver string

const (
	driverAuto     cgroupDriver = "auto"
	driverCgroupfs cgroupDriver = "cgroupfs"
	driverSystemd  cgroupDriver = "systemd"
)

// config holds the parsed command-line configuration.
type config struct {
	targetID    string
	cgroupPath  string // explicit cgroup base path (e.g., /kubepods/burstable/pod-uid/container-id)
	driver      cgroupDriver
	commandArgs []string
}

// for testing: override cgroup filesystem root and syscall.Exec
var (
	cgroupRoot  = "/sys/fs/cgroup"
	execCommand = syscall.Exec
)

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cg-inject: %v\n", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "cg-inject: %v\n", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (config, error) {
	cfg := config{driver: driverAuto}

	// find the -- separator
	dashIdx := -1
	for i, a := range args {
		if a == "--" {
			dashIdx = i
			break
		}
	}
	if dashIdx < 0 {
		return cfg, errors.New("missing '--' separator before command")
	}

	cfg.commandArgs = args[dashIdx+1:]
	if len(cfg.commandArgs) == 0 {
		return cfg, errors.New("no command specified after '--'")
	}

	if err := parseFlags(&cfg, args[:dashIdx]); err != nil {
		return cfg, err
	}
	if err := validateConfig(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func parseFlags(cfg *config, flagArgs []string) error {
	for i := 0; i < len(flagArgs); i++ {
		switch flagArgs[i] {
		case "--target-id":
			i++
			if i >= len(flagArgs) {
				return errors.New("--target-id requires a value")
			}
			cfg.targetID = flagArgs[i]
		case "--cgroup-driver":
			i++
			if i >= len(flagArgs) {
				return errors.New("--cgroup-driver requires a value")
			}
			d := cgroupDriver(flagArgs[i])
			if d != driverAuto && d != driverCgroupfs && d != driverSystemd {
				return fmt.Errorf("unknown cgroup driver %q (expected auto, cgroupfs, or systemd)", flagArgs[i])
			}
			cfg.driver = d
		case "--cgroup-path":
			i++
			if i >= len(flagArgs) {
				return errors.New("--cgroup-path requires a value")
			}
			cfg.cgroupPath = flagArgs[i]
		default:
			return fmt.Errorf("unknown flag %q", flagArgs[i])
		}
	}
	return nil
}

func validateConfig(cfg *config) error {
	if cfg.cgroupPath != "" {
		if cfg.targetID != "" {
			return errors.New("--cgroup-path and --target-id are mutually exclusive")
		}
		if strings.Contains(cfg.cgroupPath, "..") {
			return errors.New("--cgroup-path must not contain '..' path components")
		}
		return nil
	}
	if cfg.targetID == "" {
		return errors.New("--target-id is required (or use --cgroup-path)")
	}
	if !isValidContainerID(cfg.targetID) {
		return fmt.Errorf("invalid container ID %q: must be 12-64 hex characters", cfg.targetID)
	}
	return nil
}

const (
	minContainerIDLen = 12
	maxContainerIDLen = 64
)

// isValidContainerID checks that the string is a valid Docker container ID (12-64 hex chars).
func isValidContainerID(id string) bool {
	if len(id) < minContainerIDLen || len(id) > maxContainerIDLen {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func run(cfg config) error {
	version := detectCgroupVersion()

	var paths []string
	if cfg.cgroupPath != "" {
		paths = cgroupProcsPathsFromBase(cfg.cgroupPath, version)
	} else {
		driver := cfg.driver
		if driver == driverAuto {
			driver = detectDriver(version)
		}
		paths = cgroupProcsPaths(cfg.targetID, version, driver)
	}

	pid := os.Getpid()
	for _, procsPath := range paths {
		if err := writePID(procsPath, pid); err != nil {
			return fmt.Errorf("writing PID %d to %s: %w", pid, procsPath, err)
		}
	}

	// Resolve the full path to the command
	cmdPath := cfg.commandArgs[0]
	return execCommand(cmdPath, cfg.commandArgs, os.Environ())
}

// detectCgroupVersion checks whether the system uses cgroups v2 or v1.
// If /sys/fs/cgroup/cgroup.controllers exists, it's v2.
func detectCgroupVersion() cgroupVersion {
	if _, err := os.Stat(cgroupRoot + "/cgroup.controllers"); err == nil {
		return cgroupV2
	}
	return cgroupV1
}

// detectDriver infers the Docker cgroup driver by looking for Docker-specific
// cgroup directories. The /docker/ directory is created by the cgroupfs driver,
// while docker-*.scope entries under system.slice are created by the systemd driver.
// Falls back to cgroupfs (Docker's default) if neither is found.
func detectDriver(version cgroupVersion) cgroupDriver {
	if version == cgroupV2 {
		// Check for cgroupfs driver's /docker/ directory first (Docker's default)
		if _, err := os.Stat(cgroupRoot + "/docker"); err == nil {
			return driverCgroupfs
		}
		// Check for systemd driver's scope entries
		if _, err := os.Stat(cgroupRoot + "/system.slice"); err == nil {
			return driverSystemd
		}
	} else {
		// v1: check under the cpu controller hierarchy
		if _, err := os.Stat(cgroupRoot + "/cpu/docker"); err == nil {
			return driverCgroupfs
		}
		if _, err := os.Stat(cgroupRoot + "/cpu/system.slice"); err == nil {
			return driverSystemd
		}
	}
	return driverCgroupfs
}

// v1Controllers lists the cgroup v1 controller hierarchies that stress-ng needs
// for accurate resource accounting (CPU, memory, block I/O, CPU accounting, PIDs).
var v1Controllers = []string{"cpu", "memory", "blkio", "cpuacct", "pids"}

// cgroupProcsPaths returns the cgroup.procs paths to write the PID into.
// On cgroups v2 (unified hierarchy), a single write covers all controllers.
// On cgroups v1, each controller has a separate hierarchy requiring its own write.
func cgroupProcsPaths(targetID string, version cgroupVersion, driver cgroupDriver) []string {
	if version == cgroupV2 {
		if driver == driverSystemd {
			return []string{cgroupRoot + "/system.slice/docker-" + targetID + ".scope/cgroup.procs"}
		}
		return []string{cgroupRoot + "/docker/" + targetID + "/cgroup.procs"}
	}
	// cgroups v1: write to each controller hierarchy
	paths := make([]string, 0, len(v1Controllers))
	for _, ctrl := range v1Controllers {
		if driver == driverSystemd {
			paths = append(paths, cgroupRoot+"/"+ctrl+"/system.slice/docker-"+targetID+".scope/cgroup.procs")
		} else {
			paths = append(paths, cgroupRoot+"/"+ctrl+"/docker/"+targetID+"/cgroup.procs")
		}
	}
	return paths
}

// cgroupProcsPathsFromBase returns cgroup.procs paths using an explicit base cgroup path.
// On cgroups v2, a single path suffices. On v1, each controller hierarchy is addressed.
// path.Join is used to normalize double slashes when basePath has a leading slash.
func cgroupProcsPathsFromBase(basePath string, version cgroupVersion) []string {
	if version == cgroupV2 {
		return []string{path.Join(cgroupRoot, basePath, "cgroup.procs")}
	}
	paths := make([]string, 0, len(v1Controllers))
	for _, ctrl := range v1Controllers {
		paths = append(paths, path.Join(cgroupRoot, ctrl, basePath, "cgroup.procs"))
	}
	return paths
}

// writePID writes a PID to an existing cgroup.procs file.
// Uses O_WRONLY without O_CREATE to fail if the cgroup path doesn't exist.
func writePID(procsPath string, pid int) error {
	f, err := os.OpenFile(procsPath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d\n", pid)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}
