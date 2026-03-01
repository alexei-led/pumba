package containerd

import (
	"context"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	types "github.com/containerd/typeurl/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	metrictypes "github.com/containerd/containerd/api/types"
)

type stubImage struct{ containerd.Image }

type mockProcess struct {
	mock.Mock
}

func (m *mockProcess) ID() string  { return "exec-proc" }
func (m *mockProcess) Pid() uint32 { return 0 }
func (m *mockProcess) IO() cio.IO  { return nil }
func (m *mockProcess) Status(context.Context) (containerd.Status, error) {
	return containerd.Status{}, nil
}
func (m *mockProcess) CloseIO(context.Context, ...containerd.IOCloserOpts) error { return nil }
func (m *mockProcess) Resize(context.Context, uint32, uint32) error              { return nil }

func (m *mockProcess) Start(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *mockProcess) Kill(ctx context.Context, sig syscall.Signal, _ ...containerd.KillOpts) error {
	return m.Called(ctx, sig).Error(0)
}

func (m *mockProcess) Wait(ctx context.Context) (<-chan containerd.ExitStatus, error) {
	args := m.Called(ctx)
	if ch := args.Get(0); ch != nil {
		return ch.(<-chan containerd.ExitStatus), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockProcess) Delete(ctx context.Context, _ ...containerd.ProcessDeleteOpts) (*containerd.ExitStatus, error) {
	args := m.Called(ctx)
	return nil, args.Error(0)
}

type mockContainer struct {
	mock.Mock
}

func (m *mockContainer) ID() string {
	return m.Called().String(0)
}

func (m *mockContainer) Info(ctx context.Context, _ ...containerd.InfoOpts) (containers.Container, error) {
	args := m.Called(ctx)
	return args.Get(0).(containers.Container), args.Error(1)
}

func (m *mockContainer) Task(ctx context.Context, _ cio.Attach) (containerd.Task, error) {
	args := m.Called(ctx)
	if t := args.Get(0); t != nil {
		return t.(containerd.Task), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockContainer) Delete(ctx context.Context, _ ...containerd.DeleteOpts) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockContainer) NewTask(ctx context.Context, _ cio.Creator, _ ...containerd.NewTaskOpts) (containerd.Task, error) {
	args := m.Called(ctx)
	if t := args.Get(0); t != nil {
		return t.(containerd.Task), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockContainer) Spec(context.Context) (*oci.Spec, error)           { return nil, nil }
func (m *mockContainer) Image(context.Context) (containerd.Image, error)   { return nil, nil }
func (m *mockContainer) Labels(context.Context) (map[string]string, error) { return nil, nil }
func (m *mockContainer) SetLabels(context.Context, map[string]string) (map[string]string, error) {
	return nil, nil
}
func (m *mockContainer) Extensions(context.Context) (map[string]types.Any, error)        { return nil, nil }
func (m *mockContainer) Update(context.Context, ...containerd.UpdateContainerOpts) error { return nil }
func (m *mockContainer) Checkpoint(context.Context, string, ...containerd.CheckpointOpts) (containerd.Image, error) {
	return nil, nil
}
func (m *mockContainer) Restore(context.Context, cio.Creator, string) (int, error) { return 0, nil }

type mockTask struct {
	mock.Mock
	pid uint32
}

func (m *mockTask) Status(ctx context.Context) (containerd.Status, error) {
	args := m.Called(ctx)
	return args.Get(0).(containerd.Status), args.Error(1)
}

func (m *mockTask) ID() string  { return "" }
func (m *mockTask) Pid() uint32 { return m.pid }
func (m *mockTask) Start(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *mockTask) Delete(ctx context.Context, _ ...containerd.ProcessDeleteOpts) (*containerd.ExitStatus, error) {
	args := m.Called(ctx)
	return nil, args.Error(0)
}

func (m *mockTask) Kill(ctx context.Context, sig syscall.Signal, _ ...containerd.KillOpts) error {
	return m.Called(ctx, sig).Error(0)
}

func (m *mockTask) Wait(ctx context.Context) (<-chan containerd.ExitStatus, error) {
	args := m.Called(ctx)
	if ch := args.Get(0); ch != nil {
		return ch.(<-chan containerd.ExitStatus), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockTask) CloseIO(context.Context, ...containerd.IOCloserOpts) error { return nil }
func (m *mockTask) Resize(context.Context, uint32, uint32) error              { return nil }
func (m *mockTask) IO() cio.IO                                                { return nil }

func (m *mockTask) Pause(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *mockTask) Resume(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *mockTask) Exec(ctx context.Context, id string, pspec *specs.Process, _ cio.Creator) (containerd.Process, error) {
	args := m.Called(ctx, id, pspec)
	if p := args.Get(0); p != nil {
		return p.(containerd.Process), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockTask) Pids(context.Context) ([]containerd.ProcessInfo, error) { return nil, nil }
func (m *mockTask) Checkpoint(context.Context, ...containerd.CheckpointTaskOpts) (containerd.Image, error) {
	return nil, nil
}
func (m *mockTask) Update(context.Context, ...containerd.UpdateTaskOpts) error { return nil }
func (m *mockTask) LoadProcess(context.Context, string, cio.Attach) (containerd.Process, error) {
	return nil, nil
}
func (m *mockTask) Metrics(context.Context) (*metrictypes.Metric, error) { return nil, nil }
func (m *mockTask) Spec(context.Context) (*oci.Spec, error)              { return nil, nil }

func newTestClient(api apiClient) *containerdClient {
	return &containerdClient{client: api, namespace: "test-ns"}
}

func newMockContainer(id, image string, labels map[string]string, task *mockTask) *mockContainer {
	mc := new(mockContainer)
	mc.On("ID").Return(id)
	mc.On("Info", mock.Anything).Return(containers.Container{
		ID:     id,
		Image:  image,
		Labels: labels,
	}, nil)
	if task != nil {
		mc.On("Task", mock.Anything).Return(task, nil)
	} else {
		mc.On("Task", mock.Anything).Return(nil, errdefs.ErrNotFound)
	}
	return mc
}

func newRunningTask() *mockTask {
	t := new(mockTask)
	t.On("Status", mock.Anything).Return(containerd.Status{Status: containerd.Running}, nil)
	return t
}

func newStoppedTask() *mockTask {
	t := new(mockTask)
	t.On("Status", mock.Anything).Return(containerd.Status{Status: containerd.Stopped}, nil)
	return t
}

func testContainer(id string) *ctr.Container {
	return &ctr.Container{
		ContainerID:   id,
		ContainerName: id,
		State:         ctr.StateRunning,
	}
}

func setupLoadContainer(api *MockapiClient, id string, mc *mockContainer) {
	api.EXPECT().LoadContainer(mock.Anything, id).Return(mc, nil)
}

func newSuccessProcess() *mockProcess {
	p := new(mockProcess)
	exitCh := make(chan containerd.ExitStatus, 1)
	exitCh <- *containerd.NewExitStatus(0, time.Now(), nil)
	p.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	p.On("Start", mock.Anything).Return(nil)
	p.On("Delete", mock.Anything).Return(nil)
	return p
}

func newFailProcess(code uint32) *mockProcess {
	p := new(mockProcess)
	exitCh := make(chan containerd.ExitStatus, 1)
	exitCh <- *containerd.NewExitStatus(code, time.Now(), nil)
	p.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	p.On("Start", mock.Anything).Return(nil)
	p.On("Delete", mock.Anything).Return(nil)
	return p
}

func setupExec(task *mockTask, proc *mockProcess) {
	task.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(proc, nil)
}

func TestListContainers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		containers    func() []containerd.Container
		opts          ctr.ListOpts
		filter        func(*ctr.Container) bool
		wantLen       int
		wantState     string
		wantErr       bool
		wantErrSubstr string
		verify        func(t *testing.T, result []*ctr.Container)
	}{
		{
			name: "running_only_includes_running_container",
			containers: func() []containerd.Container {
				return []containerd.Container{newMockContainer("c1", "nginx:latest", nil, newRunningTask())}
			},
			opts:      ctr.ListOpts{All: false},
			wantLen:   1,
			wantState: ctr.StateRunning,
		},
		{
			name: "stopped_skipped_when_not_all",
			containers: func() []containerd.Container {
				return []containerd.Container{newMockContainer("c1", "nginx:latest", nil, newStoppedTask())}
			},
			opts:    ctr.ListOpts{All: false},
			wantLen: 0,
		},
		{
			name: "stopped_included_when_all",
			containers: func() []containerd.Container {
				return []containerd.Container{newMockContainer("c1", "nginx:latest", nil, newStoppedTask())}
			},
			opts:      ctr.ListOpts{All: true},
			wantLen:   1,
			wantState: ctr.StateExited,
		},
		{
			name: "no_task_skipped_when_not_all",
			containers: func() []containerd.Container {
				return []containerd.Container{newMockContainer("c1", "nginx:latest", nil, nil)}
			},
			opts:    ctr.ListOpts{All: false},
			wantLen: 0,
		},
		{
			name: "no_task_included_when_all",
			containers: func() []containerd.Container {
				return []containerd.Container{newMockContainer("c1", "nginx:latest", nil, nil)}
			},
			opts:      ctr.ListOpts{All: true},
			wantLen:   1,
			wantState: ctr.StateExited,
		},
		{
			name: "filter_func_excludes_non_matching",
			containers: func() []containerd.Container {
				return []containerd.Container{
					newMockContainer("keep", "nginx:latest", nil, newRunningTask()),
					newMockContainer("drop", "redis:latest", nil, newRunningTask()),
				}
			},
			opts:    ctr.ListOpts{All: false},
			filter:  func(c *ctr.Container) bool { return c.ID() == "keep" },
			wantLen: 1,
		},
		{
			name: "nil_filter_returns_all_running",
			containers: func() []containerd.Container {
				return []containerd.Container{
					newMockContainer("c1", "nginx:latest", nil, newRunningTask()),
					newMockContainer("c2", "redis:latest", nil, newRunningTask()),
				}
			},
			opts:    ctr.ListOpts{All: false},
			wantLen: 2,
		},
		{
			name: "empty_list_returns_empty",
			containers: func() []containerd.Container {
				return []containerd.Container{}
			},
			opts:    ctr.ListOpts{All: true},
			wantLen: 0,
		},
		{
			name: "api_error_returns_error",
			containers: func() []containerd.Container {
				return nil
			},
			opts:          ctr.ListOpts{},
			wantErr:       true,
			wantErrSubstr: "failed to list containerd containers",
		},
		{
			name: "container_conversion_maps_fields",
			containers: func() []containerd.Container {
				return []containerd.Container{
					newMockContainer("abc123", "myapp:v2", map[string]string{"env": "prod", "app": "web"}, newRunningTask()),
				}
			},
			opts:      ctr.ListOpts{All: false},
			wantLen:   1,
			wantState: ctr.StateRunning,
			verify: func(t *testing.T, result []*ctr.Container) {
				c := result[0]
				assert.Equal(t, "abc123", c.ID())
				assert.Equal(t, "abc123", c.Name())
				assert.Equal(t, "myapp:v2", c.Image)
				assert.Equal(t, "myapp:v2", c.ImageID)
				assert.Equal(t, map[string]string{"env": "prod", "app": "web"}, c.Labels)
				assert.NotNil(t, c.Networks)
			},
		},
		{
			name:          "labels_not_supported_returns_error",
			containers:    func() []containerd.Container { return nil },
			opts:          ctr.ListOpts{Labels: []string{"app=web"}},
			wantErr:       true,
			wantErrSubstr: "label filtering is not yet implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			api := NewMockapiClient(t)
			ctrs := tt.containers()
			if tt.wantErrSubstr != "label filtering is not yet implemented" {
				if tt.wantErr {
					api.EXPECT().Containers(mock.Anything).Return([]containerd.Container(nil), assert.AnError)
				} else {
					api.EXPECT().Containers(mock.Anything).Return(ctrs, nil)
				}
			}

			client := newTestClient(api)
			result, err := client.ListContainers(context.Background(), tt.filter, tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSubstr)
				assert.Nil(t, result)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.wantLen)
			if tt.wantLen > 0 && tt.wantState != "" {
				assert.Equal(t, tt.wantState, result[0].State)
			}
			if tt.verify != nil {
				tt.verify(t, result)
			}
		})
	}
}

func TestStopContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.StopContainer(context.Background(), testContainer("c1"), 10, true)
	assert.NoError(t, err)
}

func TestStopContainer_Success(t *testing.T) {
	task := newRunningTask()
	exitCh := make(chan containerd.ExitStatus, 1)
	exitCh <- containerd.ExitStatus{}
	task.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	task.On("Kill", mock.Anything, syscall.SIGTERM).Return(nil)
	task.On("Delete", mock.Anything).Return(nil)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.StopContainer(context.Background(), testContainer("c1"), 10, false)
	require.NoError(t, err)
	task.AssertCalled(t, "Kill", mock.Anything, syscall.SIGTERM)
	task.AssertCalled(t, "Delete", mock.Anything)
}

func TestStopContainer_Timeout_SIGKILL(t *testing.T) {
	task := newRunningTask()
	exitCh := make(chan containerd.ExitStatus, 1)
	task.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	task.On("Kill", mock.Anything, syscall.SIGTERM).Return(nil)
	task.On("Kill", mock.Anything, syscall.SIGKILL).Run(func(_ mock.Arguments) {
		exitCh <- containerd.ExitStatus{}
	}).Return(nil)
	task.On("Delete", mock.Anything).Return(nil)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.StopContainer(context.Background(), testContainer("c1"), 0, false)
	require.NoError(t, err)
	task.AssertCalled(t, "Kill", mock.Anything, syscall.SIGKILL)
}

