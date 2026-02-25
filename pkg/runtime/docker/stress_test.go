package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStressContainerBasic(t *testing.T) {
	type args struct {
		ctx       context.Context
		c         *ctr.Container
		stressors []string
		image     string
		pull      bool
		duration  time.Duration
		dryrun    bool
	}
	tests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, *ctr.Container, []string, string, bool, time.Duration, bool)
		wantErr bool
	}{
		{
			name: "stress container dry run",
			args: args{
				ctx: context.TODO(),
				c: &ctr.Container{
					ContainerID:   "abc123",
					ContainerName: "test-container",
					Labels:        map[string]string{},
					Networks:      map[string]ctr.NetworkLink{},
				},
				stressors: []string{"--cpu", "2", "--timeout", "30s"},
				image:     "alexeiled/stress-ng:latest",
				duration:  30 * time.Second,
				dryrun:    true,
			},
			mockSet: func(api *mocks.APIClient, c *ctr.Container, stressors []string, image string, pull bool, duration time.Duration, dryrun bool) {
			},
			wantErr: false,
		},
		{
			name: "stress container image pull failure",
			args: args{
				ctx: context.TODO(),
				c: &ctr.Container{
					ContainerID:   "abc123",
					ContainerName: "test-container",
					Labels:        map[string]string{},
					Networks:      map[string]ctr.NetworkLink{},
				},
				stressors: []string{"--cpu", "2", "--timeout", "30s"},
				image:     "alexeiled/stress-ng:latest",
				pull:      true,
				duration:  30 * time.Second,
				dryrun:    false,
			},
			mockSet: func(api *mocks.APIClient, c *ctr.Container, stressors []string, image string, pull bool, duration time.Duration, dryrun bool) {
				api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(DetailsResponse(AsMap("ID", "abc123")), nil)
				api.EXPECT().ImagePull(mock.Anything, image, mock.Anything).Return(nil, errors.New("pull error")).Once()
			},
			wantErr: true,
		},
		{
			name: "stress container creation failure",
			args: args{
				ctx: context.TODO(),
				c: &ctr.Container{
					ContainerID:   "abc123",
					ContainerName: "test-container",
					Labels:        map[string]string{},
					Networks:      map[string]ctr.NetworkLink{},
				},
				stressors: []string{"--cpu", "2", "--timeout", "30s"},
				image:     "alexeiled/stress-ng:latest",
				pull:      false,
				duration:  30 * time.Second,
				dryrun:    false,
			},
			mockSet: func(api *mocks.APIClient, c *ctr.Container, stressors []string, image string, pull bool, duration time.Duration, dryrun bool) {
				api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(DetailsResponse(AsMap("ID", "abc123")), nil)
				api.EXPECT().ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{}, errors.New("create error")).Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine()
			tt.mockSet(api, tt.args.c, tt.args.stressors, tt.args.image, tt.args.pull, tt.args.duration, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
			_, _, _, err := client.StressContainer(tt.args.ctx, tt.args.c, tt.args.stressors, tt.args.image, tt.args.pull, tt.args.duration, false, tt.args.dryrun)

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.StressContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

func TestStressContainerInfoAPIFailure(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(DetailsResponse(AsMap("ID", "abc123")), nil)
	api.EXPECT().Info(mock.Anything).Return(system.Info{}, errors.New("connection refused"))

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, true, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get docker info")
	api.AssertExpectations(t)
}

func TestStressContainerInfoFailureWithSystemdInspect(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().Info(mock.Anything).Return(system.Info{}, errors.New("connection refused"))
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(
		DetailsResponse(AsMap("ID", "abc123", "CgroupParent", "kubepods-burstable-podXYZ.slice")), nil,
	)

	var capturedHostConfig *container.HostConfig
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything, mock.MatchedBy(func(hc *container.HostConfig) bool {
		capturedHostConfig = hc
		return true
	}), mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)

	assert.Error(t, err)
	assert.NotNil(t, capturedHostConfig)
	assert.Equal(t, "kubepods-burstable-podXYZ.slice", capturedHostConfig.Resources.CgroupParent)

	api.AssertExpectations(t)
}

func TestStressContainerSystemdCgroupDriver(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "systemd"}, nil)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(DetailsResponse(AsMap("ID", "abc123")), nil)
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything, mock.MatchedBy(func(hc *container.HostConfig) bool {
		return hc.Resources.CgroupParent == "system.slice"
	}), mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)

	assert.Error(t, err)
	api.AssertExpectations(t)
}

