package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStopContainer_DefaultSuccess(t *testing.T) {
	c := NewTestContainer(AsMap("ID", "abc123", "Name", "foo", "Image", "abc123"))
	notRunningContainer := DetailsResponse(AsMap("Running", false))

	api := NewMockEngine(t)
	api.EXPECT().ContainerKill(mock.Anything, "abc123", "SIGTERM").Return(nil)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(notRunningContainer, nil).Once()

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_DryRun(t *testing.T) {
	c := NewTestContainer(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))

	api := NewMockEngine(t)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, true)

	assert.NoError(t, err)
}

func TestKillContainer_DefaultSuccess(t *testing.T) {
	c := NewTestContainer(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))

	api := NewMockEngine(t)
	api.EXPECT().ContainerKill(mock.Anything, "abc123", "SIGTERM").Return(nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.KillContainer(context.TODO(), c, "SIGTERM", false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestKillContainer_DryRun(t *testing.T) {
	c := NewTestContainer(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))

	api := NewMockEngine(t)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.KillContainer(context.TODO(), c, "SIGTERM", true)

	assert.NoError(t, err)
}

func TestStopContainer_CustomSignalSuccess(t *testing.T) {
	c := NewTestContainer(AsMap(
		"ID", "abc123",
		"Name", "foo",
		"Labels", map[string]string{"com.gaiaadm.pumba.stop-signal": "SIGUSR1"},
	))

	notRunningContainer := DetailsResponse(AsMap("Running", false))

	api := NewMockEngine(t)
	api.EXPECT().ContainerKill(mock.Anything, "abc123", "SIGUSR1").Return(nil)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(notRunningContainer, nil).Once()

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_KillContainerError(t *testing.T) {
	c := NewTestContainer(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))

	api := NewMockEngine(t)
	api.EXPECT().ContainerKill(mock.Anything, "abc123", "SIGTERM").Return(errors.New("oops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.Error(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_2ndKillContainerError(t *testing.T) {
	c := NewTestContainer(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))

	api := NewMockEngine(t)
	api.EXPECT().ContainerKill(mock.Anything, "abc123", "SIGTERM").Return(nil)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(DetailsResponse(AsMap()), errors.New("dangit"))
	api.EXPECT().ContainerKill(mock.Anything, "abc123", "SIGKILL").Return(errors.New("whoops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "failed to kill container: whoops")
	api.AssertExpectations(t)
}

func TestRemoveContainer_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	engineClient := NewMockEngine(t)
	removeOpts := ctypes.RemoveOptions{RemoveVolumes: true, RemoveLinks: true, Force: true}
	engineClient.EXPECT().ContainerRemove(mock.Anything, "abc123", removeOpts).Return(nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.RemoveContainer(context.TODO(), c, ctr.RemoveOpts{Force: true, Links: true, Volumes: true})

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestRemoveContainer_DryRun(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	engineClient := NewMockEngine(t)

	client := dockerClient{containerAPI: engineClient}
	err := client.RemoveContainer(context.TODO(), c, ctr.RemoveOpts{Force: true, Links: true, Volumes: true, DryRun: true})

	assert.NoError(t, err)
}

func TestPauseContainer_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}
	engineClient := NewMockEngine(t)
	engineClient.EXPECT().ContainerPause(mock.Anything, "abc123").Return(nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.PauseContainer(context.TODO(), c, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestUnpauseContainer_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}
	engineClient := NewMockEngine(t)
	engineClient.EXPECT().ContainerUnpause(mock.Anything, "abc123").Return(nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.UnpauseContainer(context.TODO(), c, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestPauseContainer_DryRun(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	engineClient := NewMockEngine(t)
	client := dockerClient{containerAPI: engineClient}
	err := client.PauseContainer(context.TODO(), c, true)

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerPause", mock.Anything, "abc123")
}

func TestPauseContainer_PauseError(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}
	engineClient := NewMockEngine(t)
	engineClient.EXPECT().ContainerPause(mock.Anything, "abc123").Return(errors.New("pause"))

	client := dockerClient{containerAPI: engineClient}
	err := client.PauseContainer(context.TODO(), c, false)

	assert.Error(t, err)
	engineClient.AssertExpectations(t)
}

func TestPauseContainer_UnpauseError(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}
	engineClient := NewMockEngine(t)
	engineClient.EXPECT().ContainerUnpause(mock.Anything, "abc123").Return(errors.New("unpause"))

	client := dockerClient{containerAPI: engineClient}
	err := client.UnpauseContainer(context.TODO(), c, false)

	assert.Error(t, err)
	engineClient.AssertExpectations(t)
}

func TestStartContainer_DefaultSuccess(t *testing.T) {
	c := NewTestContainer(AsMap("ID", "abc123", "Name", "foo", "Image", "abc123"))

	api := NewMockEngine(t)
	api.EXPECT().ContainerStart(mock.Anything, "abc123", ctypes.StartOptions{}).Return(nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StartContainer(context.TODO(), c, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartContainer_DryRun(t *testing.T) {
	c := NewTestContainer(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))

	api := NewMockEngine(t)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StartContainer(context.TODO(), c, true)

	assert.NoError(t, err)
}

func TestRestartContainer(t *testing.T) {
	type args struct {
		ctx     context.Context
		c       *ctr.Container
		timeout time.Duration
		dryrun  bool
	}
	tests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, context.Context, string, time.Duration, bool)
		wantErr bool
	}{
		{
			name: "restart container successfully",
			args: args{
				ctx:     context.TODO(),
				c:       &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				timeout: 10 * time.Second,
				dryrun:  false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, id string, timeout time.Duration, dryrun bool) {
				_ = timeout // Used for documentating the function signature
				api.EXPECT().ContainerRestart(mock.Anything, id, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "restart container dry run",
			args: args{
				ctx:     context.TODO(),
				c:       &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				timeout: 5 * time.Second,
				dryrun:  true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, id string, timeout time.Duration, dryrun bool) {
				// Should not be called in dry run mode
			},
			wantErr: false,
		},
		{
			name: "restart container error",
			args: args{
				ctx:     context.TODO(),
				c:       &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				timeout: 10 * time.Second,
				dryrun:  false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, id string, timeout time.Duration, dryrun bool) {
				_ = timeout // Used for documentating the function signature
				api.EXPECT().ContainerRestart(mock.Anything, id, mock.Anything).Return(errors.New("restart error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			tt.mockSet(api, tt.args.ctx, tt.args.c.ID(), tt.args.timeout, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.RestartContainer(tt.args.ctx, tt.args.c, tt.args.timeout, tt.args.dryrun)

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.RestartContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

func TestStopContainerWithID(t *testing.T) {
	type args struct {
		ctx         context.Context
		containerID string
		timeout     time.Duration
		dryrun      bool
	}
	tests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, context.Context, string, time.Duration, bool)
		wantErr bool
	}{
		{
			name: "stop container by ID successfully",
			args: args{
				ctx:         context.TODO(),
				containerID: "abc123",
				timeout:     10 * time.Second,
				dryrun:      false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, id string, timeout time.Duration, dryrun bool) {
				_ = timeout // Used for documentating the function signature
				api.EXPECT().ContainerStop(mock.Anything, id, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "stop container by ID dry run",
			args: args{
				ctx:         context.TODO(),
				containerID: "abc123",
				timeout:     5 * time.Second,
				dryrun:      true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, id string, timeout time.Duration, dryrun bool) {
				// Should not be called in dry run mode
			},
			wantErr: false,
		},
		{
			name: "stop container by ID error",
			args: args{
				ctx:         context.TODO(),
				containerID: "abc123",
				timeout:     10 * time.Second,
				dryrun:      false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, id string, timeout time.Duration, dryrun bool) {
				_ = timeout // Used for documentating the function signature
				api.EXPECT().ContainerStop(mock.Anything, id, mock.Anything).Return(errors.New("stop error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			tt.mockSet(api, tt.args.ctx, tt.args.containerID, tt.args.timeout, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.StopContainerWithID(tt.args.ctx, tt.args.containerID, tt.args.timeout, tt.args.dryrun)

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.StopContainerWithID() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

func TestStartContainer(t *testing.T) {
	type args struct {
		ctx    context.Context
		c      *ctr.Container
		dryrun bool
	}
	tests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, context.Context, string, bool)
		wantErr bool
	}{
		{
			name: "start container successfully",
			args: args{
				ctx:    context.TODO(),
				c:      &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				dryrun: false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, id string, dryrun bool) {
				api.EXPECT().ContainerStart(ctx, id, ctypes.StartOptions{}).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "start container dry run",
			args: args{
				ctx:    context.TODO(),
				c:      &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				dryrun: true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, id string, dryrun bool) {
				// Should not be called in dry run mode
			},
			wantErr: false,
		},
		{
			name: "start container error",
			args: args{
				ctx:    context.TODO(),
				c:      &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				dryrun: false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, id string, dryrun bool) {
				api.EXPECT().ContainerStart(ctx, id, ctypes.StartOptions{}).Return(errors.New("start error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			tt.mockSet(api, tt.args.ctx, tt.args.c.ID(), tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.StartContainer(tt.args.ctx, tt.args.c, tt.args.dryrun)

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.StartContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

func TestWaitForStop(t *testing.T) {
	c := &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"}

	tests := []struct {
		name      string
		waitTime  int
		mockSet   func(*mocks.APIClient)
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "container stops on first inspect",
			waitTime: 5,
			mockSet: func(api *mocks.APIClient) {
				api.EXPECT().ContainerInspect(mock.Anything, "abc123").
					Return(DetailsResponse(AsMap("Running", false)), nil).Once()
			},
		},
		{
			name:     "inspect error",
			waitTime: 5,
			mockSet: func(api *mocks.APIClient) {
				api.EXPECT().ContainerInspect(mock.Anything, "abc123").
					Return(DetailsResponse(AsMap()), errors.New("inspect error")).Once()
			},
			wantErr:   true,
			errSubstr: "failed to inspect container",
		},
		{
			name:     "timeout when container never stops",
			waitTime: 0,
			mockSet: func(api *mocks.APIClient) {
				api.EXPECT().ContainerInspect(mock.Anything, "abc123").
					Return(DetailsResponse(AsMap("Running", true)), nil).Maybe()
			},
			wantErr:   true,
			errSubstr: "timeout on waiting to stop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			tt.mockSet(api)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.waitForStop(context.Background(), c, tt.waitTime)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPauseUnpauseContainer(t *testing.T) {
	type args struct {
		ctx    context.Context
		c      *ctr.Container
		dryrun bool
	}

	pauseTests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, context.Context, *ctr.Container, bool, bool)
		wantErr bool
		isPause bool
	}{
		{
			name: "pause container successfully",
			args: args{
				ctx:    context.TODO(),
				c:      &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				dryrun: false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, dryrun bool, isPause bool) {
				api.EXPECT().ContainerPause(ctx, c.ID()).Return(nil)
			},
			wantErr: false,
			isPause: true,
		},
		{
			name: "pause container dry run",
			args: args{
				ctx:    context.TODO(),
				c:      &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				dryrun: true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, dryrun bool, isPause bool) {
			},
			wantErr: false,
			isPause: true,
		},
		{
			name: "pause container error",
			args: args{
				ctx:    context.TODO(),
				c:      &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				dryrun: false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, dryrun bool, isPause bool) {
				api.EXPECT().ContainerPause(ctx, c.ID()).Return(errors.New("pause error"))
			},
			wantErr: true,
			isPause: true,
		},
		{
			name: "unpause container successfully",
			args: args{
				ctx:    context.TODO(),
				c:      &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				dryrun: false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, dryrun bool, isPause bool) {
				api.EXPECT().ContainerUnpause(ctx, c.ID()).Return(nil)
			},
			wantErr: false,
			isPause: false,
		},
		{
			name: "unpause container dry run",
			args: args{
				ctx:    context.TODO(),
				c:      &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				dryrun: true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, dryrun bool, isPause bool) {
			},
			wantErr: false,
			isPause: false,
		},
		{
			name: "unpause container error",
			args: args{
				ctx:    context.TODO(),
				c:      &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				dryrun: false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, dryrun bool, isPause bool) {
				api.EXPECT().ContainerUnpause(ctx, c.ID()).Return(errors.New("unpause error"))
			},
			wantErr: true,
			isPause: false,
		},
	}

	for _, tt := range pauseTests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			tt.mockSet(api, tt.args.ctx, tt.args.c, tt.args.dryrun, tt.isPause)

			client := dockerClient{containerAPI: api, imageAPI: api}

			var err error
			if tt.isPause {
				err = client.PauseContainer(tt.args.ctx, tt.args.c, tt.args.dryrun)
			} else {
				err = client.UnpauseContainer(tt.args.ctx, tt.args.c, tt.args.dryrun)
			}

			if (err != nil) != tt.wantErr {
				methodName := "PauseContainer"
				if !tt.isPause {
					methodName = "UnpauseContainer"
				}
				t.Errorf("dockerClient.%s() error = %v, wantErr %v", methodName, err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

func TestRemoveContainer(t *testing.T) {
	type args struct {
		ctx     context.Context
		c       *ctr.Container
		force   bool
		links   bool
		volumes bool
		dryrun  bool
	}
	tests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, context.Context, *ctr.Container, bool, bool, bool, bool)
		wantErr bool
	}{
		{
			name: "remove container successfully",
			args: args{
				ctx:     context.TODO(),
				c:       &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				force:   true,
				links:   false,
				volumes: true,
				dryrun:  false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, force, links, volumes, dryrun bool) {
				api.EXPECT().ContainerRemove(ctx, c.ID(), ctypes.RemoveOptions{
					RemoveVolumes: volumes,
					RemoveLinks:   links,
					Force:         force,
				}).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "remove container with links",
			args: args{
				ctx:     context.TODO(),
				c:       &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				force:   true,
				links:   true,
				volumes: true,
				dryrun:  false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, force, links, volumes, dryrun bool) {
				api.EXPECT().ContainerRemove(ctx, c.ID(), ctypes.RemoveOptions{
					RemoveVolumes: volumes,
					RemoveLinks:   links,
					Force:         force,
				}).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "remove container dry run",
			args: args{
				ctx:     context.TODO(),
				c:       &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				force:   true,
				links:   false,
				volumes: true,
				dryrun:  true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, force, links, volumes, dryrun bool) {
			},
			wantErr: false,
		},
		{
			name: "remove container error",
			args: args{
				ctx:     context.TODO(),
				c:       &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				force:   true,
				links:   false,
				volumes: true,
				dryrun:  false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, force, links, volumes, dryrun bool) {
				api.EXPECT().ContainerRemove(ctx, c.ID(), ctypes.RemoveOptions{
					RemoveVolumes: volumes,
					RemoveLinks:   links,
					Force:         force,
				}).Return(errors.New("remove error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			tt.mockSet(api, tt.args.ctx, tt.args.c, tt.args.force, tt.args.links, tt.args.volumes, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.RemoveContainer(tt.args.ctx, tt.args.c, ctr.RemoveOpts{
				Force:   tt.args.force,
				Links:   tt.args.links,
				Volumes: tt.args.volumes,
				DryRun:  tt.args.dryrun,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.RemoveContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}
