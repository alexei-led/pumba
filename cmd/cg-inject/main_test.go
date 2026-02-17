package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    config
		wantErr string
	}{
		{
			name: "basic with defaults",
			args: []string{"--target-id", "aabbccddeeff", "--", "/stress-ng", "--cpu", "1"},
			want: config{
				targetID:    "aabbccddeeff",
				driver:      driverAuto,
				commandArgs: []string{"/stress-ng", "--cpu", "1"},
			},
		},
		{
			name: "explicit cgroupfs driver",
			args: []string{"--target-id", "aabbccddeeff", "--cgroup-driver", "cgroupfs", "--", "/stress-ng"},
			want: config{
				targetID:    "aabbccddeeff",
				driver:      driverCgroupfs,
				commandArgs: []string{"/stress-ng"},
			},
		},
		{
			name: "explicit systemd driver",
			args: []string{"--target-id", "ddeeff112233", "--cgroup-driver", "systemd", "--", "/stress-ng", "-v"},
			want: config{
				targetID:    "ddeeff112233",
				driver:      driverSystemd,
				commandArgs: []string{"/stress-ng", "-v"},
			},
		},
		{
			name: "auto driver explicit",
			args: []string{"--cgroup-driver", "auto", "--target-id", "112233aabbcc", "--", "/bin/sh"},
			want: config{
				targetID:    "112233aabbcc",
				driver:      driverAuto,
				commandArgs: []string{"/bin/sh"},
			},
		},
		{
			name:    "missing separator",
			args:    []string{"--target-id", "aabbccddeeff"},
			wantErr: "missing '--' separator before command",
		},
		{
			name:    "no command after separator",
			args:    []string{"--target-id", "aabbccddeeff", "--"},
			wantErr: "no command specified after '--'",
		},
		{
			name:    "invalid container ID",
			args:    []string{"--target-id", "not-a-hex-id", "--", "/stress-ng"},
			wantErr: "invalid container ID",
		},
		{
			name:    "container ID too short",
			args:    []string{"--target-id", "abc12", "--", "/stress-ng"},
			wantErr: "invalid container ID",
		},
		{
			name:    "missing target-id",
			args:    []string{"--", "/stress-ng"},
			wantErr: "--target-id is required",
		},
		{
			name:    "target-id without value",
			args:    []string{"--target-id", "--", "/stress-ng"},
			wantErr: "--target-id requires a value",
		},
		{
			name:    "cgroup-driver without value",
			args:    []string{"--target-id", "abc", "--cgroup-driver", "--", "/stress-ng"},
			wantErr: "--cgroup-driver requires a value",
		},
		{
			name:    "unknown cgroup driver",
			args:    []string{"--target-id", "abc", "--cgroup-driver", "nope", "--", "/stress-ng"},
			wantErr: `unknown cgroup driver "nope"`,
		},
		{
			name:    "unknown flag",
			args:    []string{"--target-id", "abc", "--bogus", "--", "/stress-ng"},
			wantErr: `unknown flag "--bogus"`,
		},
		{
			name: "cgroup-path instead of target-id",
			args: []string{"--cgroup-path", "kubepods/burstable/pod-abc/container123", "--", "/stress-ng", "--cpu", "1"},
			want: config{
				cgroupPath:  "kubepods/burstable/pod-abc/container123",
				driver:      driverAuto,
				commandArgs: []string{"/stress-ng", "--cpu", "1"},
			},
		},
		{
			name:    "cgroup-path and target-id mutually exclusive",
			args:    []string{"--cgroup-path", "kubepods/foo", "--target-id", "aabbccddeeff", "--", "/stress-ng"},
			wantErr: "--cgroup-path and --target-id are mutually exclusive",
		},
		{
			name:    "cgroup-path without value",
			args:    []string{"--cgroup-path", "--", "/stress-ng"},
			wantErr: "--cgroup-path requires a value",
		},
		{
			name:    "cgroup-path with path traversal",
			args:    []string{"--cgroup-path", "kubepods/../../etc", "--", "/stress-ng"},
			wantErr: "must not contain '..'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.targetID != tt.want.targetID {
				t.Errorf("targetID = %q, want %q", got.targetID, tt.want.targetID)
			}
			if got.cgroupPath != tt.want.cgroupPath {
				t.Errorf("cgroupPath = %q, want %q", got.cgroupPath, tt.want.cgroupPath)
			}
			if got.driver != tt.want.driver {
				t.Errorf("driver = %q, want %q", got.driver, tt.want.driver)
			}
			if len(got.commandArgs) != len(tt.want.commandArgs) {
				t.Fatalf("commandArgs len = %d, want %d", len(got.commandArgs), len(tt.want.commandArgs))
			}
			for i := range got.commandArgs {
				if got.commandArgs[i] != tt.want.commandArgs[i] {
					t.Errorf("commandArgs[%d] = %q, want %q", i, got.commandArgs[i], tt.want.commandArgs[i])
				}
			}
		})
	}
}