func TestStressContainerConfigNoCgroupsV1Artifacts(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(DetailsResponse(AsMap("ID", "abc123")), nil)

	var capturedConfig *container.Config
	var capturedHostConfig *container.HostConfig
	api.EXPECT().ContainerCreate(mock.Anything,
		mock.MatchedBy(func(cfg *container.Config) bool {
			capturedConfig = cfg
			return true
		}),
		mock.MatchedBy(func(hc *container.HostConfig) bool {
			capturedHostConfig = hc
			return true
		}),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "2"}, "stress-ng:latest", false, 10*time.Second, false, false)

	assert.Error(t, err)
	assert.NotNil(t, capturedConfig)
	assert.NotNil(t, capturedHostConfig)

	assert.Equal(t, []string{"/stress-ng"}, []string(capturedConfig.Entrypoint))
	assert.Equal(t, []string{"--cpu", "2"}, []string(capturedConfig.Cmd))

	assert.Empty(t, capturedHostConfig.Mounts)
	assert.Empty(t, capturedHostConfig.Binds)
	assert.Empty(t, capturedHostConfig.CapAdd)
	assert.Empty(t, capturedHostConfig.SecurityOpt)
	assert.Equal(t, "/docker/abc123", capturedHostConfig.Resources.CgroupParent)
	assert.True(t, capturedHostConfig.AutoRemove)

	api.AssertExpectations(t)
}

func TestStressContainerInjectCgroupCgroupfs(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(DetailsResponse(AsMap("ID", "abc123")), nil)
	api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)

	var capturedConfig *container.Config
	var capturedHostConfig *container.HostConfig
	api.EXPECT().ContainerCreate(mock.Anything,
		mock.MatchedBy(func(cfg *container.Config) bool {
			capturedConfig = cfg
			return true
		}),
		mock.MatchedBy(func(hc *container.HostConfig) bool {
			capturedHostConfig = hc
			return true
		}),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "2"}, "stress-ng:latest", false, 10*time.Second, true, false)

	assert.Error(t, err)
	assert.NotNil(t, capturedConfig)
	assert.NotNil(t, capturedHostConfig)

	assert.Equal(t, []string{"/cg-inject"}, []string(capturedConfig.Entrypoint))
	assert.Equal(t, []string{"--target-id", "abc123", "--cgroup-driver", "cgroupfs", "--", "/stress-ng", "--cpu", "2"}, []string(capturedConfig.Cmd))

	assert.Equal(t, container.CgroupnsMode("host"), capturedHostConfig.CgroupnsMode)
	assert.Equal(t, []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"}, capturedHostConfig.Binds)
	assert.Empty(t, capturedHostConfig.Resources.CgroupParent)
	assert.Empty(t, capturedHostConfig.CapAdd)
	assert.True(t, capturedHostConfig.AutoRemove)

	api.AssertExpectations(t)
}

func TestStressContainerInjectCgroupSystemd(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "def456",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().ContainerInspect(mock.Anything, "def456").Return(DetailsResponse(AsMap("ID", "def456")), nil)
	api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "systemd"}, nil)

	var capturedConfig *container.Config
	var capturedHostConfig *container.HostConfig
	api.EXPECT().ContainerCreate(mock.Anything,
		mock.MatchedBy(func(cfg *container.Config) bool {
			capturedConfig = cfg
			return true
		}),
		mock.MatchedBy(func(hc *container.HostConfig) bool {
			capturedHostConfig = hc
			return true
		}),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, true, false)

	assert.Error(t, err)
	assert.NotNil(t, capturedConfig)
	assert.NotNil(t, capturedHostConfig)

	assert.Equal(t, []string{"/cg-inject"}, []string(capturedConfig.Entrypoint))
	assert.Equal(t, []string{"--target-id", "def456", "--cgroup-driver", "systemd", "--", "/stress-ng", "--cpu", "1"}, []string(capturedConfig.Cmd))

	assert.Equal(t, container.CgroupnsMode("host"), capturedHostConfig.CgroupnsMode)
	assert.Equal(t, []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"}, capturedHostConfig.Binds)
	assert.Empty(t, capturedHostConfig.Resources.CgroupParent)
	assert.Empty(t, capturedHostConfig.CapAdd)
	assert.True(t, capturedHostConfig.AutoRemove)

	api.AssertExpectations(t)
}

func TestStressContainerK8sCgroupParentCgroupfs(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(
		DetailsResponse(AsMap("ID", "abc123", "CgroupParent", "/kubepods/burstable/pod-abc123")), nil,
	)

	var capturedHostConfig *container.HostConfig
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything,
		mock.MatchedBy(func(hc *container.HostConfig) bool {
			capturedHostConfig = hc
			return true
		}),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "2"}, "stress-ng:latest", false, 10*time.Second, false, false)

	assert.Error(t, err)
	assert.NotNil(t, capturedHostConfig)
	assert.Equal(t, "/kubepods/burstable/pod-abc123/abc123", capturedHostConfig.Resources.CgroupParent)

	api.AssertExpectations(t)
}

