package podman

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	dockerapi "github.com/docker/docker/client"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newStressTestTarget returns a small helper container and a stub /proc cgroup
// payload that parses to a systemd v2 scope under machine.slice — the shape
// observed on a default rootful `podman machine`.
const v2SystemdCgroup = "0::/machine.slice/libpod-abc.scope\n"

func newStressTarget() *ctr.Container {
	return &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "/target",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}
}

func stubCgroupReader(t *testing.T, fn func(pid int) ([]byte, error)) {
	t.Helper()
	orig := cgroupReader
	cgroupReader = fn
	t.Cleanup(func() { cgroupReader = orig })
}

func runningInspect(pid int) ctypes.InspectResponse {
	return ctypes.InspectResponse{
		ContainerJSONBase: &ctypes.ContainerJSONBase{
			ID: "abc123",
			State: &ctypes.State{
				Running: true,
				Pid:     pid,
			},
		},
	}
}

// fakeConn implements net.Conn with Close-only behavior; sufficient for the
// attach.Close() path in drainStressOutput.
type fakeConn struct{ net.Conn }

func (fakeConn) Close() error { return nil }

func newAttachWith(body string) types.HijackedResponse {
	return types.HijackedResponse{
		Conn:   fakeConn{},
		Reader: bufio.NewReader(strings.NewReader(body)),
	}
}

func TestStressContainer_Dryrun(t *testing.T) {
	api := mocks.NewAPIClient(t)
	p := &podmanClient{api: api, socketURI: "unix:///run/podman/podman.sock"}

	id, outCh, errCh, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, true)
	require.NoError(t, err)
	require.Empty(t, id)
	require.Nil(t, outCh)
	require.Nil(t, errCh)
	// no api calls expected — mocks.NewAPIClient auto-asserts at cleanup.
}

func TestStressContainer_RootlessGuard(t *testing.T) {
	api := mocks.NewAPIClient(t)
	p := &podmanClient{api: api, rootless: true, socketURI: "unix:///run/user/1000/podman/podman.sock"}

	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stress")
	require.Contains(t, err.Error(), "rootful podman")
	require.Contains(t, err.Error(), p.socketURI)
}

func TestStressContainer_InspectError(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(ctypes.InspectResponse{}, errors.New("inspect boom")).Once()

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inspect target")
	require.Contains(t, err.Error(), "inspect boom")
}

func TestStressContainer_NoPID(t *testing.T) {
	api := mocks.NewAPIClient(t)
	// Not-running state: Pid is zero.
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(ctypes.InspectResponse{
		ContainerJSONBase: &ctypes.ContainerJSONBase{ID: "abc123", State: &ctypes.State{Pid: 0}},
	}, nil).Once()

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not running")
}

func TestStressContainer_CgroupReaderError(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(4242), nil).Once()
	stubCgroupReader(t, func(pid int) ([]byte, error) {
		require.Equal(t, 4242, pid)
		return nil, errors.New("no such file")
	})

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read /proc/4242/cgroup")
	require.Contains(t, err.Error(), "no such file")
}

func TestStressContainer_CgroupParseError(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(4242), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte("garbage-no-colons\n"), nil })

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse target cgroup")
}

func TestStressContainer_DefaultSystemd_SetsCgroupParent(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(4242), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte(v2SystemdCgroup), nil })

	var gotConfig *ctypes.Config
	var gotHost *ctypes.HostConfig
	api.EXPECT().ContainerCreate(
		mock.Anything,
		mock.MatchedBy(func(c *ctypes.Config) bool { gotConfig = c; return true }),
		mock.MatchedBy(func(hc *ctypes.HostConfig) bool { gotHost = hc; return true }),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(ctypes.CreateResponse{}, errors.New("stop here")).Once()

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "2"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create stress-ng container")

	require.NotNil(t, gotConfig)
	require.Equal(t, "stress-ng:latest", gotConfig.Image)
	require.Equal(t, "true", gotConfig.Labels[skipLabelKey])
	require.Equal(t, []string{"/stress-ng"}, []string(gotConfig.Entrypoint))
	require.Equal(t, []string{"--cpu", "2"}, []string(gotConfig.Cmd))

	require.NotNil(t, gotHost)
	require.True(t, gotHost.AutoRemove)
	require.Equal(t, "/machine.slice", gotHost.Resources.CgroupParent)
	require.Empty(t, gotHost.Binds)
	require.Empty(t, string(gotHost.CgroupnsMode))
}

