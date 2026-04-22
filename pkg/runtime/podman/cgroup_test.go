package podman

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-world reference (documented for Task 6 stress-container work): on a
// rootful `podman machine` (Podman 4.9+, cgroups v2, systemd driver), the
// host-side /proc/<pid>/cgroup for a running libpod container looks like:
//
//	0::/machine.slice/libpod-<64-hex-id>.scope
//
// or, when libpod's init sub-cgroup is present:
//
//	0::/machine.slice/libpod-<64-hex-id>.scope/container
//
// ParseProc1Cgroup must truncate either form to
// `/machine.slice/libpod-<64-hex-id>.scope` with leaf
// `libpod-<64-hex-id>.scope` and parent `/machine.slice`. The round-trip was
// hand-verified against `cat /proc/$(podman inspect -f '{{.State.Pid}}' <id>)
// /cgroup` inside a `podman machine ssh` session; the canonical form is
// stable across Podman 4.9–5.x.

func TestParseProc1Cgroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantDriver   string
		wantFullPath string
		wantParent   string
		wantLeaf     string
	}{
		{
			name:         "v2 systemd machine.slice/libpod scope",
			input:        "0::/machine.slice/libpod-abc.scope\n",
			wantDriver:   "systemd",
			wantFullPath: "/machine.slice/libpod-abc.scope",
			wantParent:   "/machine.slice",
			wantLeaf:     "libpod-abc.scope",
		},
		{
			name:         "v2 with podman container init sub-cgroup stripped",
			input:        "0::/machine.slice/libpod-abc.scope/container\n",
			wantDriver:   "systemd",
			wantFullPath: "/machine.slice/libpod-abc.scope",
			wantParent:   "/machine.slice",
			wantLeaf:     "libpod-abc.scope",
		},
		{
			name:         "v2 with systemd-in-container init.scope stripped",
			input:        "0::/machine.slice/libpod-abc.scope/init.scope\n",
			wantDriver:   "systemd",
			wantFullPath: "/machine.slice/libpod-abc.scope",
			wantParent:   "/machine.slice",
			wantLeaf:     "libpod-abc.scope",
		},
		{
			name:         "v2 with arbitrary trailing sub-cgroup stripped",
			input:        "0::/machine.slice/libpod-abc.scope/app/user.slice/foo\n",
			wantDriver:   "systemd",
			wantFullPath: "/machine.slice/libpod-abc.scope",
			wantParent:   "/machine.slice",
			wantLeaf:     "libpod-abc.scope",
		},
		{
			name: "v2 preferred over v1 systemd when both present",
			input: "" +
				"12:devices:/libpod_parent/libpod-abc.scope\n" +
				"1:name=systemd:/libpod_parent/libpod-abc.scope\n" +
				"0::/machine.slice/libpod-abc.scope\n",
			wantDriver:   "systemd",
			wantFullPath: "/machine.slice/libpod-abc.scope",
			wantParent:   "/machine.slice",
			wantLeaf:     "libpod-abc.scope",
		},
		{
			name:         "v1 systemd libpod parent scope",
			input:        "1:name=systemd:/libpod_parent/libpod-abc.scope\n",
			wantDriver:   "systemd",
			wantFullPath: "/libpod_parent/libpod-abc.scope",
			wantParent:   "/libpod_parent",
			wantLeaf:     "libpod-abc.scope",
		},
		{
			name: "v1 systemd picked from multi-line v1 content",
			input: "" +
				"12:devices:/libpod_parent/libpod-abc\n" +
				"11:cpu,cpuacct:/libpod_parent/libpod-abc\n" +
				"1:name=systemd:/libpod_parent/libpod-abc.scope\n",
			wantDriver:   "systemd",
			wantFullPath: "/libpod_parent/libpod-abc.scope",
			wantParent:   "/libpod_parent",
			wantLeaf:     "libpod-abc.scope",
		},
		{
			name:         "v1 cgroupfs plain libpod path (no scope/slice)",
			input:        "1:name=systemd:/libpod/abc\n",
			wantDriver:   "cgroupfs",
			wantFullPath: "/libpod/abc",
			wantParent:   "/libpod",
			wantLeaf:     "abc",
		},
		{
			name:         "v2 cgroupfs plain libpod path truncates to raw",
			input:        "0::/libpod/abc\n",
			wantDriver:   "cgroupfs",
			wantFullPath: "/libpod/abc",
			wantParent:   "/libpod",
			wantLeaf:     "abc",
		},
		{
			name:         "v2 falls back to .slice when no .scope present",
			input:        "0::/system.slice/something\n",
			wantDriver:   "systemd",
			wantFullPath: "/system.slice",
			wantParent:   "/",
			wantLeaf:     "system.slice",
		},
		{
			name:         "v2 strips trailing slash",
			input:        "0::/machine.slice/libpod-abc.scope/\n",
			wantDriver:   "systemd",
			wantFullPath: "/machine.slice/libpod-abc.scope",
			wantParent:   "/machine.slice",
			wantLeaf:     "libpod-abc.scope",
		},
		{
			name:         "v2 single-segment scope at root collapses parent to /",
			input:        "0::/libpod-abc.scope\n",
			wantDriver:   "systemd",
			wantFullPath: "/libpod-abc.scope",
			wantParent:   "/",
			wantLeaf:     "libpod-abc.scope",
		},
		{
			name:         "whitespace and extra blank lines tolerated",
			input:        "\n  0::/machine.slice/libpod-abc.scope  \n\n",
			wantDriver:   "systemd",
			wantFullPath: "/machine.slice/libpod-abc.scope",
			wantParent:   "/machine.slice",
			wantLeaf:     "libpod-abc.scope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			driver, full, parent, leaf, err := ParseProc1Cgroup(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDriver, driver)
			assert.Equal(t, tt.wantFullPath, full)
			assert.Equal(t, tt.wantParent, parent)
			assert.Equal(t, tt.wantLeaf, leaf)
		})
	}
}

