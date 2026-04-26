package docker

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// fakeExecAttach returns a HijackedResponse suitable for mocking
// ContainerExecAttach in tests that don't care about the exec stream output.
func fakeExecAttach() types.HijackedResponse {
	conn := &mockConn{}
	conn.On("Close").Return(nil)
	return types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(strings.NewReader("")),
	}
}

func NewMockEngine() *mocks.APIClient {
	return new(mocks.APIClient)
}

func mockAllContainers(_ *ctr.Container) bool {
	return true
}
func mockNoContainers(_ *ctr.Container) bool {
	return false
}

func TestNetemContainer_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	engineClient := NewMockEngine()

	checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}
	engineClient.EXPECT().ContainerExecCreate(mock.Anything, "abc123", checkConfig).Return(ctypes.ExecCreateResponse{ID: "checkID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(mock.Anything, "checkID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(mock.Anything, "checkID").Return(ctypes.ExecInspect{}, nil)

	config := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay", "500ms"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(mock.Anything, "abc123", config).Return(ctypes.ExecCreateResponse{ID: "testID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(mock.Anything, "testID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(mock.Anything, "testID").Return(ctypes.ExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, nil, nil, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestStopNetemContainer_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", checkConfig).Return(ctypes.ExecCreateResponse{ID: "checkID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "checkID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "checkID").Return(ctypes.ExecInspect{}, nil)

	stopConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "del", "dev", "eth0", "root", "netem"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", stopConfig).Return(ctypes.ExecCreateResponse{ID: "testID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "testID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "testID").Return(ctypes.ExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.StopNetemContainer(context.TODO(), c, "eth0", nil, nil, nil, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainer_DryRun(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	engineClient := NewMockEngine()
	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, nil, nil, 1*time.Millisecond, "", false, true)

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerExecCreate", mock.Anything)
	engineClient.AssertNotCalled(t, "ContainerExecAttach", "abc123", mock.Anything)
}

func TestNetemContainerIPFilter_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", checkConfig).Return(ctypes.ExecCreateResponse{ID: "checkID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "checkID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "checkID").Return(ctypes.ExecInspect{}, nil)

	config1 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "handle", "1:", "prio"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config1).Return(ctypes.ExecCreateResponse{ID: "cmd1"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd1", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd1").Return(ctypes.ExecInspect{}, nil)

	config2 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:1", "handle", "10:", "sfq"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config2).Return(ctypes.ExecCreateResponse{ID: "cmd2"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd2", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd2").Return(ctypes.ExecInspect{}, nil)

	config3 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:2", "handle", "20:", "sfq"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config3).Return(ctypes.ExecCreateResponse{ID: "cmd3"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd3", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd3").Return(ctypes.ExecInspect{}, nil)

	config4 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:3", "handle", "30:", "netem", "delay", "500ms"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config4).Return(ctypes.ExecCreateResponse{ID: "cmd4"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd4", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd4").Return(ctypes.ExecInspect{}, nil)

	config5 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "filter", "add", "dev", "eth0", "protocol", "ip",
		"parent", "1:0", "prio", "1", "u32", "match", "ip", "dst", "10.10.0.1/32", "flowid", "1:3"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config5).Return(ctypes.ExecCreateResponse{ID: "cmd5"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd5", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd5").Return(ctypes.ExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, []*net.IPNet{{IP: net.IP{10, 10, 0, 1}, Mask: net.IPMask{255, 255, 255, 255}}}, nil, nil, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainerSportFilter_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", checkConfig).Return(ctypes.ExecCreateResponse{ID: "checkID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "checkID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "checkID").Return(ctypes.ExecInspect{}, nil)

	config1 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "handle", "1:", "prio"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config1).Return(ctypes.ExecCreateResponse{ID: "cmd1"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd1", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd1").Return(ctypes.ExecInspect{}, nil)

	config2 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:1", "handle", "10:", "sfq"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config2).Return(ctypes.ExecCreateResponse{ID: "cmd2"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd2", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd2").Return(ctypes.ExecInspect{}, nil)

	config3 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:2", "handle", "20:", "sfq"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config3).Return(ctypes.ExecCreateResponse{ID: "cmd3"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd3", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd3").Return(ctypes.ExecInspect{}, nil)

	config4 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:3", "handle", "30:", "netem", "delay", "500ms"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config4).Return(ctypes.ExecCreateResponse{ID: "cmd4"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd4", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd4").Return(ctypes.ExecInspect{}, nil)

	config5 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "filter", "add", "dev", "eth0", "protocol", "ip",
		"parent", "1:0", "prio", "1", "u32", "match", "ip", "sport", "1234", "0xffff", "flowid", "1:3"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config5).Return(ctypes.ExecCreateResponse{ID: "cmd5"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd5", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd5").Return(ctypes.ExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, []string{"1234"}, nil, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainerDportFilter_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	ctx := mock.Anything
	engineClient := NewMockEngine()

	checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", checkConfig).Return(ctypes.ExecCreateResponse{ID: "checkID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "checkID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "checkID").Return(ctypes.ExecInspect{}, nil)

	config1 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "handle", "1:", "prio"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config1).Return(ctypes.ExecCreateResponse{ID: "cmd1"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd1", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd1").Return(ctypes.ExecInspect{}, nil)

	config2 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:1", "handle", "10:", "sfq"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config2).Return(ctypes.ExecCreateResponse{ID: "cmd2"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd2", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd2").Return(ctypes.ExecInspect{}, nil)

	config3 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:2", "handle", "20:", "sfq"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config3).Return(ctypes.ExecCreateResponse{ID: "cmd3"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd3", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd3").Return(ctypes.ExecInspect{}, nil)

	config4 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:3", "handle", "30:", "netem", "delay", "500ms"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config4).Return(ctypes.ExecCreateResponse{ID: "cmd4"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd4", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd4").Return(ctypes.ExecInspect{}, nil)

	config5 := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "filter", "add", "dev", "eth0", "protocol", "ip",
		"parent", "1:0", "prio", "1", "u32", "match", "ip", "dport", "1234", "0xffff", "flowid", "1:3"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", config5).Return(ctypes.ExecCreateResponse{ID: "cmd5"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "cmd5", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "cmd5").Return(ctypes.ExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), c, "eth0", []string{"delay", "500ms"}, nil, nil, []string{"1234"}, 1*time.Millisecond, "", false, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

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
	engineClient := NewMockEngine()

	engineClient.EXPECT().ImagePull(ctx, config.Image, imagetypes.PullOptions{}).Return(io.NopCloser(readerResponse), nil)
	engineClient.EXPECT().ContainerCreate(ctx, &config, &hconfig, (*network.NetworkingConfig)(nil), (*specs.Platform)(nil), "").Return(ctypes.CreateResponse{ID: "tcID"}, nil)
	engineClient.EXPECT().ContainerStart(ctx, "tcID", ctypes.StartOptions{}).Return(nil)
	engineClient.EXPECT().ContainerExecCreate(ctx, "tcID", ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "test", "one"}}).Return(ctypes.ExecCreateResponse{ID: "execID1"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "execID1", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecCreate(ctx, "tcID", ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "test", "two"}}).Return(ctypes.ExecCreateResponse{ID: "execID2"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "execID2", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerRemove(ctx, "tcID", ctypes.RemoveOptions{Force: true}).Return(nil)

	client := dockerClient{containerAPI: engineClient, imageAPI: engineClient}
	err := client.tcContainerCommands(context.TODO(), c, [][]string{{"test", "one"}, {"test", "two"}}, "pumba/tcimage", true)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
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
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
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
				engine.EXPECT().ImagePull(ctx, image, imagetypes.PullOptions{}).Return(io.NopCloser(readerResponse), nil)
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(nil)
				inspect := ctypes.InspectResponse{
					ContainerJSONBase: &ctypes.ContainerJSONBase{
						State: &ctypes.State{},
					},
				}
				conn.On("Close").Return(nil)
				engine.EXPECT().ContainerInspect(ctx, "000").Return(inspect, nil)
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
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ImagePull(ctx, image, imagetypes.PullOptions{}).Return(io.NopCloser(bytes.NewReader([]byte("{}"))), errors.New("failed to pull image"))
			},
			wantErr: true,
		},
		{
			name: "stress test without pull image",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(nil)
				inspect := ctypes.InspectResponse{
					ContainerJSONBase: &ctypes.ContainerJSONBase{
						State: &ctypes.State{},
					},
				}
				engine.EXPECT().ContainerInspect(ctx, "000").Return(inspect, nil)
				conn.On("Close").Return(nil)
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
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(nil)
				inspect := ctypes.InspectResponse{
					ContainerJSONBase: &ctypes.ContainerJSONBase{
						State: &ctypes.State{ExitCode: 1},
					},
				}
				conn.On("Close").Return(nil)
				engine.EXPECT().ContainerInspect(ctx, "000").Return(inspect, nil)
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
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(nil)
				conn.On("Close").Return(nil)
				engine.EXPECT().ContainerInspect(ctx, "000").Return(ctypes.InspectResponse{}, errors.New("filed to inspect"))
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
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(errors.New("failed to start"))
				// goroutine may or may not complete before test exits
				conn.On("Close").Return(nil).Maybe()
				inspect := ctypes.InspectResponse{
					ContainerJSONBase: &ctypes.ContainerJSONBase{
						State: &ctypes.State{},
					},
				}
				engine.EXPECT().ContainerInspect(ctx, "000").Return(inspect, nil).Maybe()
			},
			want:    "000",
			wantErr: true,
		},
		{
			name: "fail to attach to stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{}, errors.New("failed to attach"))
			},
			wantErr: true,
		},
		{
			name: "fail to create to stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{}, errors.New("failed to create"))
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
				systemAPI:    mockClient,
			}
			// init mock engine
			tt.mockInit(tt.args.ctx, mockClient, mConn, tt.args.targetID, tt.args.stressors, tt.args.image, tt.args.pull)
			// test stress command
			got, got1, got2, err := client.stressContainerCommand(tt.args.ctx, tt.args.targetID, tt.args.stressors, tt.args.image, tt.args.pull, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.stressContainerCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("dockerClient.stressContainerCommand() got = %v, want %v", got, tt.want)
			}
			// only read from channels when no direct error (channels are nil on error path)
			if err == nil && (got1 != nil || got2 != nil) {
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
			}
			mockClient.AssertExpectations(t)
			mConn.AssertExpectations(t)
		})
	}
}

