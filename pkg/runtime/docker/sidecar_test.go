package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_tcContainerCommands(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "targetID",
	}

	config := ctypes.Config{
		Labels:     map[string]string{"com.gaiaadm.pumba.skip": "true"},
		Entrypoint: []string{"tail"},
		Cmd:        []string{"-f", "/dev/null"},
		Image:      "pumba/tcimage",
		StopSignal: "SIGKILL",
	}
	hconfig := ctypes.HostConfig{
		AutoRemove:   false,
		CapAdd:       []string{"NET_ADMIN"},
		NetworkMode:  ctypes.NetworkMode("container:targetID"),
		PortBindings: nat.PortMap{},
		DNS:          []string{},
		DNSOptions:   []string{},
		DNSSearch:    []string{},
	}
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
	engineClient := NewMockEngine(t)

	engineClient.EXPECT().ImagePull(ctx, config.Image, imagetypes.PullOptions{}).Return(io.NopCloser(readerResponse), nil)
	engineClient.EXPECT().ContainerCreate(ctx, &config, &hconfig, (*network.NetworkingConfig)(nil), (*specs.Platform)(nil), "").Return(ctypes.CreateResponse{ID: "tcID"}, nil)
	engineClient.EXPECT().ContainerStart(ctx, "tcID", ctypes.StartOptions{}).Return(nil)
	engineClient.EXPECT().ContainerExecCreate(ctx, "tcID", ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "test", "one"}}).Return(ctypes.ExecCreateResponse{ID: "execID1"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "execID1", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "execID1").Return(ctypes.ExecInspect{}, nil)
	engineClient.EXPECT().ContainerExecCreate(ctx, "tcID", ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "test", "two"}}).Return(ctypes.ExecCreateResponse{ID: "execID2"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "execID2", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "execID2").Return(ctypes.ExecInspect{}, nil)
	engineClient.EXPECT().ContainerRemove(ctx, "tcID", ctypes.RemoveOptions{Force: true}).Return(nil)

	client := dockerClient{containerAPI: engineClient, imageAPI: engineClient}
	err := client.runSidecar(context.TODO(), c, [][]string{{"test", "one"}, {"test", "two"}}, "pumba/tcimage", "tc", true)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestRemoveSidecarTreatsRemovingAfterDeadlineAsSuccess(t *testing.T) {
	engineClient := NewMockEngine(t)
	engineClient.EXPECT().ContainerRemove(mock.Anything, "sidecarID", ctypes.RemoveOptions{Force: true}).Return(context.DeadlineExceeded)
	engineClient.EXPECT().ContainerInspect(mock.Anything, "sidecarID").Return(ctypes.InspectResponse{
		ContainerJSONBase: &ctypes.ContainerJSONBase{State: &ctypes.State{Status: "removing"}},
	}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.removeSidecar(context.TODO(), "sidecarID")

	assert.NoError(t, err)
}

func TestRemoveSidecarKeepsDeadlineErrorWhenContainerStillRunning(t *testing.T) {
	engineClient := NewMockEngine(t)
	engineClient.EXPECT().ContainerRemove(mock.Anything, "sidecarID", ctypes.RemoveOptions{Force: true}).Return(context.DeadlineExceeded)
	engineClient.EXPECT().ContainerInspect(mock.Anything, "sidecarID").Return(ctypes.InspectResponse{
		ContainerJSONBase: &ctypes.ContainerJSONBase{State: &ctypes.State{Status: "running", Running: true}},
	}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.removeSidecar(context.TODO(), "sidecarID")

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