func TestStressContainerK8sCgroupParentSystemd(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "def456",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "systemd"}, nil)
	api.EXPECT().ContainerInspect(mock.Anything, "def456").Return(
		DetailsResponse(AsMap("ID", "def456", "CgroupParent", "kubepods-burstable-podXYZ.slice")), nil,
	)

	var capturedHostConfig *container.HostConfig
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything,
		mock.MatchedBy(func(hc *container.HostConfig) bool {
			capturedHostConfig = hc
			return true
		}),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "1"}, "stress-ng:latest", false, 10*time.Second, false, false)

	assert.Error(t, err)
	assert.NotNil(t, capturedHostConfig)
	assert.Equal(t, "kubepods-burstable-podXYZ.slice", capturedHostConfig.Resources.CgroupParent)

	api.AssertExpectations(t)
}

func TestStressContainerEmptyCgroupParentFallback(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(DetailsResponse(AsMap("ID", "abc123")), nil)

	var capturedHostConfig *container.HostConfig
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything,
		mock.MatchedBy(func(hc *container.HostConfig) bool {
			capturedHostConfig = hc
			return true
		}),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "2"}, "stress-ng:latest", false, 10*time.Second, false, false)

	assert.Error(t, err)
	assert.NotNil(t, capturedHostConfig)
	assert.Equal(t, "/docker/abc123", capturedHostConfig.Resources.CgroupParent)

	api.AssertExpectations(t)
}

func TestStressContainerInjectCgroupWithK8sPath(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(
		DetailsResponse(AsMap("ID", "abc123", "CgroupParent", "/kubepods/burstable/pod-abc123")), nil,
	)
	api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)

	var capturedConfig *container.Config
	var capturedHostConfig *container.HostConfig
	api.EXPECT().ContainerCreate(mock.Anything,
		mock.MatchedBy(func(cfg *container.Config) bool {
			capturedConfig = cfg
			return true
		}),
		mock.MatchedBy(func(hc *container.HostConfig) bool {
			capturedHostConfig = hc
			return true
		}),
		mock.Anything, mock.Anything, mock.Anything,
	).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--cpu", "2"}, "stress-ng:latest", false, 10*time.Second, true, false)

	assert.Error(t, err)
	assert.NotNil(t, capturedConfig)
	assert.NotNil(t, capturedHostConfig)

	assert.Equal(t, []string{"/cg-inject"}, []string(capturedConfig.Entrypoint))
	assert.Equal(t, []string{"--cgroup-path", "/kubepods/burstable/pod-abc123/abc123", "--", "/stress-ng", "--cpu", "2"}, []string(capturedConfig.Cmd))

	assert.Equal(t, container.CgroupnsMode("host"), capturedHostConfig.CgroupnsMode)
	assert.Equal(t, []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"}, capturedHostConfig.Binds)
	assert.Empty(t, capturedHostConfig.Resources.CgroupParent)
	assert.True(t, capturedHostConfig.AutoRemove)

	api.AssertExpectations(t)
}

func TestStressContainerInjectCgroupCustomImage(t *testing.T) {
	api := NewMockEngine()
	c := &ctr.Container{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Labels:        map[string]string{},
		Networks:      map[string]ctr.NetworkLink{},
	}

	customImage := "ghcr.io/myorg/pumba-stress:v1.0"
	api.EXPECT().ContainerInspect(mock.Anything, "abc123").Return(DetailsResponse(AsMap("ID", "abc123")), nil)
	api.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)

	var capturedConfig *container.Config
	api.EXPECT().ContainerCreate(mock.Anything,
		mock.MatchedBy(func(cfg *container.Config) bool {
			capturedConfig = cfg
			return true
		}),
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(container.CreateResponse{}, errors.New("stop here")).Once()

	client := dockerClient{containerAPI: api, imageAPI: api, systemAPI: api}
	_, _, _, err := client.StressContainer(context.TODO(), c, []string{"--vm", "1"}, customImage, false, 10*time.Second, true, false)

	assert.Error(t, err)
	assert.NotNil(t, capturedConfig)
	assert.Equal(t, customImage, capturedConfig.Image)
	assert.Equal(t, []string{"/cg-inject"}, []string(capturedConfig.Entrypoint))

	api.AssertExpectations(t)
}