func TestStopContainer_LoadError(t *testing.T) {
	api := NewMockapiClient(t)
	api.EXPECT().LoadContainer(mock.Anything, "c1").Return(nil, assert.AnError)

	client := newTestClient(api)
	err := client.StopContainer(context.Background(), testContainer("c1"), 10, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load container")
}

func TestStopContainer_HonorsStopSignal(t *testing.T) {
	task := newRunningTask()
	exitCh := make(chan containerd.ExitStatus, 1)
	exitCh <- containerd.ExitStatus{}
	task.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	task.On("Kill", mock.Anything, syscall.SIGHUP).Return(nil)
	task.On("Delete", mock.Anything).Return(nil)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	container := &ctr.Container{
		ContainerID:   "c1",
		ContainerName: "c1",
		State:         ctr.StateRunning,
		Labels:        map[string]string{"com.gaiaadm.pumba.stop-signal": "SIGHUP"},
	}

	client := newTestClient(api)
	err := client.StopContainer(context.Background(), container, 10, false)
	require.NoError(t, err)
	task.AssertCalled(t, "Kill", mock.Anything, syscall.SIGHUP)
}

func TestKillContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.KillContainer(context.Background(), testContainer("c1"), "SIGTERM", true)
	assert.NoError(t, err)
}

func TestKillContainer_Success(t *testing.T) {
	tests := []struct {
		name    string
		signal  string
		wantSig syscall.Signal
	}{
		{"sigkill_uppercase", "SIGKILL", syscall.SIGKILL},
		{"sigterm_lowercase_no_prefix", "term", syscall.SIGTERM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := newRunningTask()
			task.On("Kill", mock.Anything, tt.wantSig).Return(nil)

			mc := newMockContainer("c1", "nginx", nil, task)
			api := NewMockapiClient(t)
			setupLoadContainer(api, "c1", mc)

			client := newTestClient(api)
			err := client.KillContainer(context.Background(), testContainer("c1"), tt.signal, false)
			require.NoError(t, err)
			task.AssertCalled(t, "Kill", mock.Anything, tt.wantSig)
		})
	}
}

func TestKillContainer_UnknownSignal_ReturnsError(t *testing.T) {
	task := newRunningTask()
	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.KillContainer(context.Background(), testContainer("c1"), "NONSENSE", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signal")
}

func TestStartContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.StartContainer(context.Background(), testContainer("c1"), true)
	assert.NoError(t, err)
}

func TestStartContainer_ExistingTask(t *testing.T) {
	task := newRunningTask()
	task.On("Start", mock.Anything).Return(nil)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.StartContainer(context.Background(), testContainer("c1"), false)
	require.NoError(t, err)
	task.AssertCalled(t, "Start", mock.Anything)
}

func TestStartContainer_NewTask(t *testing.T) {
	newTask := new(mockTask)
	newTask.On("Start", mock.Anything).Return(nil)

	mc := newMockContainer("c1", "nginx", nil, nil)
	mc.On("NewTask", mock.Anything).Return(newTask, nil)

	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.StartContainer(context.Background(), testContainer("c1"), false)
	require.NoError(t, err)
	newTask.AssertCalled(t, "Start", mock.Anything)
}

func TestRestartContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.RestartContainer(context.Background(), testContainer("c1"), 10*time.Second, true)
	assert.NoError(t, err)
}

func TestRestartContainer_Success(t *testing.T) {
	stopTask := new(mockTask)
	stopTask.On("Status", mock.Anything).Return(containerd.Status{Status: containerd.Running}, nil)
	exitCh := make(chan containerd.ExitStatus, 1)
	exitCh <- containerd.ExitStatus{}
	stopTask.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	stopTask.On("Kill", mock.Anything, syscall.SIGTERM).Return(nil)
	stopTask.On("Delete", mock.Anything).Return(nil)

	startTask := new(mockTask)
	startTask.On("Start", mock.Anything).Return(nil)

	mcStop := newMockContainer("c1", "nginx", nil, stopTask)
	mcStart := newMockContainer("c1", "nginx", nil, startTask)

	api := NewMockapiClient(t)
	api.EXPECT().LoadContainer(mock.Anything, "c1").Return(mcStop, nil).Once()
	api.EXPECT().LoadContainer(mock.Anything, "c1").Return(mcStart, nil).Once()

	client := newTestClient(api)
	err := client.RestartContainer(context.Background(), testContainer("c1"), 10*time.Second, false)
	require.NoError(t, err)
}

func TestRestartContainer_StopFails(t *testing.T) {
	api := NewMockapiClient(t)
	api.EXPECT().LoadContainer(mock.Anything, "c1").Return(nil, assert.AnError)

	client := newTestClient(api)
	err := client.RestartContainer(context.Background(), testContainer("c1"), 10*time.Second, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restart: stop failed")
}

func TestRemoveContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.RemoveContainer(context.Background(), testContainer("c1"), false, false, false, true)
	assert.NoError(t, err)
}

