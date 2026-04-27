package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNetemContainer_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	engineClient := NewMockEngine(t)

	checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}
	engineClient.EXPECT().ContainerExecCreate(mock.Anything, "abc123", checkConfig).Return(ctypes.ExecCreateResponse{ID: "checkID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(mock.Anything, "checkID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(mock.Anything, "checkID").Return(ctypes.ExecInspect{}, nil)

	config := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay", "500ms"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(mock.Anything, "abc123", config).Return(ctypes.ExecCreateResponse{ID: "testID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(mock.Anything, "testID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(mock.Anything, "testID").Return(ctypes.ExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), &ctr.NetemRequest{
		Container: c,
		Interface: "eth0",
		Command:   []string{"delay", "500ms"},
		Duration:  1 * time.Millisecond,
	})

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestStopNetemContainer_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	ctx := mock.Anything
	engineClient := NewMockEngine(t)

	checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "tc"}}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", checkConfig).Return(ctypes.ExecCreateResponse{ID: "checkID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "checkID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "checkID").Return(ctypes.ExecInspect{}, nil)

	stopConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"tc", "qdisc", "del", "dev", "eth0", "root", "netem"}, Privileged: true}
	engineClient.EXPECT().ContainerExecCreate(ctx, "abc123", stopConfig).Return(ctypes.ExecCreateResponse{ID: "testID"}, nil)
	engineClient.EXPECT().ContainerExecAttach(ctx, "testID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
	engineClient.EXPECT().ContainerExecInspect(ctx, "testID").Return(ctypes.ExecInspect{}, nil)

	client := dockerClient{containerAPI: engineClient}
	err := client.StopNetemContainer(context.TODO(), &ctr.NetemRequest{
		Container: c,
		Interface: "eth0",
	})

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainer_DryRun(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	engineClient := NewMockEngine(t)
	client := dockerClient{containerAPI: engineClient}
	err := client.NetemContainer(context.TODO(), &ctr.NetemRequest{
		Container: c,
		Interface: "eth0",
		Command:   []string{"delay", "500ms"},
		Duration:  1 * time.Millisecond,
		DryRun:    true,
	})

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerExecCreate", mock.Anything)
	engineClient.AssertNotCalled(t, "ContainerExecAttach", "abc123", mock.Anything)
}

func TestNetemContainerIPFilter_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	ctx := mock.Anything
	engineClient := NewMockEngine(t)

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
	err := client.NetemContainer(context.TODO(), &ctr.NetemRequest{
		Container: c,
		Interface: "eth0",
		Command:   []string{"delay", "500ms"},
		IPs:       []*net.IPNet{{IP: net.IP{10, 10, 0, 1}, Mask: net.IPMask{255, 255, 255, 255}}},
		Duration:  1 * time.Millisecond,
	})

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainerSportFilter_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	ctx := mock.Anything
	engineClient := NewMockEngine(t)

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
	err := client.NetemContainer(context.TODO(), &ctr.NetemRequest{
		Container: c,
		Interface: "eth0",
		Command:   []string{"delay", "500ms"},
		SPorts:    []string{"1234"},
		Duration:  1 * time.Millisecond,
	})

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainerDportFilter_Success(t *testing.T) {
	c := &ctr.Container{
		ContainerID: "abc123",
	}

	ctx := mock.Anything
	engineClient := NewMockEngine(t)

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
	err := client.NetemContainer(context.TODO(), &ctr.NetemRequest{
		Container: c,
		Interface: "eth0",
		Command:   []string{"delay", "500ms"},
		DPorts:    []string{"1234"},
		Duration:  1 * time.Millisecond,
	})

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
			api := NewMockEngine(t)
			// Set up the mock expectations
			tt.mockSet(api, tt.args.ctx, tt.args.c, tt.args.netInterface, tt.args.netemCmd, tt.args.ips, tt.args.sports, tt.args.dports, tt.args.tcimage, tt.args.pull, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.NetemContainer(tt.args.ctx, &ctr.NetemRequest{
				Container: tt.args.c,
				Interface: tt.args.netInterface,
				Command:   tt.args.netemCmd,
				IPs:       tt.args.ips,
				SPorts:    tt.args.sports,
				DPorts:    tt.args.dports,
				Duration:  tt.args.duration,
				Sidecar:   ctr.SidecarSpec{Image: tt.args.tcimage, Pull: tt.args.pull},
				DryRun:    tt.args.dryrun,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.NetemContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

// Test for StopNetem functionality
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
			api := NewMockEngine(t)
			// Set up the mock expectations
			tt.mockSet(api, tt.args.ctx, tt.args.c, tt.args.netInterface, tt.args.ip, tt.args.sports, tt.args.dports, tt.args.tcimage, tt.args.pull, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.StopNetemContainer(tt.args.ctx, &ctr.NetemRequest{
				Container: tt.args.c,
				Interface: tt.args.netInterface,
				IPs:       tt.args.ip,
				SPorts:    tt.args.sports,
				DPorts:    tt.args.dports,
				Sidecar:   ctr.SidecarSpec{Image: tt.args.tcimage, Pull: tt.args.pull},
				DryRun:    tt.args.dryrun,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.StopNetemContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}