func TestStressContainer_DefaultCgroupfs_SetsCgroupParent(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(99), nil).Once()
	// v1 libpod cgroupfs shape: driver=cgroupfs, fullPath=/libpod/abc, parent=/libpod.
	// Default mode on cgroupfs nests the sidecar under the target's full path
	// to match the Docker runtime's child-cgroup semantics (shared OOM scope).
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte("1:name=systemd:/libpod/abc\n"), nil })

	var gotHost *ctypes.HostConfig
	api.EXPECT().ContainerCreate(
		mock.Anything, mock.Anything,
		mock.MatchedBy(func(hc *ctypes.HostConfig) bool { gotHost = hc; return true }),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(ctypes.CreateResponse{}, errors.New("stop here")).Once()

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.NotNil(t, gotHost)
	require.Equal(t, "/libpod/abc", gotHost.Resources.CgroupParent)
}

func TestStressContainer_InjectCgroup_UsesFullPath(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(7777), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte(v2SystemdCgroup), nil })

	var gotConfig *ctypes.Config
	var gotHost *ctypes.HostConfig
	api.EXPECT().ContainerCreate(
		mock.Anything,
		mock.MatchedBy(func(c *ctypes.Config) bool { gotConfig = c; return true }),
		mock.MatchedBy(func(hc *ctypes.HostConfig) bool { gotHost = hc; return true }),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(ctypes.CreateResponse{}, errors.New("stop here")).Once()

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, true, false)
	require.Error(t, err)

	require.NotNil(t, gotConfig)
	require.Equal(t, []string{"/cg-inject"}, []string(gotConfig.Entrypoint))
	require.Equal(t,
		[]string{"--cgroup-path", "/machine.slice/libpod-abc.scope", "--", "/stress-ng", "--cpu", "1"},
		[]string(gotConfig.Cmd),
	)

	require.NotNil(t, gotHost)
	require.Equal(t, ctypes.CgroupnsMode("host"), gotHost.CgroupnsMode)
	require.Equal(t, []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"}, gotHost.Binds)
	require.Empty(t, gotHost.Resources.CgroupParent)
	require.True(t, gotHost.AutoRemove)
}

func TestStressContainer_PullError(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(42), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte(v2SystemdCgroup), nil })
	api.EXPECT().ImagePull(mock.Anything, "stress-ng:latest", mock.Anything).Return(nil, errors.New("net down")).Once()

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", true, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pull stress-ng image")
	require.Contains(t, err.Error(), "net down")
}

func TestStressContainer_CreateError(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(42), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte(v2SystemdCgroup), nil })
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(ctypes.CreateResponse{}, errors.New("disk full")).Once()

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create stress-ng container")
	require.Contains(t, err.Error(), "disk full")
}

func TestStressContainer_AttachError(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(42), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte(v2SystemdCgroup), nil })
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(ctypes.CreateResponse{ID: "sidecar1"}, nil).Once()
	api.EXPECT().ContainerAttach(mock.Anything, "sidecar1", mock.Anything).
		Return(types.HijackedResponse{}, errors.New("attach boom")).Once()
	// removeOnError must clean up the created-but-not-started container.
	api.EXPECT().ContainerRemove(mock.Anything, "sidecar1", ctypes.RemoveOptions{Force: true}).Return(nil).Once()

	p := &podmanClient{api: api}
	id, outCh, errCh, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "attach stress-ng container")
	require.Empty(t, id)
	require.Nil(t, outCh)
	require.Nil(t, errCh)
}