func TestRemoveContainer_Success(t *testing.T) {
	mc := newMockContainer("c1", "nginx", nil, nil)
	mc.On("Delete", mock.Anything).Return(nil)

	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.RemoveContainer(context.Background(), testContainer("c1"), false, false, false, false)
	require.NoError(t, err)
	mc.AssertCalled(t, "Delete", mock.Anything)
}

func TestRemoveContainer_Force(t *testing.T) {
	task := newRunningTask()
	waitCh := make(chan containerd.ExitStatus, 1)
	waitCh <- containerd.ExitStatus{}
	task.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(waitCh), nil)
	task.On("Kill", mock.Anything, syscall.SIGKILL).Return(nil)
	task.On("Delete", mock.Anything).Return(nil)

	mc := newMockContainer("c1", "nginx", nil, task)
	mc.On("Delete", mock.Anything).Return(nil)

	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.RemoveContainer(context.Background(), testContainer("c1"), true, false, false, false)
	require.NoError(t, err)
	task.AssertCalled(t, "Kill", mock.Anything, syscall.SIGKILL)
	task.AssertCalled(t, "Delete", mock.Anything)
	mc.AssertCalled(t, "Delete", mock.Anything)
}

func TestRemoveContainer_LoadError(t *testing.T) {
	api := NewMockapiClient(t)
	api.EXPECT().LoadContainer(mock.Anything, "c1").Return(nil, assert.AnError)

	client := newTestClient(api)
	err := client.RemoveContainer(context.Background(), testContainer("c1"), false, false, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load container")
}

func TestPauseContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.PauseContainer(context.Background(), testContainer("c1"), true)
	assert.NoError(t, err)
}

func TestPauseContainer_Success(t *testing.T) {
	task := newRunningTask()
	task.On("Pause", mock.Anything).Return(nil)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.PauseContainer(context.Background(), testContainer("c1"), false)
	require.NoError(t, err)
	task.AssertCalled(t, "Pause", mock.Anything)
}

func TestUnpauseContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.UnpauseContainer(context.Background(), testContainer("c1"), true)
	assert.NoError(t, err)
}

func TestUnpauseContainer_Success(t *testing.T) {
	task := newRunningTask()
	task.On("Resume", mock.Anything).Return(nil)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.UnpauseContainer(context.Background(), testContainer("c1"), false)
	require.NoError(t, err)
	task.AssertCalled(t, "Resume", mock.Anything)
}

func TestStopContainerWithID_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.StopContainerWithID(context.Background(), "c1", 10*time.Second, true)
	assert.NoError(t, err)
}

func TestStopContainerWithID_Success(t *testing.T) {
	task := newRunningTask()
	exitCh := make(chan containerd.ExitStatus, 1)
	exitCh <- containerd.ExitStatus{}
	task.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	task.On("Kill", mock.Anything, syscall.SIGTERM).Return(nil)
	task.On("Delete", mock.Anything).Return(nil)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.StopContainerWithID(context.Background(), "c1", 10*time.Second, false)
	require.NoError(t, err)
}

func TestParseSignal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected syscall.Signal
	}{
		{"SIGTERM", syscall.SIGTERM},
		{"SIGKILL", syscall.SIGKILL},
		{"term", syscall.SIGTERM},
		{"kill", syscall.SIGKILL},
		{"SIGUSR1", syscall.SIGUSR1},
		{"hup", syscall.SIGHUP},
		{"SIGQUIT", syscall.SIGQUIT},
		{"SIGABRT", syscall.SIGABRT},
		{"9", syscall.SIGKILL},
		{"15", syscall.SIGTERM},
		{"1", syscall.SIGHUP},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			sig, err := parseSignal(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, sig)
		})
	}
}

func TestParseSignal_InvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantErrSubstr string
	}{
		{"unknown_name", "NONSENSE", "unknown signal"},
		{"zero_numeric", "0", "invalid signal number"},
		{"negative_numeric", "-1", "invalid signal number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSignal(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrSubstr)
		})
	}
}

func TestExecContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.ExecContainer(context.Background(), testContainer("c1"), "ls", []string{"-la"}, true)
	assert.NoError(t, err)
}

func TestExecContainer_Success(t *testing.T) {
	proc := newSuccessProcess()
	task := newRunningTask()
	setupExec(task, proc)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.ExecContainer(context.Background(), testContainer("c1"), "echo", []string{"hello"}, false)
	require.NoError(t, err)
	proc.AssertCalled(t, "Start", mock.Anything)
	proc.AssertCalled(t, "Delete", mock.Anything)
}

