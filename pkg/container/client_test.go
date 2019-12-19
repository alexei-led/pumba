package container

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"

	"github.com/alexei-led/pumba/mocks"
	"github.com/alexei-led/pumba/pkg/util"
)

func NewMockEngine() *mocks.APIClient {
	return new(mocks.APIClient)
}

func mock_allContainers(c Container) bool {
	return true
}
func mock_noContainers(c Container) bool {
	return false
}

func TestListContainers_Success(t *testing.T) {
	containerDetails := ContainerDetailsResponse(AsMap("Image", "abc123"))
	allContainersResponse := Containers(ContainerResponse(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	imageDetails := ImageDetailsResponse(AsMap())

	api := NewMockEngine()
	api.On("ContainerList", mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.On("ContainerInspect", mock.Anything, "foo").Return(containerDetails, nil)
	api.On("ImageInspectWithRaw", mock.Anything, "abc123").Return(imageDetails, []byte{}, nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	containers, err := client.ListContainers(context.TODO(), mock_allContainers, ListOpts{All: true})

	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Equal(t, containerDetails, containers[0].containerInfo)
	assert.Equal(t, imageDetails, containers[0].imageInfo)
	api.AssertExpectations(t)
}

func TestListContainers_Filter(t *testing.T) {
	containerDetails := ContainerDetailsResponse(AsMap("Image", "abc123"))
	allContainersResponse := Containers(ContainerResponse(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	imageDetails := ImageDetailsResponse(AsMap())

	api := NewMockEngine()
	api.On("ContainerList", mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.On("ContainerInspect", mock.Anything, "foo").Return(containerDetails, nil)
	api.On("ImageInspectWithRaw", mock.Anything, "abc123").Return(imageDetails, []byte{}, nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	containers, err := client.ListContainers(context.TODO(), mock_noContainers, ListOpts{})

	assert.NoError(t, err)
	assert.Len(t, containers, 0)
	api.AssertExpectations(t)
}

func TestListContainers_ListError(t *testing.T) {
	api := NewMockEngine()
	api.On("ContainerList", mock.Anything, mock.Anything).Return(Containers(), errors.New("oops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	_, err := client.ListContainers(context.TODO(), mock_allContainers, ListOpts{All: true})

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestListContainers_InspectContainerError(t *testing.T) {
	api := NewMockEngine()
	allContainersResponse := Containers(ContainerResponse(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	api.On("ContainerList", mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.On("ContainerInspect", mock.Anything, "foo").Return(ContainerDetailsResponse(AsMap()), errors.New("uh-oh"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	_, err := client.ListContainers(context.TODO(), mock_allContainers, ListOpts{All: true})

	assert.Error(t, err)
	assert.EqualError(t, err, "uh-oh")
	api.AssertExpectations(t)
}

func TestListContainers_InspectImageError(t *testing.T) {
	allContainersResponse := Containers(ContainerResponse(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	containerDetailsResponse := ContainerDetailsResponse(AsMap("Image", "abc123"))
	imageDetailsResponse := ImageDetailsResponse(AsMap())
	api := NewMockEngine()
	api.On("ContainerList", mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.On("ContainerInspect", mock.Anything, "foo").Return(containerDetailsResponse, nil)
	api.On("ImageInspectWithRaw", mock.Anything, "abc123").Return(imageDetailsResponse, []byte{}, errors.New("whoops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	_, err := client.ListContainers(context.TODO(), mock_allContainers, ListOpts{All: true})

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestStopContainer_DefaultSuccess(t *testing.T) {
	containerDetails := ContainerDetailsResponse(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))
	c := Container{containerInfo: containerDetails}
	notRunningContainer := ContainerDetailsResponse(AsMap("Running", false))

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(notRunningContainer, nil).Once()

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
		)),
	}

	notRunningContainer := ContainerDetailsResponse(AsMap("Running", false))

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(notRunningContainer, nil).Once()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGKILL").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(ContainerDetailsResponse(AsMap()), errors.New("Not Found"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "ContainerKill", mock.Anything, "abc123", "SIGTERM")
	api.AssertNotCalled(t, "ContainerInspect", mock.Anything, "abc123")
	api.AssertNotCalled(t, "ContainerKill", mock.Anything, "abc123", "SIGKILL")
	api.AssertNotCalled(t, "ContainerInspect", mock.Anything, "abc123")
}

func TestKillContainer_DefaultSuccess(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
		)),
	}

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.KillContainer(context.TODO(), c, "SIGTERM", false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestKillContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
		)),
	}

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.KillContainer(context.TODO(), c, "SIGTERM", true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "ContainerKill", mock.Anything, "abc123", "SIGTERM")
}

func TestStopContainer_CustomSignalSuccess(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
			"Labels", map[string]string{"com.gaiaadm.pumba.stop-signal": "SIGUSR1"},
		)),
	}

	notRunningContainer := ContainerDetailsResponse(AsMap("Running", false))

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGUSR1").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(notRunningContainer, nil).Once()

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_KillContainerError(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
		)),
	}

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(errors.New("oops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestStopContainer_2ndKillContainerError(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
		)),
	}

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(ContainerDetailsResponse(AsMap()), errors.New("dangit"))
	api.On("ContainerKill", mock.Anything, "abc123", "SIGKILL").Return(errors.New("whoops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestRemoveContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}

	engineClient := NewMockEngine()
	removeOpts := types.ContainerRemoveOptions{RemoveVolumes: true, RemoveLinks: true, Force: true}
	engineClient.On("ContainerRemove", mock.Anything, "abc123", removeOpts).Return(nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.RemoveContainer(context.TODO(), c, true, true, true, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestRemoveContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}

	engineClient := NewMockEngine()
	removeOpts := types.ContainerRemoveOptions{RemoveVolumes: true, RemoveLinks: true, Force: true}
	engineClient.On("ContainerRemove", mock.Anything, "abc123", removeOpts).Return(nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.RemoveContainer(context.TODO(), c, true, true, true, true)

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerRemove", mock.Anything, "abc123", removeOpts)
}

func TestPauseContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}
	engineClient := NewMockEngine()
	engineClient.On("ContainerPause", mock.Anything, "abc123").Return(nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.PauseContainer(context.TODO(), c, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestUnpauseContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}
	engineClient := NewMockEngine()
	engineClient.On("ContainerUnpause", mock.Anything, "abc123").Return(nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.UnpauseContainer(context.TODO(), c, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestPauseContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}

	engineClient := NewMockEngine()
	client := dockerClient{containerAPI: engineClient}
	err := client.PauseContainer(context.TODO(), c, true)

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerPause", mock.Anything, "abc123")
}

func TestPauseContainer_PauseError(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}
	engineClient := NewMockEngine()
	engineClient.On("ContainerPause", mock.Anything, "abc123").Return(errors.New("pause"))

	client := dockerClient{containerAPI: engineClient}
	err := client.PauseContainer(context.TODO(), c, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "pause")
	engineClient.AssertExpectations(t)
}

func TestPauseContainer_UnpauseError(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}
	engineClient := NewMockEngine()
	engineClient.On("ContainerUnpause", mock.Anything, "abc123").Return(errors.New("unpause"))

	client := dockerClient{containerAPI: engineClient}
	err := client.UnpauseContainer(context.TODO(), c, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "unpause")
	engineClient.AssertExpectations(t)
}

func TestNetemContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}

	engineClient := NewMockEngine()

	checkConfig := types.ExecConfig{Cmd: []string{"which", "tc"}}
	engineClient.On("ContainerExecCreate", mock.Anything, "abc123", checkConfig).Return(types.IDResponse{ID: "checkID"}, nil)
	engineClient.On("ContainerExecStart", mock.Anything, "checkID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", mock.Anything, "checkID").Return(types.ContainerExecInspect{}, nil)

	config := types.ExecConfig{Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay", "500ms"}, Privileged: true}
	engineClient.On("ContainerExecCreate", mock.Anything, "abc123", config).Return(types.IDResponse{ID: "testID"}, nil)
	engineClient.On("ContainerExecStart", mock.Anything, "testID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", mock.Anything, "testID").Return(types.ContainerExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestStopNetemContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := types.ExecConfig{Cmd: []string{"which", "tc"}}
	engineClient.On("ContainerExecCreate", ctx, "abc123", checkConfig).Return(types.IDResponse{ID: "checkID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "checkID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "checkID").Return(types.ContainerExecInspect{}, nil)

	stopConfig := types.ExecConfig{Cmd: []string{"tc", "qdisc", "del", "dev", "eth0", "root", "netem"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", stopConfig).Return(types.IDResponse{ID: "testID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "testID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "testID").Return(types.ContainerExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.StopNetemContainer(context.TODO(), c, "eth0", nil, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}

	engineClient := NewMockEngine()
	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, 1*time.Millisecond, "", false, true)

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerExecCreate", mock.Anything)
	engineClient.AssertNotCalled(t, "ContainerExecStart", "abc123", mock.Anything)
}

func TestNetemContainerIPFilter_Success(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := types.ExecConfig{Cmd: []string{"which", "tc"}}
	engineClient.On("ContainerExecCreate", ctx, "abc123", checkConfig).Return(types.IDResponse{ID: "checkID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "checkID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "checkID").Return(types.ContainerExecInspect{}, nil)

	config1 := types.ExecConfig{Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "handle", "1:", "prio"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config1).Return(types.IDResponse{ID: "cmd1"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd1", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "cmd1").Return(types.ContainerExecInspect{}, nil)

	config2 := types.ExecConfig{Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:1", "handle", "10:", "sfq"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config2).Return(types.IDResponse{ID: "cmd2"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd2", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "cmd2").Return(types.ContainerExecInspect{}, nil)

	config3 := types.ExecConfig{Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:2", "handle", "20:", "sfq"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config3).Return(types.IDResponse{ID: "cmd3"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd3", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "cmd3").Return(types.ContainerExecInspect{}, nil)

	config4 := types.ExecConfig{Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:3", "handle", "30:", "netem", "delay", "500ms"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config4).Return(types.IDResponse{ID: "cmd4"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd4", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "cmd4").Return(types.ContainerExecInspect{}, nil)

	config5 := types.ExecConfig{Cmd: []string{"tc", "filter", "add", "dev", "eth0", "protocol", "ip",
		"parent", "1:0", "prio", "1", "u32", "match", "ip", "dst", "10.10.0.1/32", "flowid", "1:3"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config5).Return(types.IDResponse{ID: "cmd5"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd5", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "cmd5").Return(types.ContainerExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, []*net.IPNet{util.ParseCIDR("10.10.0.1")}, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func Test_tcContainerCommand(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "targetID")),
	}

	config := container.Config{
		Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Entrypoint: []string{"tc"},
		Cmd:        []string{"test", "me"},
		Image:      "pumba/tcimage",
	}
	// host config
	hconfig := container.HostConfig{
		// auto remove container on tc command exit
		AutoRemove: true,
		// NET_ADMIN is required for "tc netem"
		CapAdd: []string{"NET_ADMIN"},
		// use target container network stack
		NetworkMode: container.NetworkMode("container:targetID"),
		// others
		PortBindings: nat.PortMap{},
		DNS:          []string{},
		DNSOptions:   []string{},
		DNSSearch:    []string{},
	}
	// pull response
	pullResponse := ImagePullResponse{
		Status:   "ok",
		Error:    "no error",
		Progress: "done",
		ProgressDetail: struct {
			Current int `json:"current"`
			Total   int `json:"total"`
		}{
			Current: 100,
			Total:   100,
		},
	}
	pullResponseByte, _ := json.Marshal(pullResponse)
	readerResponse := bytes.NewReader(pullResponseByte)

	ctx := mock.Anything
	engineClient := NewMockEngine()

	// pull image
	engineClient.On("ImagePull", ctx, config.Image, types.ImagePullOptions{}).Return(ioutil.NopCloser(readerResponse), nil)
	// create container
	engineClient.On("ContainerCreate", ctx, &config, &hconfig, (*network.NetworkingConfig)(nil), "").Return(container.ContainerCreateCreatedBody{ID: "tcID"}, nil)
	// start container
	engineClient.On("ContainerStart", ctx, "tcID", types.ContainerStartOptions{}).Return(nil)

	client := dockerClient{containerAPI: engineClient, imageAPI: engineClient}
	err := client.tcContainerCommand(context.TODO(), c, []string{"test", "me"}, "pumba/tcimage", true)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func Test_execOnContainerSuccess(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap("ID", "abc123")),
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := types.ExecConfig{Cmd: []string{"which", "testcmd"}}
	engineClient.On("ContainerExecCreate", ctx, "abc123", checkConfig).Return(types.IDResponse{ID: "checkID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "checkID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "checkID").Return(types.ContainerExecInspect{}, nil)

	execConfig := types.ExecConfig{Cmd: []string{"testcmd", "arg1", "arg2", "arg3"}, Privileged: false}
	engineClient.On("ContainerExecCreate", ctx, "abc123", execConfig).Return(types.IDResponse{ID: "testID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "testID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "testID").Return(types.ContainerExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.execOnContainer(context.TODO(), c, "testcmd", []string{"arg1", "arg2", "arg3"}, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func Test_execOnContainerNotFound(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "abcName",
		)),
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := types.ExecConfig{Cmd: []string{"which", "testcmd"}}
	engineClient.On("ContainerExecCreate", ctx, "abc123", checkConfig).Return(types.IDResponse{ID: "checkID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "checkID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "checkID").Return(types.ContainerExecInspect{ExitCode: 1}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.execOnContainer(context.TODO(), c, "testcmd", []string{"arg1", "arg2", "arg3"}, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "command 'testcmd' not found inside the abcName (abc123) container")
	engineClient.AssertExpectations(t)
}

func Test_execOnContainerFailed(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "abcName",
		)),
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := types.ExecConfig{Cmd: []string{"which", "testcmd"}}
	engineClient.On("ContainerExecCreate", ctx, "abc123", checkConfig).Return(types.IDResponse{ID: "checkID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "checkID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "checkID").Return(types.ContainerExecInspect{}, nil)

	execConfig := types.ExecConfig{Cmd: []string{"testcmd", "arg1", "arg2", "arg3"}, Privileged: false}
	engineClient.On("ContainerExecCreate", ctx, "abc123", execConfig).Return(types.IDResponse{ID: "testID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "testID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "testID").Return(types.ContainerExecInspect{ExitCode: 1}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.execOnContainer(context.TODO(), c, "testcmd", []string{"arg1", "arg2", "arg3"}, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "command 'testcmd' failed in abcName (abc123) container; run it in manually to debug")
	engineClient.AssertExpectations(t)
}

func Test_execOnContainerExecStartError(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "abcName",
		)),
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := types.ExecConfig{Cmd: []string{"which", "testcmd"}}
	engineClient.On("ContainerExecCreate", ctx, "abc123", checkConfig).Return(types.IDResponse{ID: "checkID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "checkID", types.ExecStartCheck{}).Return(errors.New("oops"))

	client := dockerClient{containerAPI: engineClient}
	err := client.execOnContainer(context.TODO(), c, "testcmd", []string{"arg1", "arg2", "arg3"}, false)

	assert.Error(t, err)
	engineClient.AssertExpectations(t)
}

func Test_execOnContainerExecCreateError(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "abcName",
		)),
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := types.ExecConfig{Cmd: []string{"which", "testcmd"}}
	engineClient.On("ContainerExecCreate", ctx, "abc123", checkConfig).Return(types.IDResponse{ID: "checkID"}, errors.New("oops"))

	client := dockerClient{containerAPI: engineClient}
	err := client.execOnContainer(context.TODO(), c, "testcmd", []string{"arg1", "arg2", "arg3"}, false)

	assert.Error(t, err)
	engineClient.AssertExpectations(t)
}

func Test_execOnContainerExecInspectError(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "abcName",
		)),
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := types.ExecConfig{Cmd: []string{"which", "testcmd"}}
	engineClient.On("ContainerExecCreate", ctx, "abc123", checkConfig).Return(types.IDResponse{ID: "checkID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "checkID", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "checkID").Return(types.ContainerExecInspect{}, errors.New("oops"))

	client := dockerClient{containerAPI: engineClient}
	err := client.execOnContainer(context.TODO(), c, "testcmd", []string{"arg1", "arg2", "arg3"}, false)

	assert.Error(t, err)
	engineClient.AssertExpectations(t)
}

func TestStartContainer_DefaultSuccess(t *testing.T) {
	containerDetails := ContainerDetailsResponse(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))
	c := Container{containerInfo: containerDetails}

	api := NewMockEngine()
	api.On("ContainerStart", mock.Anything, "abc123", types.ContainerStartOptions{}).Return(nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StartContainer(context.TODO(), c, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: ContainerDetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
		)),
	}

	api := NewMockEngine()
	api.On("ContainerStart", mock.Anything, "abc123", types.ContainerStartOptions{}).Return(nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StartContainer(context.TODO(), c, true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "ContainerStart", mock.Anything, "abc123", types.ContainerStartOptions{})
}