// Test for IPTables functionality
func TestIPTablesContainer(t *testing.T) {
	type args struct {
		ctx       context.Context
		c         *ctr.Container
		cmdPrefix []string
		cmdSuffix []string
		srcIPs    []*net.IPNet
		dstIPs    []*net.IPNet
		sports    []string
		dports    []string
		duration  time.Duration
		image     string
		pull      bool
		dryrun    bool
	}
	tests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, context.Context, *ctr.Container, []string, []string, []*net.IPNet, []*net.IPNet, []string, []string, string, bool, bool)
		wantErr bool
	}{
		{
			name: "iptables with dry run",
			args: args{
				ctx:       context.TODO(),
				c:         &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				cmdPrefix: []string{"-A", "INPUT"},
				cmdSuffix: []string{"-j", "DROP"},
				dryrun:    true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, image string, pull, dryrun bool) {
				// No calls expected in dry run mode
			},
			wantErr: false,
		},
		{
			name: "simple iptables command without IP filters",
			args: args{
				ctx:       context.TODO(),
				c:         &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				cmdPrefix: []string{"-A", "INPUT"},
				cmdSuffix: []string{"-j", "DROP"},
				image:     "",
				dryrun:    false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, image string, pull, dryrun bool) {
				// The container has iptables installed, so we execute directly
				cmdArgs := append(cmdPrefix, cmdSuffix...)
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "iptables"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"iptables"}, cmdArgs...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{}, nil)
			},
			wantErr: false,
		},
		{
			name: "iptables with source IPs",
			args: args{
				ctx:       context.TODO(),
				c:         &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				cmdPrefix: []string{"-A", "INPUT"},
				cmdSuffix: []string{"-j", "DROP"},
				srcIPs:    []*net.IPNet{{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(32, 32)}},
				dryrun:    false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, image string, pull, dryrun bool) {
				// The container has iptables installed, so we execute directly
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "iptables"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

				cmdArgs := append(append([]string{}, cmdPrefix...), "-s", "10.0.0.1/32")
				cmdArgs = append(cmdArgs, cmdSuffix...)
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"iptables"}, cmdArgs...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{}, nil)
			},
			wantErr: false,
		},
		{
			name: "iptables with destination ports",
			args: args{
				ctx:       context.TODO(),
				c:         &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				cmdPrefix: []string{"-A", "INPUT"},
				cmdSuffix: []string{"-j", "DROP"},
				dports:    []string{"80", "443"},
				dryrun:    false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, image string, pull, dryrun bool) {
				// The container has iptables installed, so we execute directly
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "iptables"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

				// Expect two commands - one for each port
				for _, dport := range dports {
					cmdArgs := append(append([]string{}, cmdPrefix...), "--dport", dport)
					cmdArgs = append(cmdArgs, cmdSuffix...)
					api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"iptables"}, cmdArgs...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID-" + dport}, nil)
					api.EXPECT().ContainerExecAttach(ctx, "execID-"+dport, ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
					api.EXPECT().ContainerExecInspect(ctx, "execID-"+dport).Return(ctypes.ExecInspect{}, nil)
				}
			},
			wantErr: false,
		},
		{
			name: "iptables execution failure",
			args: args{
				ctx:       context.TODO(),
				c:         &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				cmdPrefix: []string{"-A", "INPUT"},
				cmdSuffix: []string{"-j", "DROP"},
				dryrun:    false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, image string, pull, dryrun bool) {
				cmdArgs := append(cmdPrefix, cmdSuffix...)
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "iptables"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"iptables"}, cmdArgs...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{ExitCode: 1}, nil) // Exit code 1 indicates command failure
			},
			wantErr: true,
		},
		{
			name: "iptables not installed in container",
			args: args{
				ctx:       context.TODO(),
				c:         &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				cmdPrefix: []string{"-A", "INPUT"},
				cmdSuffix: []string{"-j", "DROP"},
				dryrun:    false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, image string, pull, dryrun bool) {
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "iptables"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{ExitCode: 1}, nil) // Exit code 1 indicates command not found
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine()
			// Set up the mock expectations
			tt.mockSet(api, tt.args.ctx, tt.args.c, tt.args.cmdPrefix, tt.args.cmdSuffix, tt.args.srcIPs, tt.args.dstIPs, tt.args.sports, tt.args.dports, tt.args.image, tt.args.pull, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.IPTablesContainer(tt.args.ctx, tt.args.c, tt.args.cmdPrefix, tt.args.cmdSuffix, tt.args.srcIPs, tt.args.dstIPs, tt.args.sports, tt.args.dports, tt.args.duration, tt.args.image, tt.args.pull, tt.args.dryrun)

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.IPTablesContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

// Test for NetemContainer functionality
func TestNetemContainer(t *testing.T) {
	type args struct {
		ctx          context.Context
		c            *ctr.Container
		netInterface string
		netemCmd     []string
		ips          []*net.IPNet
		sports       []string
		dports       []string
		duration     time.Duration
		tcimage      string
		pull         bool
		dryrun       bool
	}
	tests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, context.Context, *ctr.Container, string, []string, []*net.IPNet, []string, []string, string, bool, bool)
		wantErr bool
	}{
		{
			name: "netem with dry run",
			args: args{
				ctx:          context.TODO(),
				c:            &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				netInterface: "eth0",
				netemCmd:     []string{"delay", "100ms"},
				dryrun:       true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) {
				// No calls expected in dry run mode
			},
			wantErr: false,
		},
		{
			name: "netem delay without filters",
			args: args{
				ctx:          context.TODO(),
				c:            &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				netInterface: "eth0",
				netemCmd:     []string{"delay", "100ms"},
				dryrun:       false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) {
				// The container has tc installed, so we execute directly
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

				// Call with the tc command
				tcCmd := []string{"qdisc", "add", "dev", netInterface, "root", "netem", "delay", "100ms"}
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"tc"}, tcCmd...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{}, nil)
			},
			wantErr: false,
		},
		{
			name: "netem with ip filter",
			args: args{
				ctx:          context.TODO(),
				c:            &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				netInterface: "eth0",
				netemCmd:     []string{"delay", "100ms"},
				ips:          []*net.IPNet{{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(32, 32)}},
				dryrun:       false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) {
				// The container has tc installed, so we execute directly
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

				// With IP filter, need multiple tc commands
				// First command - create priority qdisc
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{
					AttachStdout: true,
					AttachStderr: true,
					Cmd:          []string{"tc", "qdisc", "add", "dev", netInterface, "root", "handle", "1:", "prio"},
					Privileged:   true,
				}).Return(ctypes.ExecCreateResponse{ID: "cmd1"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "cmd1", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "cmd1").Return(ctypes.ExecInspect{}, nil)

				// Second command - add sfq qdisc for first class
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{
					AttachStdout: true,
					AttachStderr: true,
					Cmd:          []string{"tc", "qdisc", "add", "dev", netInterface, "parent", "1:1", "handle", "10:", "sfq"},
					Privileged:   true,
				}).Return(ctypes.ExecCreateResponse{ID: "cmd2"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "cmd2", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "cmd2").Return(ctypes.ExecInspect{}, nil)

				// Third command - add sfq qdisc for second class
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{
					AttachStdout: true,
					AttachStderr: true,
					Cmd:          []string{"tc", "qdisc", "add", "dev", netInterface, "parent", "1:2", "handle", "20:", "sfq"},
					Privileged:   true,
				}).Return(ctypes.ExecCreateResponse{ID: "cmd3"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "cmd3", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "cmd3").Return(ctypes.ExecInspect{}, nil)

				// Fourth command - add netem qdisc for third class
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{
					AttachStdout: true,
					AttachStderr: true,
					Cmd:          []string{"tc", "qdisc", "add", "dev", netInterface, "parent", "1:3", "handle", "30:", "netem", "delay", "100ms"},
					Privileged:   true,
				}).Return(ctypes.ExecCreateResponse{ID: "cmd4"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "cmd4", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "cmd4").Return(ctypes.ExecInspect{}, nil)

				// Fifth command - add filter for IP
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{
					AttachStdout: true,
					AttachStderr: true,
					Cmd: []string{"tc", "filter", "add", "dev", netInterface, "protocol", "ip",
						"parent", "1:0", "prio", "1", "u32", "match", "ip", "dst", "10.0.0.1/32", "flowid", "1:3"},
					Privileged: true,
				}).Return(ctypes.ExecCreateResponse{ID: "cmd5"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "cmd5", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "cmd5").Return(ctypes.ExecInspect{}, nil)
			},
			wantErr: false,
		},
		{
			name: "tc not installed",
			args: args{
				ctx:          context.TODO(),
				c:            &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				netInterface: "eth0",
				netemCmd:     []string{"delay", "100ms"},
				dryrun:       false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) {
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{ExitCode: 1}, nil) // Exit code 1 indicates command not found
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine()
			// Set up the mock expectations
			tt.mockSet(api, tt.args.ctx, tt.args.c, tt.args.netInterface, tt.args.netemCmd, tt.args.ips, tt.args.sports, tt.args.dports, tt.args.tcimage, tt.args.pull, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.NetemContainer(tt.args.ctx, tt.args.c, tt.args.netInterface, tt.args.netemCmd, tt.args.ips, tt.args.sports, tt.args.dports, tt.args.duration, tt.args.tcimage, tt.args.pull, tt.args.dryrun)

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.NetemContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

// Test for Stop, StopNetem, and StopIPTables functionality
func TestStopNetemIPTables(t *testing.T) {
	type stopNetemArgs struct {
		ctx          context.Context
		c            *ctr.Container
		netInterface string
		ip           []*net.IPNet
		sports       []string
		dports       []string
		tcimage      string
		pull         bool
		dryrun       bool
	}

	stopNetemTests := []struct {
		name    string
		args    stopNetemArgs
		mockSet func(*mocks.APIClient, context.Context, *ctr.Container, string, []*net.IPNet, []string, []string, string, bool, bool)
		wantErr bool
	}{
		{
			name: "stop netem without filters dry run",
			args: stopNetemArgs{
				ctx:          context.TODO(),
				c:            &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				netInterface: "eth0",
				dryrun:       true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, netInterface string, ip []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) {
				// No calls expected in dry run mode
			},
			wantErr: false,
		},
		{
			name: "stop netem without filters",
			args: stopNetemArgs{
				ctx:          context.TODO(),
				c:            &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				netInterface: "eth0",
				dryrun:       false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, netInterface string, ip []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) {
				// Simple case - just remove the root qdisc
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

				// Expect tc qdisc del command
				tcCmd := []string{"qdisc", "del", "dev", netInterface, "root", "netem"}
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"tc"}, tcCmd...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{}, nil)
			},
			wantErr: false,
		},
		{
			name: "stop netem with IP filters",
			args: stopNetemArgs{
				ctx:          context.TODO(),
				c:            &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				netInterface: "eth0",
				ip:           []*net.IPNet{{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(32, 32)}},
				dryrun:       false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, netInterface string, ip []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) {
				// With IP filters - need to remove all parent qdiscs
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

				// Need to remove child qdiscs first
				tcCmd1 := []string{"qdisc", "del", "dev", netInterface, "parent", "1:1", "handle", "10:"}
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"tc"}, tcCmd1...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID1"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID1", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID1").Return(ctypes.ExecInspect{}, nil)

				tcCmd2 := []string{"qdisc", "del", "dev", netInterface, "parent", "1:2", "handle", "20:"}
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"tc"}, tcCmd2...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID2"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID2", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID2").Return(ctypes.ExecInspect{}, nil)

				tcCmd3 := []string{"qdisc", "del", "dev", netInterface, "parent", "1:3", "handle", "30:"}
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"tc"}, tcCmd3...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID3"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID3", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID3").Return(ctypes.ExecInspect{}, nil)

				// Finally remove the root qdisc
				tcCmd4 := []string{"qdisc", "del", "dev", netInterface, "root", "handle", "1:", "prio"}
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"tc"}, tcCmd4...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID4"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID4", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID4").Return(ctypes.ExecInspect{}, nil)
			},
			wantErr: false,
		},
		{
			name: "stop netem command error",
			args: stopNetemArgs{
				ctx:          context.TODO(),
				c:            &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				netInterface: "eth0",
				dryrun:       false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, netInterface string, ip []*net.IPNet, sports, dports []string, tcimage string, pull, dryrun bool) {
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

				// Command execution fails
				tcCmd := []string{"qdisc", "del", "dev", netInterface, "root", "netem"}
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{"tc"}, tcCmd...), Privileged: true}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{ExitCode: 1}, nil) // Exit code 1 indicates failure
			},
			wantErr: true,
		},
	}

	for _, tt := range stopNetemTests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine()
			// Set up the mock expectations
			tt.mockSet(api, tt.args.ctx, tt.args.c, tt.args.netInterface, tt.args.ip, tt.args.sports, tt.args.dports, tt.args.tcimage, tt.args.pull, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.StopNetemContainer(tt.args.ctx, tt.args.c, tt.args.netInterface, tt.args.ip, tt.args.sports, tt.args.dports, tt.args.tcimage, tt.args.pull, tt.args.dryrun)

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.StopNetemContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

func TestHTTPClient(t *testing.T) {
	tests := []struct {
		name      string
		daemonURL string
		tlsConfig *tls.Config
		wantErr   bool
	}{
		{
			name:      "tcp url with no TLS",
			daemonURL: "tcp://localhost:2375",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name:      "http url with no TLS",
			daemonURL: "http://localhost:2375",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name:      "tcp url with TLS",
			daemonURL: "tcp://localhost:2376",
			tlsConfig: &tls.Config{},
			wantErr:   false,
		},
		{
			name:      "https url with TLS",
			daemonURL: "https://localhost:2376",
			tlsConfig: &tls.Config{},
			wantErr:   false,
		},
		{
			name:      "unix socket",
			daemonURL: "unix:///var/run/docker.sock",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name:      "invalid URL",
			daemonURL: "://invalid-url",
			tlsConfig: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := HTTPClient(tt.daemonURL, tt.tlsConfig)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				// Check that the client has appropriate transport
				transport, ok := client.Transport.(*http.Transport)
				assert.True(t, ok)

				// Check TLS configuration
				if tt.tlsConfig != nil {
					assert.Equal(t, tt.tlsConfig, transport.TLSClientConfig)
				}

				// For unix socket, check if the dial function is set
				if tt.daemonURL != "" && strings.HasPrefix(tt.daemonURL, "unix:") {
					assert.NotNil(t, transport.DialContext)
				}
			}
		})
	}
}