func TestExecContainer_NonZeroExit(t *testing.T) {
	proc := newFailProcess(1)
	task := newRunningTask()
	setupExec(task, proc)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.ExecContainer(context.Background(), testContainer("c1"), "false", nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exited with code 1")
}

func TestExecContainer_LoadError(t *testing.T) {
	api := NewMockapiClient(t)
	api.EXPECT().LoadContainer(mock.Anything, "c1").Return(nil, assert.AnError)

	client := newTestClient(api)
	err := client.ExecContainer(context.Background(), testContainer("c1"), "ls", nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load container")
}

func TestExecContainer_TaskError(t *testing.T) {
	mc := newMockContainer("c1", "nginx", nil, nil)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.ExecContainer(context.Background(), testContainer("c1"), "ls", nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get task")
}

func TestExecContainer_ExecError(t *testing.T) {
	task := newRunningTask()
	task.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.ExecContainer(context.Background(), testContainer("c1"), "ls", nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to exec")
}

func TestExecContainer_StartError(t *testing.T) {
	proc := new(mockProcess)
	exitCh := make(chan containerd.ExitStatus, 1)
	proc.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	proc.On("Start", mock.Anything).Return(assert.AnError)
	proc.On("Delete", mock.Anything).Return(nil)

	task := newRunningTask()
	setupExec(task, proc)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.ExecContainer(context.Background(), testContainer("c1"), "ls", nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start exec")
}

func TestNetemContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.NetemContainer(context.Background(), testContainer("c1"), "eth0",
		[]string{"delay", "100ms"}, nil, nil, nil, 0, "", false, true)
	assert.NoError(t, err)
}

func TestNetemContainer_Success(t *testing.T) {
	proc := newSuccessProcess()
	task := newRunningTask()
	setupExec(task, proc)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.NetemContainer(context.Background(), testContainer("c1"), "eth0",
		[]string{"delay", "100ms"}, nil, nil, nil, 0, "", false, false)
	require.NoError(t, err)
}

func TestStopNetemContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.StopNetemContainer(context.Background(), testContainer("c1"), "eth0",
		nil, nil, nil, "", false, true)
	assert.NoError(t, err)
}

func TestStopNetemContainer_Success(t *testing.T) {
	proc := newSuccessProcess()
	task := newRunningTask()
	setupExec(task, proc)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.StopNetemContainer(context.Background(), testContainer("c1"), "eth0",
		nil, nil, nil, "", false, false)
	require.NoError(t, err)
}

func TestIPTablesContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.IPTablesContainer(context.Background(), testContainer("c1"),
		[]string{"-A", "INPUT"}, []string{"-j", "DROP"}, nil, nil, nil, nil, 0, "", false, true)
	assert.NoError(t, err)
}

func TestIPTablesContainer_Success(t *testing.T) {
	proc := newSuccessProcess()
	task := newRunningTask()
	setupExec(task, proc)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.IPTablesContainer(context.Background(), testContainer("c1"),
		[]string{"-A", "INPUT"}, []string{"-j", "DROP"}, nil, nil, nil, nil, 0, "", false, false)
	require.NoError(t, err)
}

func TestIPTablesContainer_ExecError(t *testing.T) {
	task := newRunningTask()
	task.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.IPTablesContainer(context.Background(), testContainer("c1"),
		[]string{"-A", "INPUT"}, []string{"-j", "DROP"}, nil, nil, nil, nil, 0, "", false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to run iptables command")
}

func TestStopIPTablesContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	err := client.StopIPTablesContainer(context.Background(), testContainer("c1"),
		[]string{"-D", "INPUT"}, []string{"-j", "DROP"}, nil, nil, nil, nil, "", false, true)
	assert.NoError(t, err)
}

func TestStopIPTablesContainer_Success(t *testing.T) {
	proc := newSuccessProcess()
	task := newRunningTask()
	setupExec(task, proc)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	err := client.StopIPTablesContainer(context.Background(), testContainer("c1"),
		[]string{"-D", "INPUT"}, []string{"-j", "DROP"}, nil, nil, nil, nil, "", false, false)
	require.NoError(t, err)
}

func TestStressContainer_Dryrun(t *testing.T) {
	client := newTestClient(NewMockapiClient(t))
	id, outCh, errCh, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "", false, 10*time.Second, false, true)
	assert.NoError(t, err)
	assert.Equal(t, "", id)
	assert.Nil(t, outCh)
	assert.Nil(t, errCh)
}

func TestStressContainer_Success(t *testing.T) {
	proc := newSuccessProcess()
	task := newRunningTask()
	setupExec(task, proc)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	id, outCh, errCh, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "", false, 10*time.Second, false, false)
	require.NoError(t, err)
	assert.Equal(t, "c1", id)

	select {
	case out := <-outCh:
		assert.Equal(t, "c1", out)
	case e := <-errCh:
		t.Fatalf("unexpected error: %v", e)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for stress result")
	}
}

func TestStressContainer_ExecError(t *testing.T) {
	task := newRunningTask()
	task.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	id, outCh, errCh, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "", false, 10*time.Second, false, false)
	require.NoError(t, err)
	assert.Equal(t, "c1", id)

	select {
	case e := <-errCh:
		assert.Error(t, e)
	case <-outCh:
		t.Fatal("expected error, got success")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for stress error")
	}
}

func newRunningTaskWithPID(pid uint32) *mockTask {
	t := newRunningTask()
	t.pid = pid
	return t
}

func setCgroupReader(t *testing.T, pid uint32, content string) {
	t.Helper()
	setCgroupReaderFunc(t, func(p uint32) ([]byte, error) {
		if p == pid {
			return []byte(content), nil
		}
		return nil, fmt.Errorf("unexpected PID %d in test", p)
	})
}

func setCgroupReaderFunc(t *testing.T, fn func(uint32) ([]byte, error)) {
	t.Helper()
	orig := cgroupReader
	cgroupReader = fn
	t.Cleanup(func() { cgroupReader = orig })
}

func stressPrefix() any {
	return mock.MatchedBy(func(id string) bool {
		return strings.HasPrefix(id, "pumba-stress-")
	})
}

func setupStressSidecar(api *MockapiClient, stressImage string, sidecarCntr *mockContainer) {
	api.EXPECT().GetImage(mock.Anything, stressImage).Return(&stubImage{}, nil)
	api.EXPECT().NewContainer(
		mock.Anything,  // ctx
		stressPrefix(), // sidecar ID
		mock.Anything,  // WithImage
		mock.Anything,  // WithNewSnapshot
		mock.Anything,  // WithNewSpec
		mock.Anything,  // WithContainerLabels
	).Return(sidecarCntr, nil)
}

func setupStressTarget(t *testing.T, targetPID uint32, cgroupContent string) *MockapiClient {
	t.Helper()
	setCgroupReader(t, targetPID, cgroupContent)
	targetTask := newRunningTaskWithPID(targetPID)
	targetMC := newMockContainer("c1", "nginx", nil, targetTask)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", targetMC)
	return api
}

func newSidecarWithExitCode(t *testing.T, code uint32) (*mockContainer, *mockTask) {
	t.Helper()
	sidecarTask := &mockTask{}
	t.Cleanup(func() { sidecarTask.AssertExpectations(t) })
	exitCh := make(chan containerd.ExitStatus, 1)
	exitCh <- *containerd.NewExitStatus(code, time.Now(), nil)
	sidecarTask.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	sidecarTask.On("Start", mock.Anything).Return(nil)
	sidecarTask.On("Delete", mock.Anything).Return(nil)

	sidecarCntr := new(mockContainer)
	t.Cleanup(func() { sidecarCntr.AssertExpectations(t) })
	sidecarCntr.On("NewTask", mock.Anything).Return(sidecarTask, nil)
	sidecarCntr.On("Delete", mock.Anything).Return(nil)

	return sidecarCntr, sidecarTask
}