func TestCgroupProcsPaths(t *testing.T) {
	tests := []struct {
		name     string
		targetID string
		version  cgroupVersion
		driver   cgroupDriver
		want     []string
	}{
		{
			name:     "v2 cgroupfs",
			targetID: "abc123",
			version:  cgroupV2,
			driver:   driverCgroupfs,
			want:     []string{"/test/cgroup/docker/abc123/cgroup.procs"},
		},
		{
			name:     "v2 systemd",
			targetID: "def456",
			version:  cgroupV2,
			driver:   driverSystemd,
			want:     []string{"/test/cgroup/system.slice/docker-def456.scope/cgroup.procs"},
		},
		{
			name:     "v1 cgroupfs",
			targetID: "abc123",
			version:  cgroupV1,
			driver:   driverCgroupfs,
			want: []string{
				"/test/cgroup/cpu/docker/abc123/cgroup.procs",
				"/test/cgroup/memory/docker/abc123/cgroup.procs",
				"/test/cgroup/blkio/docker/abc123/cgroup.procs",
				"/test/cgroup/cpuacct/docker/abc123/cgroup.procs",
				"/test/cgroup/pids/docker/abc123/cgroup.procs",
			},
		},
		{
			name:     "v1 systemd",
			targetID: "aabbccdd1122",
			version:  cgroupV1,
			driver:   driverSystemd,
			want: []string{
				"/test/cgroup/cpu/system.slice/docker-aabbccdd1122.scope/cgroup.procs",
				"/test/cgroup/memory/system.slice/docker-aabbccdd1122.scope/cgroup.procs",
				"/test/cgroup/blkio/system.slice/docker-aabbccdd1122.scope/cgroup.procs",
				"/test/cgroup/cpuacct/system.slice/docker-aabbccdd1122.scope/cgroup.procs",
				"/test/cgroup/pids/system.slice/docker-aabbccdd1122.scope/cgroup.procs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origRoot := cgroupRoot
			cgroupRoot = "/test/cgroup"
			defer func() { cgroupRoot = origRoot }()

			got := cgroupProcsPaths(tt.targetID, tt.version, tt.driver)
			if len(got) != len(tt.want) {
				t.Fatalf("cgroupProcsPaths() returned %d paths, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("cgroupProcsPaths()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCgroupProcsPathsFromBase(t *testing.T) {
	origRoot := cgroupRoot
	defer func() { cgroupRoot = origRoot }()
	cgroupRoot = "/test/cgroup"

	tests := []struct {
		name     string
		basePath string
		version  cgroupVersion
		want     []string
	}{
		{
			name:     "v2 K8s path",
			basePath: "kubepods/burstable/pod-abc123/deadbeef1234",
			version:  cgroupV2,
			want:     []string{"/test/cgroup/kubepods/burstable/pod-abc123/deadbeef1234/cgroup.procs"},
		},
		{
			name:     "v1 K8s path iterates controllers",
			basePath: "kubepods/burstable/pod-abc123/deadbeef1234",
			version:  cgroupV1,
			want: []string{
				"/test/cgroup/cpu/kubepods/burstable/pod-abc123/deadbeef1234/cgroup.procs",
				"/test/cgroup/memory/kubepods/burstable/pod-abc123/deadbeef1234/cgroup.procs",
				"/test/cgroup/blkio/kubepods/burstable/pod-abc123/deadbeef1234/cgroup.procs",
				"/test/cgroup/cpuacct/kubepods/burstable/pod-abc123/deadbeef1234/cgroup.procs",
				"/test/cgroup/pids/kubepods/burstable/pod-abc123/deadbeef1234/cgroup.procs",
			},
		},
		{
			name:     "v2 simple Docker path",
			basePath: "docker/abc123",
			version:  cgroupV2,
			want:     []string{"/test/cgroup/docker/abc123/cgroup.procs"},
		},
		{
			name:     "v2 leading slash normalized",
			basePath: "/kubepods/burstable/pod-abc123/deadbeef1234",
			version:  cgroupV2,
			want:     []string{"/test/cgroup/kubepods/burstable/pod-abc123/deadbeef1234/cgroup.procs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cgroupProcsPathsFromBase(tt.basePath, tt.version)
			if len(got) != len(tt.want) {
				t.Fatalf("returned %d paths, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetectCgroupVersion(t *testing.T) {
	origRoot := cgroupRoot
	defer func() { cgroupRoot = origRoot }()

	t.Run("v2 when cgroup.controllers exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir
		// Create cgroup.controllers to simulate v2
		if err := os.WriteFile(filepath.Join(tmpDir, "cgroup.controllers"), []byte("cpu memory"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := detectCgroupVersion(); got != cgroupV2 {
			t.Errorf("detectCgroupVersion() = %v, want cgroupV2", got)
		}
	})

	t.Run("v1 when cgroup.controllers absent", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir
		if got := detectCgroupVersion(); got != cgroupV1 {
			t.Errorf("detectCgroupVersion() = %v, want cgroupV1", got)
		}
	})
}

func TestDetectDriver(t *testing.T) {
	origRoot := cgroupRoot
	defer func() { cgroupRoot = origRoot }()

	t.Run("v2 with docker dir returns cgroupfs", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir
		if err := os.MkdirAll(filepath.Join(tmpDir, "docker"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got := detectDriver(cgroupV2); got != driverCgroupfs {
			t.Errorf("detectDriver(v2) = %q, want cgroupfs", got)
		}
	})

	t.Run("v2 with docker dir prefers cgroupfs over systemd", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir
		// Both exist (systemd host with Docker using cgroupfs driver)
		if err := os.MkdirAll(filepath.Join(tmpDir, "docker"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(tmpDir, "system.slice"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got := detectDriver(cgroupV2); got != driverCgroupfs {
			t.Errorf("detectDriver(v2) = %q, want cgroupfs (docker dir should take priority)", got)
		}
	})

	t.Run("v2 with only system.slice returns systemd", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir
		if err := os.MkdirAll(filepath.Join(tmpDir, "system.slice"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got := detectDriver(cgroupV2); got != driverSystemd {
			t.Errorf("detectDriver(v2) = %q, want systemd", got)
		}
	})

	t.Run("v2 with neither returns cgroupfs", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir
		if got := detectDriver(cgroupV2); got != driverCgroupfs {
			t.Errorf("detectDriver(v2) = %q, want cgroupfs", got)
		}
	})

	t.Run("v1 with cpu/docker returns cgroupfs", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir
		if err := os.MkdirAll(filepath.Join(tmpDir, "cpu", "docker"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got := detectDriver(cgroupV1); got != driverCgroupfs {
			t.Errorf("detectDriver(v1) = %q, want cgroupfs", got)
		}
	})

	t.Run("v1 with only cpu/system.slice returns systemd", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir
		if err := os.MkdirAll(filepath.Join(tmpDir, "cpu", "system.slice"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got := detectDriver(cgroupV1); got != driverSystemd {
			t.Errorf("detectDriver(v1) = %q, want systemd", got)
		}
	})

	t.Run("v1 without either returns cgroupfs", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir
		if got := detectDriver(cgroupV1); got != driverCgroupfs {
			t.Errorf("detectDriver(v1) = %q, want cgroupfs", got)
		}
	})
}

func TestWritePID(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cgroup.procs")

	// Pre-create the file (writePID uses O_WRONLY without O_CREATE, like real cgroup.procs)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writePID(path, 12345); err != nil {
		t.Fatalf("writePID() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != "12345\n" {
		t.Errorf("file content = %q, want %q", string(data), "12345\n")
	}
}

func TestWritePID_ErrorOnMissingDir(t *testing.T) {
	err := writePID("/nonexistent/dir/cgroup.procs", 1)
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

func TestRun(t *testing.T) {
	origRoot := cgroupRoot
	origExec := execCommand
	defer func() {
		cgroupRoot = origRoot
		execCommand = origExec
	}()

	t.Run("successful run with cgroupfs v2", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir

		// Create v2 indicator
		if err := os.WriteFile(filepath.Join(tmpDir, "cgroup.controllers"), []byte("cpu"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create the target cgroup directory and cgroup.procs file
		targetDir := filepath.Join(tmpDir, "docker", "abc123")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, "cgroup.procs"), nil, 0o644); err != nil {
			t.Fatal(err)
		}

		var execCalled bool
		var execPath string
		var execArgs []string
		execCommand = func(argv0 string, argv []string, envv []string) error {
			execCalled = true
			execPath = argv0
			execArgs = argv
			return nil
		}

		cfg := config{
			targetID:    "abc123",
			driver:      driverAuto,
			commandArgs: []string{"/stress-ng", "--cpu", "1"},
		}

		if err := run(cfg); err != nil {
			t.Fatalf("run() error: %v", err)
		}

		if !execCalled {
			t.Fatal("exec was not called")
		}
		if execPath != "/stress-ng" {
			t.Errorf("exec path = %q, want /stress-ng", execPath)
		}
		if len(execArgs) != 3 || execArgs[0] != "/stress-ng" || execArgs[1] != "--cpu" || execArgs[2] != "1" {
			t.Errorf("exec args = %v, want [/stress-ng --cpu 1]", execArgs)
		}

		// Verify PID was written
		data, err := os.ReadFile(filepath.Join(targetDir, "cgroup.procs"))
		if err != nil {
			t.Fatalf("reading cgroup.procs: %v", err)
		}
		if len(data) == 0 {
			t.Error("cgroup.procs is empty")
		}
	})

	t.Run("successful run with cgroup-path v2", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir

		// Create v2 indicator
		if err := os.WriteFile(filepath.Join(tmpDir, "cgroup.controllers"), []byte("cpu"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create the K8s-style cgroup directory and cgroup.procs file
		targetDir := filepath.Join(tmpDir, "kubepods", "burstable", "pod-abc", "deadbeef1234")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, "cgroup.procs"), nil, 0o644); err != nil {
			t.Fatal(err)
		}

		var execCalled bool
		execCommand = func(argv0 string, argv []string, _ []string) error {
			execCalled = true
			return nil
		}

		cfg := config{
			cgroupPath:  "kubepods/burstable/pod-abc/deadbeef1234",
			commandArgs: []string{"/stress-ng", "--cpu", "1"},
		}

		if err := run(cfg); err != nil {
			t.Fatalf("run() error: %v", err)
		}
		if !execCalled {
			t.Fatal("exec was not called")
		}

		data, err := os.ReadFile(filepath.Join(targetDir, "cgroup.procs"))
		if err != nil {
			t.Fatalf("reading cgroup.procs: %v", err)
		}
		if len(data) == 0 {
			t.Error("cgroup.procs is empty")
		}
	})

	t.Run("successful run with cgroup-path v1", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir

		// v1: no cgroup.controllers file
		// Create controller dirs with cgroup.procs
		basePath := "kubepods/burstable/pod-abc/deadbeef1234"
		for _, ctrl := range v1Controllers {
			dir := filepath.Join(tmpDir, ctrl, basePath)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "cgroup.procs"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}

		var execCalled bool
		execCommand = func(string, []string, []string) error {
			execCalled = true
			return nil
		}

		cfg := config{
			cgroupPath:  basePath,
			commandArgs: []string{"/stress-ng"},
		}

		if err := run(cfg); err != nil {
			t.Fatalf("run() error: %v", err)
		}
		if !execCalled {
			t.Fatal("exec was not called")
		}

		// Verify PID was written to all controllers
		for _, ctrl := range v1Controllers {
			data, err := os.ReadFile(filepath.Join(tmpDir, ctrl, basePath, "cgroup.procs"))
			if err != nil {
				t.Fatalf("reading %s cgroup.procs: %v", ctrl, err)
			}
			if len(data) == 0 {
				t.Errorf("%s cgroup.procs is empty", ctrl)
			}
		}
	})

	t.Run("write PID fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir

		execCommand = func(string, []string, []string) error {
			t.Fatal("exec should not be called on write failure")
			return nil
		}

		cfg := config{
			targetID:    "aabbccddeeff",
			driver:      driverCgroupfs,
			commandArgs: []string{"/stress-ng"},
		}

		err := run(cfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("exec failure propagated", func(t *testing.T) {
		tmpDir := t.TempDir()
		cgroupRoot = tmpDir

		// Create v2 indicator so cgroupfs uses /docker/ path
		if err := os.WriteFile(filepath.Join(tmpDir, "cgroup.controllers"), []byte("cpu"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create target cgroup directory and cgroup.procs file
		targetDir := filepath.Join(tmpDir, "docker", "aabbccddee11")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, "cgroup.procs"), nil, 0o644); err != nil {
			t.Fatal(err)
		}

		execCommand = func(string, []string, []string) error {
			return errors.New("exec failed")
		}

		cfg := config{
			targetID:    "aabbccddee11",
			driver:      driverCgroupfs,
			commandArgs: []string{"/stress-ng"},
		}

		err := run(cfg)
		if err == nil || !strings.Contains(err.Error(), "exec failed") {
			t.Fatalf("expected exec error, got: %v", err)
		}
	})
}