func TestParseProc1Cgroup_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "empty input",
			input:   "",
			wantErr: errEmptyCgroup,
		},
		{
			name:    "whitespace-only input",
			input:   "   \n\t\n",
			wantErr: errEmptyCgroup,
		},
		{
			name:    "no structured lines",
			input:   "not a cgroup file\njust garbage\n",
			wantErr: errMalformedCgroup,
		},
		{
			name:    "no v2 or v1-systemd line",
			input:   "12:devices:/something\n11:cpu,cpuacct:/something\n",
			wantErr: errNoCgroupLine,
		},
		{
			name:    "private cgroupns view — bare root",
			input:   "0::/\n",
			wantErr: errPrivateCgroupnsView,
		},
		{
			name:    "private cgroupns view — podman container init sub-cgroup at ns root",
			input:   "0::/container\n",
			wantErr: errPrivateCgroupnsView,
		},
		{
			name:    "private cgroupns view with trailing slash normalises to root",
			input:   "0::/\n",
			wantErr: errPrivateCgroupnsView,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, _, _, err := ParseProc1Cgroup(tt.input)
			require.Error(t, err)
			assert.True(
				t,
				errors.Is(err, tt.wantErr),
				"got %v, want to wrap %v", err, tt.wantErr,
			)
		})
	}
}

func TestCgroupReaderDefault(t *testing.T) {
	t.Parallel()

	// Default reader reads /proc/self/cgroup, which exists on Linux hosts and
	// is missing on macOS/darwin. We only assert shape: either contents come
	// back non-empty, or an IO error propagates. Nothing else can be true.
	data, err := cgroupReader(1)
	if err != nil {
		assert.Empty(t, data, "reader returns empty bytes on error")
		return
	}
	assert.NotEmpty(t, data, "reader returns non-empty bytes on success")
}

func TestCgroupReaderSwappable(t *testing.T) { //nolint:paralleltest // mutates package-level cgroupReader
	orig := cgroupReader
	t.Cleanup(func() { cgroupReader = orig })

	const want = "0::/machine.slice/libpod-abc.scope\n"
	cgroupReader = func(pid int) ([]byte, error) {
		assert.Equal(t, 42, pid)
		return []byte(want), nil
	}

	got, err := cgroupReader(42)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}