func TestStressContainer_Success_FullFlow(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(42), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte(v2SystemdCgroup), nil })
	api.EXPECT().ImagePull(mock.Anything, "stress-ng:latest", mock.Anything).
		Return(io.NopCloser(strings.NewReader("")), nil).Once()
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(ctypes.CreateResponse{ID: "sidecar1"}, nil).Once()
	api.EXPECT().ContainerAttach(mock.Anything, "sidecar1", mock.Anything).
		Return(newAttachWith("stress-ng done\n"), nil).Once()
	api.EXPECT().ContainerStart(mock.Anything, "sidecar1", mock.Anything).Return(nil).Once()
	// drainStressOutput inspects post-EOF for exit code.
	api.EXPECT().ContainerInspect(mock.Anything, "sidecar1").Return(ctypes.InspectResponse{
		ContainerJSONBase: &ctypes.ContainerJSONBase{
			ID:    "sidecar1",
			State: &ctypes.State{ExitCode: 0},
		},
	}, nil).Once()

	p := &podmanClient{api: api}
	id, outCh, errCh, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", true, 10*time.Second, false, false)
	require.NoError(t, err)
	require.Equal(t, "sidecar1", id)
	require.NotNil(t, outCh)
	require.NotNil(t, errCh)

	select {
	case out := <-outCh:
		require.Contains(t, out, "stress-ng done")
	case err := <-errCh:
		t.Fatalf("unexpected error from drain goroutine: %v", err)
	case <-time.After(time.Second):
		t.Fatal("drain goroutine did not complete")
	}
	// outerr closes once output is delivered; a receive returns zero+false.
	select {
	case _, ok := <-errCh:
		require.False(t, ok, "outerr channel should be closed")
	case <-time.After(time.Second):
		t.Fatal("outerr channel should be closed after success")
	}
}

func TestStressContainer_StartError(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(42), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte(v2SystemdCgroup), nil })
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(ctypes.CreateResponse{ID: "sidecar1"}, nil).Once()
	api.EXPECT().ContainerAttach(mock.Anything, "sidecar1", mock.Anything).
		Return(newAttachWith(""), nil).Once()
	api.EXPECT().ContainerStart(mock.Anything, "sidecar1", mock.Anything).
		Return(errors.New("start boom")).Once()
	// removeOnError must clean up; no drain goroutine is launched after start failure.
	api.EXPECT().ContainerRemove(mock.Anything, "sidecar1", ctypes.RemoveOptions{Force: true}).Return(nil).Once()

	p := &podmanClient{api: api}
	id, outCh, errCh, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "start stress-ng container")
	require.Empty(t, id)
	require.Nil(t, outCh)
	require.Nil(t, errCh)
}

func TestBuildStressConfig_DefaultSystemd(t *testing.T) {
	cfg, hc := buildStressConfig("img", []string{"--cpu", "1"}, driverSystemd, "/machine.slice/libpod-abc.scope", "/machine.slice", "/machine.slice/libpod-abc.scope", false)
	require.Equal(t, "img", cfg.Image)
	require.Equal(t, "true", cfg.Labels[skipLabelKey])
	require.Equal(t, []string{"/stress-ng"}, []string(cfg.Entrypoint))
	require.Equal(t, []string{"--cpu", "1"}, []string(cfg.Cmd))
	require.True(t, hc.AutoRemove)
	require.Equal(t, "/machine.slice", hc.Resources.CgroupParent)
	require.Empty(t, hc.Binds)
}

func TestBuildStressConfig_DefaultCgroupfs(t *testing.T) {
	cfg, hc := buildStressConfig("img", []string{"--cpu", "1"}, driverCgroupfs, "/libpod/abc", "/libpod", "/libpod/abc", false)
	require.Equal(t, []string{"/stress-ng"}, []string(cfg.Entrypoint))
	require.Equal(t, []string{"--cpu", "1"}, []string(cfg.Cmd))
	require.Equal(t, "/libpod/abc", hc.Resources.CgroupParent, "cgroupfs nests sidecar under target's full path for shared OOM scope")
}

