package containerd

import (
	"context"
	"syscall"
	"testing"

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

	metrictypes "github.com/containerd/containerd/api/types"
)

// --- mocks ---

type mockAPIClient struct {
	mock.Mock
}

func (m *mockAPIClient) Containers(ctx context.Context, filters ...string) ([]containerd.Container, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]containerd.Container), args.Error(1)
}

func (m *mockAPIClient) LoadContainer(ctx context.Context, id string) (containerd.Container, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(containerd.Container), args.Error(1)
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

func (m *mockContainer) Delete(context.Context, ...containerd.DeleteOpts) error { return nil }
func (m *mockContainer) NewTask(context.Context, cio.Creator, ...containerd.NewTaskOpts) (containerd.Task, error) {
	return nil, nil
}
func (m *mockContainer) Spec(context.Context) (*oci.Spec, error)                                     { return nil, nil }
func (m *mockContainer) Image(context.Context) (containerd.Image, error)                             { return nil, nil }
func (m *mockContainer) Labels(context.Context) (map[string]string, error)                           { return nil, nil }
func (m *mockContainer) SetLabels(context.Context, map[string]string) (map[string]string, error)     { return nil, nil }
func (m *mockContainer) Extensions(context.Context) (map[string]types.Any, error)                    { return nil, nil }
func (m *mockContainer) Update(context.Context, ...containerd.UpdateContainerOpts) error             { return nil }
func (m *mockContainer) Checkpoint(context.Context, string, ...containerd.CheckpointOpts) (containerd.Image, error) {
	return nil, nil
}
func (m *mockContainer) Restore(context.Context, cio.Creator, string) (int, error) { return 0, nil }

type mockTask struct {
	mock.Mock
}

func (m *mockTask) Status(ctx context.Context) (containerd.Status, error) {
	args := m.Called(ctx)
	return args.Get(0).(containerd.Status), args.Error(1)
}

func (m *mockTask) ID() string                                                        { return "" }
func (m *mockTask) Pid() uint32                                                       { return 0 }
func (m *mockTask) Start(context.Context) error                                       { return nil }
func (m *mockTask) Delete(context.Context, ...containerd.ProcessDeleteOpts) (*containerd.ExitStatus, error) {
	return nil, nil
}
func (m *mockTask) Kill(context.Context, syscall.Signal, ...containerd.KillOpts) error { return nil }
func (m *mockTask) Wait(context.Context) (<-chan containerd.ExitStatus, error)         { return nil, nil }
func (m *mockTask) CloseIO(context.Context, ...containerd.IOCloserOpts) error          { return nil }
func (m *mockTask) Resize(context.Context, uint32, uint32) error                       { return nil }
func (m *mockTask) IO() cio.IO                                                        { return nil }
func (m *mockTask) Pause(context.Context) error                                       { return nil }
func (m *mockTask) Resume(context.Context) error                                      { return nil }
func (m *mockTask) Exec(context.Context, string, *specs.Process, cio.Creator) (containerd.Process, error) {
	return nil, nil
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

// --- helpers ---

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

// --- tests ---

func TestListContainers_RunningOnly(t *testing.T) {
	task := newRunningTask()
	mc := newMockContainer("c1", "nginx:latest", nil, task)

	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container{mc}, nil)

	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), nil, ctr.ListOpts{All: false})

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "c1", result[0].ID())
	assert.Equal(t, ctr.StateRunning, result[0].State)
	api.AssertExpectations(t)
}

func TestListContainers_SkipsStoppedWhenNotAll(t *testing.T) {
	task := newStoppedTask()
	mc := newMockContainer("c1", "nginx:latest", nil, task)

	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container{mc}, nil)

	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), nil, ctr.ListOpts{All: false})

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestListContainers_IncludesStoppedWhenAll(t *testing.T) {
	task := newStoppedTask()
	mc := newMockContainer("c1", "nginx:latest", nil, task)

	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container{mc}, nil)

	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), nil, ctr.ListOpts{All: true})

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, ctr.StateExited, result[0].State)
}

func TestListContainers_NoTask_SkippedWhenNotAll(t *testing.T) {
	mc := newMockContainer("c1", "nginx:latest", nil, nil)

	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container{mc}, nil)

	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), nil, ctr.ListOpts{All: false})

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestListContainers_NoTask_IncludedWhenAll(t *testing.T) {
	mc := newMockContainer("c1", "nginx:latest", nil, nil)

	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container{mc}, nil)

	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), nil, ctr.ListOpts{All: true})

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, ctr.StateExited, result[0].State)
}

func TestListContainers_FilterFunc(t *testing.T) {
	t1 := newRunningTask()
	t2 := newRunningTask()
	mc1 := newMockContainer("keep", "nginx:latest", nil, t1)
	mc2 := newMockContainer("drop", "redis:latest", nil, t2)

	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container{mc1, mc2}, nil)

	filterKeep := func(c *ctr.Container) bool { return c.ID() == "keep" }
	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), filterKeep, ctr.ListOpts{All: false})

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "keep", result[0].ID())
}

func TestListContainers_NilFilter_ReturnsAll(t *testing.T) {
	t1 := newRunningTask()
	t2 := newRunningTask()
	mc1 := newMockContainer("c1", "nginx:latest", nil, t1)
	mc2 := newMockContainer("c2", "redis:latest", nil, t2)

	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container{mc1, mc2}, nil)

	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), nil, ctr.ListOpts{All: false})

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestListContainers_APIError(t *testing.T) {
	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container(nil), assert.AnError)

	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), nil, ctr.ListOpts{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list containerd containers")
}

func TestListContainers_ContainerConversion(t *testing.T) {
	task := newRunningTask()
	labels := map[string]string{"env": "prod", "app": "web"}
	mc := newMockContainer("abc123", "myapp:v2", labels, task)

	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container{mc}, nil)

	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), nil, ctr.ListOpts{All: false})

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	c := result[0]
	assert.Equal(t, "abc123", c.ID())
	assert.Equal(t, "abc123", c.Name())
	assert.Equal(t, "myapp:v2", c.Image)
	assert.Equal(t, "myapp:v2", c.ImageID)
	assert.Equal(t, ctr.StateRunning, c.State)
	assert.Equal(t, labels, c.Labels)
	assert.NotNil(t, c.Networks)
}

func TestListContainers_EmptyList(t *testing.T) {
	api := new(mockAPIClient)
	api.On("Containers", mock.Anything, mock.Anything).Return([]containerd.Container{}, nil)

	client := newTestClient(api)
	result, err := client.ListContainers(context.Background(), nil, ctr.ListOpts{All: true})

	assert.NoError(t, err)
	assert.Empty(t, result)
}