func TestNewHTTPClient(t *testing.T) {
	// Create test cases for different URL schemes
	tests := []struct {
		name    string
		address *url.URL
		tlsConf *tls.Config
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "http scheme",
			address: &url.URL{Scheme: "http", Host: "localhost:2375"},
			tlsConf: nil,
			timeout: 10 * time.Second,
			wantErr: false,
		},
		{
			name:    "https scheme with TLS",
			address: &url.URL{Scheme: "https", Host: "localhost:2376"},
			tlsConf: &tls.Config{InsecureSkipVerify: true},
			timeout: 10 * time.Second,
			wantErr: false,
		},
		{
			name:    "unix scheme",
			address: &url.URL{Scheme: "unix", Path: "/var/run/docker.sock"},
			tlsConf: nil,
			timeout: 10 * time.Second,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newHTTPClient(tt.address, tt.tlsConf, tt.timeout)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				// Check that the client has appropriate transport
				transport, ok := client.Transport.(*http.Transport)
				assert.True(t, ok)

				// Check TLS configuration
				if tt.tlsConf != nil {
					assert.Equal(t, tt.tlsConf, transport.TLSClientConfig)
				}

				// Check DialContext is set
				assert.NotNil(t, transport.DialContext)

				// For unix scheme, check if the URL was transformed
				if tt.address.Scheme == "unix" {
					assert.Equal(t, "http", tt.address.Scheme)
					assert.Equal(t, "unix.sock", tt.address.Host)
					assert.Equal(t, "", tt.address.Path)
				}
			}
		})
	}
}