func TestBuildStressConfig_Inject(t *testing.T) {
	cfg, hc := buildStressConfig("img", []string{"--cpu", "1"}, driverSystemd, "/machine.slice/libpod-abc.scope", "/machine.slice", "/machine.slice/libpod-abc.scope/container", true)
	require.Equal(t, []string{"/cg-inject"}, []string(cfg.Entrypoint))
	require.Equal(t,
		[]string{"--cgroup-path", "/machine.slice/libpod-abc.scope/container", "--", "/stress-ng", "--cpu", "1"},
		[]string(cfg.Cmd),
	)
	require.Equal(t, ctypes.CgroupnsMode("host"), hc.CgroupnsMode)
	require.Equal(t, []string{"SYS_ADMIN"}, []string(hc.CapAdd))
	require.Equal(t, []string{"label=disable"}, hc.SecurityOpt)
	require.Equal(t, []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"}, hc.Binds)
	require.Empty(t, hc.Resources.CgroupParent)
	require.True(t, hc.AutoRemove)
}

// TestStressContainer_InjectCgroup_UsesNestedLeaf confirms cg-inject is
// given the un-truncated `.scope/container` leaf when /proc/<pid>/cgroup
// reports Podman's libpod init sub-cgroup shape.
func TestStressContainer_InjectCgroup_UsesNestedLeaf(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(7777), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) {
		return []byte("0::/machine.slice/libpod-abc.scope/container\n"), nil
	})

	var gotConfig *ctypes.Config
	api.EXPECT().ContainerCreate(
		mock.Anything,
		mock.MatchedBy(func(c *ctypes.Config) bool { gotConfig = c; return true }),
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(ctypes.CreateResponse{}, errors.New("stop here")).Once()

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, true, false)
	require.Error(t, err)

	require.NotNil(t, gotConfig)
	require.Equal(t,
		[]string{"--cgroup-path", "/machine.slice/libpod-abc.scope/container", "--", "/stress-ng", "--cpu", "1"},
		[]string(gotConfig.Cmd),
		"cg-inject must target the nested `container/` leaf reported by /proc/<pid>/cgroup",
	)
}

// TestStressContainer_InjectCgroup_UsesScopeLeaf covers the shape where
// Podman has NOT created a nested `container/` sub-cgroup (observed on
// some host podman installs). The scope itself is the leaf and cg-inject
// must receive the un-decorated path.
func TestStressContainer_InjectCgroup_UsesScopeLeaf(t *testing.T) {
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(runningInspect(7777), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) {
		return []byte(v2SystemdCgroup), nil // scope-only leaf
	})

	var gotConfig *ctypes.Config
	api.EXPECT().ContainerCreate(
		mock.Anything,
		mock.MatchedBy(func(c *ctypes.Config) bool { gotConfig = c; return true }),
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(ctypes.CreateResponse{}, errors.New("stop here")).Once()

	p := &podmanClient{api: api}
	_, _, _, err := p.StressContainer(t.Context(), newStressTarget(), []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, true, false)
	require.Error(t, err)

	require.NotNil(t, gotConfig)
	require.Equal(t,
		[]string{"--cgroup-path", "/machine.slice/libpod-abc.scope", "--", "/stress-ng", "--cpu", "1"},
		[]string(gotConfig.Cmd),
		"cg-inject must not invent a `container/` suffix when /proc/<pid>/cgroup doesn't report one",
	)
}

// Compile-time check that mocks.APIClient satisfies apiBackend. Guards
// against drift between the Docker SDK interface and the narrow backend.
var _ apiBackend = (*mocks.APIClient)(nil)

// Compile-time check that *dockerapi.Client (the production SDK client)
// satisfies apiBackend. Declared in a test file so the production package
// doesn't import dockerapi just to prove this.
var _ apiBackend = (*dockerapi.Client)(nil)