func TestResolveCgroupPath(t *testing.T) { //nolint:paralleltest // mutates package-level cgroupReader
	tests := []struct {
		name       string
		input      string
		readerErr  bool
		wantPath   string
		wantParent string
		wantErr    string
	}{
		{"v2_nested", "0::/kubepods/besteffort/pod123/ctr456\n", false, "/kubepods/besteffort/pod123/ctr456", "/kubepods/besteffort/pod123", ""},
		{"v1_memory_fallback", "11:memory:/kubepods/memory/pod123/ctr456\n12:cpu:/kubepods/cpu/pod123/ctr456\n", false, "/kubepods/memory/pod123/ctr456", "/kubepods/memory/pod123", ""},
		{"v2_overrides_v1", "11:memory:/v1path\n0::/v2path\n", false, "/v2path", "/", ""},
		{"root_cgroup_only", "0::/\n", false, "", "", "could not parse cgroup path"},
		{"single_level", "0::/scope\n", false, "/scope", "/", ""},
		{"empty_content", "", false, "", "", "could not parse cgroup path"},
		{"read_error", "", true, "", "", "failed to read cgroup"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.readerErr {
				setCgroupReaderFunc(t, func(uint32) ([]byte, error) {
					return nil, fmt.Errorf("no such file")
				})
			} else {
				setCgroupReader(t, 999, tc.input)
			}
			path, parent, err := resolveCgroupPath(999)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantPath, path)
			assert.Equal(t, tc.wantParent, parent)
		})
	}
}

func TestBuildStressSpecOpts(t *testing.T) {
	t.Parallel()
	t.Run("default_sidecar_mode", func(t *testing.T) {
		t.Parallel()
		opts := buildStressSpecOpts(nil, []string{"--cpu", "2"}, "/cgroup/path", "/cgroup", false)
		require.Len(t, opts, 3)
		spec := &specs.Spec{Process: &specs.Process{}, Linux: &specs.Linux{}}
		_ = opts[1](context.Background(), nil, nil, spec)
		assert.Equal(t, []string{"/stress-ng", "--cpu", "2"}, spec.Process.Args)
		_ = opts[2](context.Background(), nil, nil, spec)
		assert.Equal(t, "/cgroup", spec.Linux.CgroupsPath)
	})
	t.Run("inject_cgroup_mode", func(t *testing.T) {
		t.Parallel()
		opts := buildStressSpecOpts(nil, []string{"--vm", "2"}, "/cgroup/path", "/cgroup", true)
		require.Len(t, opts, 4)
		spec := &specs.Spec{Process: &specs.Process{}, Linux: &specs.Linux{}}
		_ = opts[1](context.Background(), nil, nil, spec)
		assert.Equal(t, []string{"/cg-inject", "--cgroup-path", "/cgroup/path", "--", "/stress-ng", "--vm", "2"}, spec.Process.Args)
		_ = opts[2](context.Background(), nil, nil, spec)
		require.Len(t, spec.Linux.Namespaces, 1)
		assert.Equal(t, specs.CgroupNamespace, spec.Linux.Namespaces[0].Type)
		assert.Equal(t, "/proc/1/ns/cgroup", spec.Linux.Namespaces[0].Path)
		_ = opts[3](context.Background(), nil, nil, spec)
		require.Len(t, spec.Mounts, 1)
		assert.Equal(t, "/sys/fs/cgroup", spec.Mounts[0].Source)
		assert.Equal(t, "/sys/fs/cgroup", spec.Mounts[0].Destination)
	})
}

func TestStressContainer_SidecarGetTaskError(t *testing.T) {
	api := NewMockapiClient(t)
	api.EXPECT().LoadContainer(mock.Anything, "c1").Return(nil, assert.AnError)

	client := newTestClient(api)
	_, _, _, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get target task")
}

func TestStressContainer_SidecarTargetPIDZero(t *testing.T) {
	task := newRunningTask() // pid=0 by default
	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	_, _, _, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PID 0")
}

func TestStressContainer_SidecarCgroupResolveError(t *testing.T) { //nolint:paralleltest // mutates package-level cgroupReader
	setCgroupReaderFunc(t, func(uint32) ([]byte, error) {
		return nil, fmt.Errorf("permission denied")
	})
	task := newRunningTaskWithPID(1234)
	mc := newMockContainer("c1", "nginx", nil, task)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", mc)

	client := newTestClient(api)
	_, _, _, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve cgroup")
}

func TestStressContainer_SidecarGetImageError(t *testing.T) {
	api := setupStressTarget(t, 1234, "0::/kubepods/pod123/ctr456\n")
	api.EXPECT().GetImage(mock.Anything, "stress-ng:latest").Return(nil, assert.AnError)

	client := newTestClient(api)
	_, _, _, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get stress image")
}

func TestStressContainer_SidecarCreateError(t *testing.T) {
	api := setupStressTarget(t, 1234, "0::/kubepods/pod123/ctr456\n")
	api.EXPECT().GetImage(mock.Anything, "stress-ng:latest").Return(&stubImage{}, nil)
	api.EXPECT().NewContainer(
		mock.Anything, stressPrefix(),
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, assert.AnError)

	client := newTestClient(api)
	_, _, _, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create stress sidecar")
}

func TestStressContainer_SidecarNewTaskError(t *testing.T) {
	sidecarCntr := new(mockContainer)
	t.Cleanup(func() { sidecarCntr.AssertExpectations(t) })
	sidecarCntr.On("NewTask", mock.Anything).Return(nil, assert.AnError)
	sidecarCntr.On("Delete", mock.Anything).Return(nil)

	api := setupStressTarget(t, 1234, "0::/kubepods/pod123/ctr456\n")
	setupStressSidecar(api, "stress-ng:latest", sidecarCntr)

	client := newTestClient(api)
	_, _, _, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create stress task")
}

func TestStressContainer_SidecarWaitError(t *testing.T) {
	sidecarTask := &mockTask{}
	t.Cleanup(func() { sidecarTask.AssertExpectations(t) })
	sidecarTask.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(nil), assert.AnError)
	sidecarTask.On("Delete", mock.Anything).Return(nil)
	sidecarCntr := new(mockContainer)
	t.Cleanup(func() { sidecarCntr.AssertExpectations(t) })
	sidecarCntr.On("NewTask", mock.Anything).Return(sidecarTask, nil)
	sidecarCntr.On("Delete", mock.Anything).Return(nil)

	api := setupStressTarget(t, 1234, "0::/kubepods/pod123/ctr456\n")
	setupStressSidecar(api, "stress-ng:latest", sidecarCntr)

	client := newTestClient(api)
	_, _, _, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set up stress task wait")
}

