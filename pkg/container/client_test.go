package container

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func NewMockEngine() *mocks.APIClient {
	return new(mocks.APIClient)
}

func mockAllContainers(_ *Container) bool {
	return true
}
func mockNoContainers(_ *Container) bool {
	return false
}

func TestListContainers_Success(t *testing.T) {
	containerDetails := DetailsResponse(AsMap("Image", "abc123"))
	allContainersResponse := Containers(Response(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	imageDetails := ImageDetailsResponse(AsMap())

	api := NewMockEngine()
	api.On("ContainerList", mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.On("ContainerInspect", mock.Anything, "foo").Return(containerDetails, nil)
	api.On("ImageInspectWithRaw", mock.Anything, "abc123").Return(imageDetails, []byte{}, nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	containers, err := client.ListContainers(context.TODO(), mockAllContainers, ListOpts{All: true})

	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Equal(t, containerDetails, containers[0].ContainerInfo)
	assert.Equal(t, imageDetails, containers[0].ImageInfo)
	api.AssertExpectations(t)
}

func TestListContainers_Filter(t *testing.T) {
	containerDetails := DetailsResponse(AsMap("Image", "abc123"))
	allContainersResponse := Containers(Response(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	imageDetails := ImageDetailsResponse(AsMap())

	api := NewMockEngine()
	api.On("ContainerList", mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.On("ContainerInspect", mock.Anything, "foo").Return(containerDetails, nil)
	api.On("ImageInspectWithRaw", mock.Anything, "abc123").Return(imageDetails, []byte{}, nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	containers, err := client.ListContainers(context.TODO(), mockNoContainers, ListOpts{})

	assert.NoError(t, err)
	assert.Len(t, containers, 0)
	api.AssertExpectations(t)
}

func TestListContainers_ListError(t *testing.T) {
	api := NewMockEngine()
	api.On("ContainerList", mock.Anything, mock.Anything).Return(Containers(), errors.New("oops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	_, err := client.ListContainers(context.TODO(), mockAllContainers, ListOpts{All: true})

	assert.Error(t, err)
	assert.EqualError(t, err, "failed to list containers: oops")
	api.AssertExpectations(t)
}

func TestListContainers_InspectContainerError(t *testing.T) {
	api := NewMockEngine()
	allContainersResponse := Containers(Response(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	api.On("ContainerList", mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.On("ContainerInspect", mock.Anything, "foo").Return(DetailsResponse(AsMap()), errors.New("uh-oh"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	_, err := client.ListContainers(context.TODO(), mockAllContainers, ListOpts{All: true})

	assert.Error(t, err)
	assert.EqualError(t, err, "failed to inspect container: uh-oh")
	api.AssertExpectations(t)
}

func TestListContainers_InspectImageError(t *testing.T) {
	allContainersResponse := Containers(Response(AsMap(
		"ID", "foo",
		"Names", []string{"bar"})),
	)
	resp := DetailsResponse(AsMap("Image", "abc123"))
	imageDetailsResponse := ImageDetailsResponse(AsMap())
	api := NewMockEngine()
	api.On("ContainerList", mock.Anything, mock.Anything).Return(allContainersResponse, nil)
	api.On("ContainerInspect", mock.Anything, "foo").Return(resp, nil)
	api.On("ImageInspectWithRaw", mock.Anything, "abc123").Return(imageDetailsResponse, []byte{}, errors.New("whoops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	_, err := client.ListContainers(context.TODO(), mockAllContainers, ListOpts{All: true})

	assert.Error(t, err)
	assert.EqualError(t, err, "failed to inspect container image: whoops")
	api.AssertExpectations(t)
}

func TestStopContainer_DefaultSuccess(t *testing.T) {
	containerDetails := DetailsResponse(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))
	c := &Container{ContainerInfo: containerDetails}
	notRunningContainer := DetailsResponse(AsMap("Running", false))

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(notRunningContainer, nil).Once()

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_DryRun(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
		)),
	}

	notRunningContainer := DetailsResponse(AsMap("Running", false))

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(notRunningContainer, nil).Once()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGKILL").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(DetailsResponse(AsMap()), errors.New("not found"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "ContainerKill", mock.Anything, "abc123", "SIGTERM")
	api.AssertNotCalled(t, "ContainerInspect", mock.Anything, "abc123")
	api.AssertNotCalled(t, "ContainerKill", mock.Anything, "abc123", "SIGKILL")
	api.AssertNotCalled(t, "ContainerInspect", mock.Anything, "abc123")
}

func TestKillContainer_DefaultSuccess(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap(
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
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap(
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
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
			"Labels", map[string]string{"com.gaiaadm.pumba.stop-signal": "SIGUSR1"},
		)),
	}

	notRunningContainer := DetailsResponse(AsMap("Running", false))

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGUSR1").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(notRunningContainer, nil).Once()

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_KillContainerError(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
		)),
	}

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(errors.New("oops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.Error(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_2ndKillContainerError(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap(
			"ID", "abc123",
			"Name", "foo",
		)),
	}

	api := NewMockEngine()
	api.On("ContainerKill", mock.Anything, "abc123", "SIGTERM").Return(nil)
	api.On("ContainerInspect", mock.Anything, "abc123").Return(DetailsResponse(AsMap()), errors.New("dangit"))
	api.On("ContainerKill", mock.Anything, "abc123", "SIGKILL").Return(errors.New("whoops"))

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StopContainer(context.TODO(), c, 1, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "failed to kill container: whoops")
	api.AssertExpectations(t)
}

func TestRemoveContainer_Success(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
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
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
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
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
	}
	engineClient := NewMockEngine()
	engineClient.On("ContainerPause", mock.Anything, "abc123").Return(nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.PauseContainer(context.TODO(), c, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestUnpauseContainer_Success(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
	}
	engineClient := NewMockEngine()
	engineClient.On("ContainerUnpause", mock.Anything, "abc123").Return(nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.UnpauseContainer(context.TODO(), c, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestPauseContainer_DryRun(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
	}

	engineClient := NewMockEngine()
	client := dockerClient{containerAPI: engineClient}
	err := client.PauseContainer(context.TODO(), c, true)

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerPause", mock.Anything, "abc123")
}

func TestPauseContainer_PauseError(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
	}
	engineClient := NewMockEngine()
	engineClient.On("ContainerPause", mock.Anything, "abc123").Return(errors.New("pause"))

	client := dockerClient{containerAPI: engineClient}
	err := client.PauseContainer(context.TODO(), c, false)

	assert.Error(t, err)
	engineClient.AssertExpectations(t)
}

func TestPauseContainer_UnpauseError(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
	}
	engineClient := NewMockEngine()
	engineClient.On("ContainerUnpause", mock.Anything, "abc123").Return(errors.New("unpause"))

	client := dockerClient{containerAPI: engineClient}
	err := client.UnpauseContainer(context.TODO(), c, false)

	assert.Error(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainer_Success(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
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
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, nil, nil, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestStopNetemContainer_Success(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
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
	err := client.StopNetemContainer(context.TODO(), c, "eth0", nil, nil, nil, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainer_DryRun(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
	}

	engineClient := NewMockEngine()
	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, nil, nil, 1*time.Millisecond, "", false, true)

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerExecCreate", mock.Anything)
	engineClient.AssertNotCalled(t, "ContainerExecStart", "abc123", mock.Anything)
}

func TestNetemContainerIPFilter_Success(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
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
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, []*net.IPNet{{IP: net.IP{10, 10, 0, 1}, Mask: net.IPMask{255, 255, 255, 255}}}, nil, nil, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainerSportFilter_Success(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
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
		"parent", "1:0", "prio", "1", "u32", "match", "ip", "sport", "1234", "0xffff", "flowid", "1:3"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config5).Return(types.IDResponse{ID: "cmd5"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd5", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "cmd5").Return(types.ContainerExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, []string{"1234"}, nil, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainerDportFilter_Success(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "abc123")),
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
		"parent", "1:0", "prio", "1", "u32", "match", "ip", "dport", "1234", "0xffff", "flowid", "1:3"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config5).Return(types.IDResponse{ID: "cmd5"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd5", types.ExecStartCheck{}).Return(nil)
	engineClient.On("ContainerExecInspect", ctx, "cmd5").Return(types.ContainerExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, nil, []string{"1234"}, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func Test_tcContainerCommand(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("ID", "targetID")),
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
	pullResponse := imagePullResponse{
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
	engineClient.On("ImagePull", ctx, config.Image, types.ImagePullOptions{}).Return(io.NopCloser(readerResponse), nil)
	// create container
	engineClient.On("ContainerCreate", ctx, &config, &hconfig, (*network.NetworkingConfig)(nil), (*specs.Platform)(nil), "").Return(container.ContainerCreateCreatedBody{ID: "tcID"}, nil)
	// start container
	engineClient.On("ContainerStart", ctx, "tcID", types.ContainerStartOptions{}).Return(nil)

	client := dockerClient{containerAPI: engineClient, imageAPI: engineClient}
	err := client.tcContainerCommand(context.TODO(), c, []string{"test", "me"}, "pumba/tcimage", true)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestStartContainer_DefaultSuccess(t *testing.T) {
	containerDetails := DetailsResponse(AsMap(
		"ID", "abc123",
		"Name", "foo",
	))
	c := &Container{ContainerInfo: containerDetails}

	api := NewMockEngine()
	api.On("ContainerStart", mock.Anything, "abc123", types.ContainerStartOptions{}).Return(nil)

	client := dockerClient{containerAPI: api, imageAPI: api}
	err := client.StartContainer(context.TODO(), c, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartContainer_DryRun(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap(
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

func Test_dockerClient_execOnContainer(t *testing.T) {
	type args struct {
		c          *Container
		ctx        context.Context
		execCmd    string
		execArgs   []string
		privileged bool
	}
	tests := []struct {
		name     string
		args     args
		mockInit func(context.Context, *mocks.APIClient, string, string, []string)
		wantErr  bool
	}{
		{
			name: "run non-privileged command with args",
			args: args{
				c:        &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123"))},
				ctx:      context.TODO(),
				execCmd:  "test-app",
				execArgs: []string{"one", "two", "three"},
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				// prepare which command
				checkConfig := types.ExecConfig{Cmd: []string{"which", cmd}}
				engine.On("ContainerExecCreate", ctx, cID, checkConfig).Return(types.IDResponse{ID: "whichID"}, nil)
				engine.On("ContainerExecStart", ctx, "whichID", types.ExecStartCheck{}).Return(nil)
				engine.On("ContainerExecInspect", ctx, "whichID").Return(types.ContainerExecInspect{}, nil)
				// prepare main command
				execConfig := types.ExecConfig{Cmd: append([]string{cmd}, args...), Privileged: false}
				engine.On("ContainerExecCreate", ctx, cID, execConfig).Return(types.IDResponse{ID: "cmdID"}, nil)
				engine.On("ContainerExecStart", ctx, "cmdID", types.ExecStartCheck{}).Return(nil)
				engine.On("ContainerExecInspect", ctx, "cmdID").Return(types.ContainerExecInspect{}, nil)
			},
		},
		{
			name: "run privileged command with args",
			args: args{
				c:          &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123"))},
				ctx:        context.TODO(),
				execCmd:    "test-app",
				execArgs:   []string{"one", "two", "three"},
				privileged: true,
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				// prepare which command
				checkConfig := types.ExecConfig{Cmd: []string{"which", cmd}}
				engine.On("ContainerExecCreate", ctx, cID, checkConfig).Return(types.IDResponse{ID: "whichID"}, nil)
				engine.On("ContainerExecStart", ctx, "whichID", types.ExecStartCheck{}).Return(nil)
				engine.On("ContainerExecInspect", ctx, "whichID").Return(types.ContainerExecInspect{}, nil)
				// prepare main command
				execConfig := types.ExecConfig{Cmd: append([]string{cmd}, args...), Privileged: true}
				engine.On("ContainerExecCreate", ctx, cID, execConfig).Return(types.IDResponse{ID: "cmdID"}, nil)
				engine.On("ContainerExecStart", ctx, "cmdID", types.ExecStartCheck{}).Return(nil)
				engine.On("ContainerExecInspect", ctx, "cmdID").Return(types.ContainerExecInspect{}, nil)
			},
		},
		{
			name: "fail to find command",
			args: args{
				c:        &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123"))},
				ctx:      context.TODO(),
				execCmd:  "test-app",
				execArgs: []string{"one", "two", "three"},
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				// prepare which command
				checkConfig := types.ExecConfig{Cmd: []string{"which", cmd}}
				engine.On("ContainerExecCreate", ctx, cID, checkConfig).Return(types.IDResponse{ID: "whichID"}, nil)
				engine.On("ContainerExecStart", ctx, "whichID", types.ExecStartCheck{}).Return(nil)
				engine.On("ContainerExecInspect", ctx, "whichID").Return(types.ContainerExecInspect{ExitCode: 1}, nil)
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecCreate 'which'",
			args: args{
				c:       &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123"))},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := types.ExecConfig{Cmd: []string{"which", cmd}}
				engine.On("ContainerExecCreate", ctx, cID, checkConfig).Return(types.IDResponse{}, errors.New("which error"))
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecStart 'which'",
			args: args{
				c:       &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123"))},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := types.ExecConfig{Cmd: []string{"which", cmd}}
				engine.On("ContainerExecCreate", ctx, cID, checkConfig).Return(types.IDResponse{ID: "whichID"}, nil)
				engine.On("ContainerExecStart", ctx, "whichID", types.ExecStartCheck{}).Return(errors.New("which error"))
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecInspect 'which'",
			args: args{
				c:       &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123"))},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := types.ExecConfig{Cmd: []string{"which", cmd}}
				engine.On("ContainerExecCreate", ctx, cID, checkConfig).Return(types.IDResponse{ID: "whichID"}, nil)
				engine.On("ContainerExecStart", ctx, "whichID", types.ExecStartCheck{}).Return(nil)
				engine.On("ContainerExecInspect", ctx, "whichID").Return(types.ContainerExecInspect{}, errors.New("which error"))
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecCreate 'test-app'",
			args: args{
				c:       &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123"))},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := types.ExecConfig{Cmd: []string{"which", cmd}}
				engine.On("ContainerExecCreate", ctx, cID, checkConfig).Return(types.IDResponse{ID: "whichID"}, nil)
				engine.On("ContainerExecStart", ctx, "whichID", types.ExecStartCheck{}).Return(nil)
				engine.On("ContainerExecInspect", ctx, "whichID").Return(types.ContainerExecInspect{}, nil)
				// prepare main command
				execConfig := types.ExecConfig{Cmd: append([]string{cmd}, args...), Privileged: false}
				engine.On("ContainerExecCreate", ctx, cID, execConfig).Return(types.IDResponse{}, errors.New("cmd error"))
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecStart 'test-app'",
			args: args{
				c:       &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123"))},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := types.ExecConfig{Cmd: []string{"which", cmd}}
				engine.On("ContainerExecCreate", ctx, cID, checkConfig).Return(types.IDResponse{ID: "whichID"}, nil)
				engine.On("ContainerExecStart", ctx, "whichID", types.ExecStartCheck{}).Return(nil)
				engine.On("ContainerExecInspect", ctx, "whichID").Return(types.ContainerExecInspect{}, nil)
				// prepare main command
				execConfig := types.ExecConfig{Cmd: append([]string{cmd}, args...), Privileged: false}
				engine.On("ContainerExecCreate", ctx, cID, execConfig).Return(types.IDResponse{ID: "cmdID"}, nil)
				engine.On("ContainerExecStart", ctx, "cmdID", types.ExecStartCheck{}).Return(errors.New("cmd error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockEngine()
			client := dockerClient{
				containerAPI: mockClient,
			}
			// init mock engine
			tt.mockInit(tt.args.ctx, mockClient, tt.args.c.ContainerInfo.ID, tt.args.execCmd, tt.args.execArgs)
			err := client.execOnContainer(tt.args.ctx, tt.args.c, tt.args.execCmd, tt.args.execArgs, tt.args.privileged)
			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.execOnContainer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func Test_dockerClient_stressContainerCommand(t *testing.T) {
	type args struct {
		ctx       context.Context
		targetID  string
		stressors []string
		image     string
		pull      bool
	}
	tests := []struct {
		name       string
		args       args
		mockInit   func(context.Context, *mocks.APIClient, *mockConn, string, []string, string, bool)
		want       string
		wantOutput string
		wantErr    bool
		wantErrCh  bool
	}{
		{
			name: "stress test with pull image",
			args: args{
				ctx:       context.TODO(),
				targetID:  "123",
				stressors: []string{"--cpu", "4"},
				image:     "test/stress-ng",
				pull:      true,
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				// pull response
				pullResponse := imagePullResponse{
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
				engine.On("ImagePull", ctx, image, types.ImagePullOptions{}).Return(io.NopCloser(readerResponse), nil)
				engine.On("ContainerCreate", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(container.ContainerCreateCreatedBody{ID: "000"}, nil)
				engine.On("ContainerAttach", ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.On("ContainerStart", ctx, "000", mock.Anything).Return(nil)
				inspect := types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{},
					},
				}
				conn.Mock.On("Close").Return(nil)
				engine.On("ContainerInspect", ctx, "000").Return(inspect, nil)
			},
			want:       "000",
			wantOutput: "stress completed",
		},
		{
			name: "stress test fail to pull image",
			args: args{
				ctx:   context.TODO(),
				image: "test/stress-ng",
				pull:  true,
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.On("ImagePull", ctx, image, types.ImagePullOptions{}).Return(io.NopCloser(bytes.NewReader([]byte("{}"))), errors.New("failed to pull image"))
			},
			wantErr: true,
		},
		{
			name: "stress test without pull image",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.On("ContainerCreate", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(container.ContainerCreateCreatedBody{ID: "000"}, nil)
				engine.On("ContainerAttach", ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.On("ContainerStart", ctx, "000", mock.Anything).Return(nil)
				inspect := types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{},
					},
				}
				engine.On("ContainerInspect", ctx, "000").Return(inspect, nil)
				conn.Mock.On("Close").Return(nil)
			},
			want:       "000",
			wantOutput: "stress completed",
		},
		{
			name: "stress-ng exit with error",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.On("ContainerCreate", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(container.ContainerCreateCreatedBody{ID: "000"}, nil)
				engine.On("ContainerAttach", ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.On("ContainerStart", ctx, "000", mock.Anything).Return(nil)
				inspect := types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{ExitCode: 1},
					},
				}
				conn.Mock.On("Close").Return(nil)
				engine.On("ContainerInspect", ctx, "000").Return(inspect, nil)
			},
			want:      "000",
			wantErrCh: true,
		},
		{
			name: "fail to inspect stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.On("ContainerCreate", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(container.ContainerCreateCreatedBody{ID: "000"}, nil)
				engine.On("ContainerAttach", ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.On("ContainerStart", ctx, "000", mock.Anything).Return(nil)
				conn.Mock.On("Close").Return(nil)
				engine.On("ContainerInspect", ctx, "000").Return(types.ContainerJSON{}, errors.New("filed to inspect"))
			},
			want:       "000",
			wantOutput: "stress completed",
			wantErrCh:  true,
		},
		{
			name: "fail to start stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.On("ContainerCreate", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(container.ContainerCreateCreatedBody{ID: "000"}, nil)
				engine.On("ContainerAttach", ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.On("ContainerStart", ctx, "000", mock.Anything).Return(errors.New("failed to start"))
				conn.Mock.On("Close").Return(nil)
				inspect := types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{},
					},
				}
				engine.On("ContainerInspect", ctx, "000").Return(inspect, nil)
			},
			want:       "000",
			wantOutput: "stress completed",
			wantErr:    true,
		},
		{
			name: "fail to attach to stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.On("ContainerCreate", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(container.ContainerCreateCreatedBody{ID: "000"}, nil)
				engine.On("ContainerAttach", ctx, "000", mock.Anything).Return(types.HijackedResponse{}, errors.New("failed to attach"))
			},
			wantErr: true,
		},
		{
			name: "fail to create to stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.On("ContainerCreate", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(container.ContainerCreateCreatedBody{}, errors.New("failed to create"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockEngine()
			mConn := &mockConn{}
			client := dockerClient{
				containerAPI: mockClient,
				imageAPI:     mockClient,
			}
			// init mock engine
			tt.mockInit(tt.args.ctx, mockClient, mConn, tt.args.targetID, tt.args.stressors, tt.args.image, tt.args.pull)
			// test stress command
			got, got1, got2, err := client.stressContainerCommand(tt.args.ctx, tt.args.targetID, tt.args.stressors, tt.args.image, tt.args.pull)
			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.stressContainerCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("dockerClient.stressContainerCommand() got = %v, want %v", got, tt.want)
			}
			select {
			case output := <-got1:
				if output != tt.wantOutput {
					t.Errorf("dockerClient.stressContainerCommand() got = %v, from output channel, want %v", output, tt.wantOutput)
				}
			case err = <-got2:
				if (err != nil) != tt.wantErrCh {
					t.Errorf("dockerClient.stressContainerCommand() error in error channel = %v, wantErrCh %v", err, tt.wantErrCh)
				}
			}
			mockClient.AssertExpectations(t)
			mConn.Mock.AssertExpectations(t)
		})
	}
}
