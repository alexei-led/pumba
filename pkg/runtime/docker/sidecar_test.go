package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/stretchr/testify/require"
)

func Test_tcContainerCommands(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "targetID",
	}

	config := ctypes.Config{
		Labels: map[string]string{
			"com.gaiaadm.pumba.skip":   "true",
			"com.gaiaadm.pumba.run-id": "run1",
			"com.gaiaadm.pumba.target": "targetID",
			"com.gaiaadm.pumba.kind":   "netem",
		},
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
	engineClient.EXPECT().ContainerExecAttach(mock.Anything, "execID1", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "execID1").Return(ctypes.ExecInspect{}, nil)
	engineClient.EXPECT().ContainerExecCreate(ctx, "tcID", ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "test", "two"}}).Return(ctypes.ExecCreateResponse{ID: "execID2"}, nil)
	engineClient.EXPECT().ContainerExecAttach(mock.Anything, "execID2", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "execID2").Return(ctypes.ExecInspect{}, nil)
	engineClient.EXPECT().ContainerRemove(ctx, "tcID", ctypes.RemoveOptions{Force: true}).Return(nil)

	client := dockerClient{containerAPI: engineClient, imageAPI: engineClient}
	labels := ctr.SidecarLabels("run1", "targetID", ctr.SidecarKindNetem)
	err := client.runSidecar(context.TODO(), c, ctr.SidecarSpec{Image: "pumba/tcimage", Pull: true}, labels, "tc", [][]string{{"test", "one"}, {"test", "two"}})

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

func TestWithSidecarRetainsOperationAndRemovalFailures(t *testing.T) {
	for _, stage := range []string{"start", "operation"} {
		t.Run(stage, func(t *testing.T) {
			api := NewMockEngine(t)
			operationErr, cleanupErr := errors.New("sidecar operation failed"), errors.New("sidecar removal failed")
			api.EXPECT().ContainerCreate(mock.Anything,
				&ctypes.Config{Entrypoint: []string{"tail"}, Cmd: []string{"-f", "/dev/null"}, Image: "nettools", StopSignal: "SIGKILL"},
				&ctypes.HostConfig{CapAdd: []string{"NET_ADMIN"}, NetworkMode: "container:target", PortBindings: nat.PortMap{}, DNS: []string{}, DNSOptions: []string{}, DNSSearch: []string{}},
				(*network.NetworkingConfig)(nil), (*specs.Platform)(nil), "",
			).Return(ctypes.CreateResponse{ID: "owned-sidecar"}, nil).Once()
			var startErr error
			if stage == "start" {
				startErr = operationErr
			}
			api.EXPECT().ContainerStart(mock.Anything, "owned-sidecar", ctypes.StartOptions{}).Return(startErr).Once()
			api.EXPECT().ContainerRemove(mock.Anything, "owned-sidecar", ctypes.RemoveOptions{Force: true}).Return(cleanupErr).Once()
			client := dockerClient{containerAPI: api}
			err := client.withSidecar(context.Background(), &ctr.Container{ContainerID: "target"}, ctr.SidecarSpec{Image: "nettools"}, nil, "test", func(string) error { require.Equal(t, "operation", stage); return operationErr })
			require.ErrorIs(t, err, operationErr)
			require.ErrorIs(t, err, cleanupErr)
		})
	}
}