func TestIPTablesExecCommandWithRealDocker(t *testing.T) {
	t.Skip("This test requires a Docker daemon to run properly")
}

func TestIPTablesForSimpleCases(t *testing.T) {
	// Test for StopIPTablesContainer in dry-run mode
	api := NewMockEngine()
	client := dockerClient{containerAPI: api, imageAPI: api}
	ctx := context.Background()
	container := &ctr.Container{
		ContainerID: "container123", ContainerName: "test-container",
	}

	// Test dry-run mode (no iptables commands will be executed)
	err := client.StopIPTablesContainer(ctx, container, []string{"-A", "INPUT"}, []string{"-j", "DROP"}, nil, nil, nil, nil, "", false, true)
	assert.NoError(t, err, "StopIPTablesContainer in dry-run mode should not return error")

	// Test ipTablesExecCommand
	t.Run("ipTablesExecCommand_integration", func(t *testing.T) {
		if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
			t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
		}

		api := NewMockEngine()
		api.EXPECT().ContainerExecCreate(mock.Anything, mock.Anything, mock.Anything).Return(ctypes.ExecCreateResponse{ID: "exec-id"}, nil)
		api.EXPECT().ContainerExecAttach(mock.Anything, mock.Anything, mock.Anything).Return(fakeExecAttach(), nil)

		client := dockerClient{containerAPI: api}
		err := client.ipTablesExecCommand(ctx, "container-id", []string{"-L"})
		assert.NoError(t, err)
	})
}
