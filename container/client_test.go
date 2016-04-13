package container

import (
	"errors"
	"testing"
	"time"

	"github.com/samalba/dockerclient"
	"github.com/samalba/dockerclient/mockclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func allContainers(Container) bool { return true }
func noContainers(Container) bool  { return false }

func TestListContainers_Success(t *testing.T) {
	ci := &dockerclient.ContainerInfo{Image: "abc123", Config: &dockerclient.ContainerConfig{Image: "img"}}
	ii := &dockerclient.ImageInfo{}
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo", Names: []string{"bar"}}}, nil)
	api.On("InspectContainer", "foo").Return(ci, nil)
	api.On("InspectImage", "abc123").Return(ii, nil)

	client := dockerClient{api: api}
	cs, err := client.ListContainers(allContainers)

	assert.NoError(t, err)
	assert.Len(t, cs, 1)
	assert.Equal(t, ci, cs[0].containerInfo)
	assert.Equal(t, ii, cs[0].imageInfo)
	api.AssertExpectations(t)
}

func TestListContainers_Filter(t *testing.T) {
	ci := &dockerclient.ContainerInfo{Image: "abc123", Config: &dockerclient.ContainerConfig{Image: "img"}}
	ii := &dockerclient.ImageInfo{}
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo", Names: []string{"bar"}}}, nil)
	api.On("InspectContainer", "foo").Return(ci, nil)
	api.On("InspectImage", "abc123").Return(ii, nil)

	client := dockerClient{api: api}
	cs, err := client.ListContainers(noContainers)

	assert.NoError(t, err)
	assert.Len(t, cs, 0)
	api.AssertExpectations(t)
}

func TestListContainers_ListError(t *testing.T) {
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{}, errors.New("oops"))

	client := dockerClient{api: api}
	_, err := client.ListContainers(allContainers)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestListContainers_InspectContainerError(t *testing.T) {
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo", Names: []string{"bar"}}}, nil)
	api.On("InspectContainer", "foo").Return(&dockerclient.ContainerInfo{}, errors.New("uh-oh"))

	client := dockerClient{api: api}
	_, err := client.ListContainers(allContainers)

	assert.Error(t, err)
	assert.EqualError(t, err, "uh-oh")
	api.AssertExpectations(t)
}

func TestListContainers_InspectImageError(t *testing.T) {
	ci := &dockerclient.ContainerInfo{Image: "abc123", Config: &dockerclient.ContainerConfig{Image: "img"}}
	ii := &dockerclient.ImageInfo{}
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo", Names: []string{"bar"}}}, nil)
	api.On("InspectContainer", "foo").Return(ci, nil)
	api.On("InspectImage", "abc123").Return(ii, errors.New("whoops"))

	client := dockerClient{api: api}
	_, err := client.ListContainers(allContainers)

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestStopContainer_DefaultSuccess(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	ci := &dockerclient.ContainerInfo{
		State: &dockerclient.State{
			Running: false,
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)
	api.On("InspectContainer", "abc123").Return(ci, nil).Once()
	api.On("RemoveContainer", "abc123", true, false).Return(nil)
	api.On("InspectContainer", "abc123").Return(&dockerclient.ContainerInfo{}, errors.New("Not Found"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, time.Second, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	ci := &dockerclient.ContainerInfo{
		State: &dockerclient.State{
			Running: false,
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)
	api.On("InspectContainer", "abc123").Return(ci, nil).Once()
	api.On("RemoveContainer", "abc123", true, false).Return(nil)
	api.On("InspectContainer", "abc123").Return(&dockerclient.ContainerInfo{}, errors.New("Not Found"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, time.Second, true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "KillContainer", "abc123", "SIGTERM")
	api.AssertNotCalled(t, "InspectContainer", "abc123")
	api.AssertNotCalled(t, "RemoveContainer", "abc123", true, false)
	api.AssertNotCalled(t, "InspectContainer", "abc123")
}

func TestKillContainer_DefaultSuccess(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)

	client := dockerClient{api: api}
	err := client.KillContainer(c, "SIGTERM", false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestKillContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)

	client := dockerClient{api: api}
	err := client.KillContainer(c, "SIGTERM", true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "KillContainer", "abc123", "SIGTERM")
}

func TestStopContainer_CustomSignalSuccess(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name: "foo",
			Id:   "abc123",
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{"com.gaiaadm.pumba.stop-signal": "SIGUSR1"}},
		},
	}

	ci := &dockerclient.ContainerInfo{
		State: &dockerclient.State{
			Running: false,
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGUSR1").Return(nil)
	api.On("InspectContainer", "abc123").Return(ci, nil).Once()
	api.On("RemoveContainer", "abc123", true, false).Return(nil)
	api.On("InspectContainer", "abc123").Return(&dockerclient.ContainerInfo{}, errors.New("Not Found"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, time.Second, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_KillContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(errors.New("oops"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, time.Second, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestStopContainer_RemoveContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)
	api.On("InspectContainer", "abc123").Return(&dockerclient.ContainerInfo{}, errors.New("dangit"))
	api.On("RemoveContainer", "abc123", true, false).Return(errors.New("whoops"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, time.Second, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestStartContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:       "foo",
			Config:     &dockerclient.ContainerConfig{},
			HostConfig: &dockerclient.HostConfig{},
		},
		imageInfo: &dockerclient.ImageInfo{
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("CreateContainer",
		mock.AnythingOfType("*dockerclient.ContainerConfig"),
		"foo",
		mock.AnythingOfType("*dockerclient.AuthConfig")).Return("def789", nil)
	api.On("StartContainer", "def789", mock.AnythingOfType("*dockerclient.HostConfig")).Return(nil)

	client := dockerClient{api: api}
	err := client.StartContainer(c)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartContainer_CreateContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:       "foo",
			Config:     &dockerclient.ContainerConfig{},
			HostConfig: &dockerclient.HostConfig{},
		},
		imageInfo: &dockerclient.ImageInfo{
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("CreateContainer", mock.Anything, "foo", mock.Anything).Return("", errors.New("oops"))

	client := dockerClient{api: api}
	err := client.StartContainer(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestStartContainer_StartContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:       "foo",
			Config:     &dockerclient.ContainerConfig{},
			HostConfig: &dockerclient.HostConfig{},
		},
		imageInfo: &dockerclient.ImageInfo{
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("CreateContainer", mock.Anything, "foo", mock.Anything).Return("def789", nil)
	api.On("StartContainer", "def789", mock.Anything).Return(errors.New("whoops"))

	client := dockerClient{api: api}
	err := client.StartContainer(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestRenameContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RenameContainer", "abc123", "foo").Return(nil)

	client := dockerClient{api: api}
	err := client.RenameContainer(c, "foo")

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestRenameContainer_Error(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RenameContainer", "abc123", "foo").Return(errors.New("oops"))

	client := dockerClient{api: api}
	err := client.RenameContainer(c, "foo")

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestRemoveImage_Success(t *testing.T) {
	c := Container{
		imageInfo: &dockerclient.ImageInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RemoveImage", "abc123", false).Return([]*dockerclient.ImageDelete{}, nil)

	client := dockerClient{api: api}
	err := client.RemoveImage(c, false, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestRemoveImage_DryRun(t *testing.T) {
	c := Container{
		imageInfo: &dockerclient.ImageInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RemoveImage", "abc123", false).Return([]*dockerclient.ImageDelete{}, nil)

	client := dockerClient{api: api}
	err := client.RemoveImage(c, false, true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "RemoveImage", "abc123", false)
}

func TestRemoveContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RemoveContainer", "abc123", true, true).Return(nil)

	client := dockerClient{api: api}
	err := client.RemoveContainer(c, true, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestRemoveContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RemoveContainer", "abc123", true, true).Return(nil)

	client := dockerClient{api: api}
	err := client.RemoveContainer(c, true, true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "RemoveContainer", "abc123", true, true)
}

func TestRemoveImage_Error(t *testing.T) {
	c := Container{
		imageInfo: &dockerclient.ImageInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RemoveImage", "abc123", false).Return([]*dockerclient.ImageDelete{}, errors.New("oops"))

	client := dockerClient{api: api}
	err := client.RemoveImage(c, false, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}