func TestStressContainer_SidecarStartError(t *testing.T) {
	exitCh := make(chan containerd.ExitStatus, 1)
	sidecarTask := &mockTask{}
	t.Cleanup(func() { sidecarTask.AssertExpectations(t) })
	sidecarTask.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil)
	sidecarTask.On("Start", mock.Anything).Return(assert.AnError)
	sidecarTask.On("Delete", mock.Anything).Return(nil)
	sidecarCntr := new(mockContainer)
	t.Cleanup(func() { sidecarCntr.AssertExpectations(t) })
	sidecarCntr.On("NewTask", mock.Anything).Return(sidecarTask, nil)
	sidecarCntr.On("Delete", mock.Anything).Return(nil)

	api := setupStressTarget(t, 1234, "0::/kubepods/pod123/ctr456\n")
	setupStressSidecar(api, "stress-ng:latest", sidecarCntr)

	client := newTestClient(api)
	_, _, _, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start stress task")
}

func TestStressContainer_SidecarExitCodes(t *testing.T) { //nolint:paralleltest // mutates package-level cgroupReader
	tests := []struct {
		name         string
		cgroupData   string
		stressors    []string
		injectCgroup bool
		exitCode     uint32
		wantErrMsg   string
	}{
		{
			name:       "default_sidecar_success",
			cgroupData: "0::/kubepods/pod123/ctr456\n",
			stressors:  []string{"--cpu", "1"},
			exitCode:   0,
		},
		{
			name:         "inject_cgroup_success",
			cgroupData:   "0::/system.slice/nerdctl-abc123.scope\n",
			stressors:    []string{"--vm", "2"},
			injectCgroup: true,
			exitCode:     0,
		},
		{
			name:       "non_zero_exit",
			cgroupData: "0::/kubepods/pod123/ctr456\n",
			stressors:  []string{"--cpu", "1"},
			exitCode:   1,
			wantErrMsg: "exited with code 1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sidecarCntr, _ := newSidecarWithExitCode(t, tc.exitCode)
			api := setupStressTarget(t, 1234, tc.cgroupData)
			setupStressSidecar(api, "stress-ng:latest", sidecarCntr)

			client := newTestClient(api)
			id, outCh, errCh, err := client.StressContainer(context.Background(), testContainer("c1"),
				tc.stressors, "stress-ng:latest", false, 10*time.Second, tc.injectCgroup, false)
			require.NoError(t, err)
			assert.Contains(t, id, "pumba-stress-")

			select {
			case out := <-outCh:
				if tc.wantErrMsg != "" {
					t.Fatalf("expected error, got success: %s", out)
				}
				assert.Equal(t, id, out)
			case e := <-errCh:
				if tc.wantErrMsg == "" {
					t.Fatalf("unexpected error: %v", e)
				}
				assert.Contains(t, e.Error(), tc.wantErrMsg)
			case <-time.After(5 * time.Second):
				t.Fatal("timeout")
			}
		})
	}
}

func TestStressContainer_SidecarContextCanceled(t *testing.T) { //nolint:paralleltest // mutates package-level cgroupReader
	sidecarTask := &mockTask{}
	t.Cleanup(func() { sidecarTask.AssertExpectations(t) })
	blockCh := make(chan containerd.ExitStatus, 1)
	sidecarTask.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(blockCh), nil)
	sidecarTask.On("Start", mock.Anything).Return(nil)
	sidecarTask.On("Kill", mock.Anything, syscall.SIGKILL).Run(func(_ mock.Arguments) {
		blockCh <- *containerd.NewExitStatus(137, time.Now(), nil) //nolint:mnd
	}).Return(nil)
	sidecarTask.On("Delete", mock.Anything).Return(nil)

	sidecarCntr := new(mockContainer)
	t.Cleanup(func() { sidecarCntr.AssertExpectations(t) })
	sidecarCntr.On("NewTask", mock.Anything).Return(sidecarTask, nil)
	sidecarCntr.On("Delete", mock.Anything).Return(nil)

	api := setupStressTarget(t, 1234, "0::/kubepods/pod123/ctr456\n")
	setupStressSidecar(api, "stress-ng:latest", sidecarCntr)

	ctx, cancel := context.WithCancel(context.Background())
	client := newTestClient(api)
	_, _, errCh, err := client.StressContainer(ctx, testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.NoError(t, err)

	cancel()

	select {
	case e := <-errCh:
		require.Error(t, e)
		assert.Contains(t, e.Error(), "exited with code 137")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for stress sidecar to handle context cancellation")
	}
}

func TestStressContainer_SidecarWaitChClosed(t *testing.T) { //nolint:paralleltest // mutates package-level cgroupReader
	sidecarTask := &mockTask{}
	t.Cleanup(func() { sidecarTask.AssertExpectations(t) })
	closedCh := make(chan containerd.ExitStatus)
	close(closedCh)
	sidecarTask.On("Wait", mock.Anything).Return((<-chan containerd.ExitStatus)(closedCh), nil)
	sidecarTask.On("Start", mock.Anything).Return(nil)
	sidecarTask.On("Delete", mock.Anything).Return(nil)

	sidecarCntr := new(mockContainer)
	t.Cleanup(func() { sidecarCntr.AssertExpectations(t) })
	sidecarCntr.On("NewTask", mock.Anything).Return(sidecarTask, nil)
	sidecarCntr.On("Delete", mock.Anything).Return(nil)

	api := setupStressTarget(t, 1234, "0::/kubepods/pod123/ctr456\n")
	setupStressSidecar(api, "stress-ng:latest", sidecarCntr)

	client := newTestClient(api)
	_, _, errCh, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)
	require.NoError(t, err)

	select {
	case e := <-errCh:
		require.Error(t, e)
		assert.Contains(t, e.Error(), "wait channel closed unexpectedly")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for closed-channel error")
	}
}

func TestStressContainer_SidecarPullImageError(t *testing.T) {
	api := setupStressTarget(t, 1234, "0::/kubepods/pod123/ctr456\n")
	api.EXPECT().Pull(mock.Anything, "stress-ng:latest", mock.Anything).Return(nil, assert.AnError)

	client := newTestClient(api)
	_, _, _, err := client.StressContainer(context.Background(), testContainer("c1"),
		[]string{"--cpu", "1"}, "stress-ng:latest", true, 10*time.Second, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to pull image")
}

func TestBuildNetemCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		iface    string
		cmds     []string
		ips      []*net.IPNet
		sports   []string
		dports   []string
		wantCmds [][]string
	}{
		{
			name:     "basic_delay",
			iface:    "eth0",
			cmds:     []string{"delay", "100ms"},
			wantCmds: [][]string{{"qdisc", "add", "dev", "eth0", "root", "netem", "delay", "100ms"}},
		},
		{
			name:     "multiple_commands",
			iface:    "eth0",
			cmds:     []string{"delay", "100ms", "loss", "10%"},
			wantCmds: [][]string{{"qdisc", "add", "dev", "eth0", "root", "netem", "delay", "100ms", "loss", "10%"}},
		},
		{
			name:  "ip_filtering_creates_prio_qdisc",
			iface: "eth0",
			cmds:  []string{"delay", "100ms"},
			ips:   func() []*net.IPNet { _, n, _ := net.ParseCIDR("10.0.0.0/8"); return []*net.IPNet{n} }(),
			wantCmds: [][]string{
				{"qdisc", "add", "dev", "eth0", "root", "handle", "1:", "prio"},
				{"qdisc", "add", "dev", "eth0", "parent", "1:1", "handle", "10:", "sfq"},
				{"qdisc", "add", "dev", "eth0", "parent", "1:2", "handle", "20:", "sfq"},
				{"qdisc", "add", "dev", "eth0", "parent", "1:3", "handle", "30:", "netem", "delay", "100ms"},
				{"filter", "add", "dev", "eth0", "protocol", "ip", "parent", "1:0", "prio", "1", "u32", "match", "ip", "dst", "10.0.0.0/8", "flowid", "1:3"},
			},
		},
		{
			name:   "sport_filtering",
			iface:  "eth0",
			cmds:   []string{"delay", "100ms"},
			sports: []string{"80"},
			wantCmds: [][]string{
				{"qdisc", "add", "dev", "eth0", "root", "handle", "1:", "prio"},
				{"qdisc", "add", "dev", "eth0", "parent", "1:1", "handle", "10:", "sfq"},
				{"qdisc", "add", "dev", "eth0", "parent", "1:2", "handle", "20:", "sfq"},
				{"qdisc", "add", "dev", "eth0", "parent", "1:3", "handle", "30:", "netem", "delay", "100ms"},
				{"filter", "add", "dev", "eth0", "protocol", "ip", "parent", "1:0", "prio", "1", "u32", "match", "ip", "sport", "80", "0xffff", "flowid", "1:3"},
			},
		},
		{
			name:   "dport_filtering",
			iface:  "eth0",
			cmds:   []string{"delay", "100ms"},
			dports: []string{"443"},
			wantCmds: [][]string{
				{"qdisc", "add", "dev", "eth0", "root", "handle", "1:", "prio"},
				{"qdisc", "add", "dev", "eth0", "parent", "1:1", "handle", "10:", "sfq"},
				{"qdisc", "add", "dev", "eth0", "parent", "1:2", "handle", "20:", "sfq"},
				{"qdisc", "add", "dev", "eth0", "parent", "1:3", "handle", "30:", "netem", "delay", "100ms"},
				{"filter", "add", "dev", "eth0", "protocol", "ip", "parent", "1:0", "prio", "1", "u32", "match", "ip", "dport", "443", "0xffff", "flowid", "1:3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmds := buildNetemCommands(tt.iface, tt.cmds, tt.ips, tt.sports, tt.dports)
			assert.Equal(t, tt.wantCmds, cmds)
		})
	}
}

func TestBuildStopNetemCommands(t *testing.T) {
	t.Parallel()

	t.Run("without_filters", func(t *testing.T) {
		t.Parallel()
		cmds := buildStopNetemCommands("eth0", false)
		assert.Equal(t, [][]string{{"qdisc", "del", "dev", "eth0", "root"}}, cmds)
	})

	t.Run("with_filters", func(t *testing.T) {
		t.Parallel()
		cmds := buildStopNetemCommands("eth0", true)
		assert.Len(t, cmds, 4)
		assert.Equal(t, []string{"qdisc", "del", "dev", "eth0", "parent", "1:1", "handle", "10:"}, cmds[0])
		assert.Equal(t, []string{"qdisc", "del", "dev", "eth0", "root", "handle", "1:", "prio"}, cmds[3])
	})
}

func TestBuildIPTablesCommands(t *testing.T) {
	t.Parallel()

	_, srcNet, _ := net.ParseCIDR("10.0.0.0/8")
	_, dstNet, _ := net.ParseCIDR("192.168.1.0/24")

	tests := []struct {
		name     string
		flags    []string
		target   []string
		srcIPs   []*net.IPNet
		dstIPs   []*net.IPNet
		srcPorts []string
		dstPorts []string
		want     [][]string
	}{
		{
			name:   "basic_rule",
			flags:  []string{"-A", "INPUT"},
			target: []string{"-j", "DROP"},
			want:   [][]string{{"-A", "INPUT", "-j", "DROP"}},
		},
		{
			name:   "with_src_and_dst_ips",
			flags:  []string{"-A", "INPUT"},
			target: []string{"-j", "DROP"},
			srcIPs: []*net.IPNet{srcNet},
			dstIPs: []*net.IPNet{dstNet},
			want: [][]string{
				{"-A", "INPUT", "-s", "10.0.0.0/8", "-j", "DROP"},
				{"-A", "INPUT", "-d", "192.168.1.0/24", "-j", "DROP"},
			},
		},
		{
			name:     "with_src_and_dst_ports",
			flags:    []string{"-A", "INPUT"},
			target:   []string{"-j", "DROP"},
			srcPorts: []string{"80", "443"},
			dstPorts: []string{"8080"},
			want: [][]string{
				{"-A", "INPUT", "--sport", "80", "-j", "DROP"},
				{"-A", "INPUT", "--sport", "443", "-j", "DROP"},
				{"-A", "INPUT", "--dport", "8080", "-j", "DROP"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmds := buildIPTablesCommands(tt.flags, tt.target, tt.srcIPs, tt.dstIPs, tt.srcPorts, tt.dstPorts)
			assert.Equal(t, tt.want, cmds)
		})
	}
}
