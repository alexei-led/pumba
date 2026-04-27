package docker

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", "iptables"}}).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				api.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)

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
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{ExitCode: 1}, nil)
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
				api.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{ExitCode: 1}, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			tt.mockSet(api, tt.args.ctx, tt.args.c, tt.args.cmdPrefix, tt.args.cmdSuffix, tt.args.srcIPs, tt.args.dstIPs, tt.args.sports, tt.args.dports, tt.args.image, tt.args.pull, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.IPTablesContainer(tt.args.ctx, &ctr.IPTablesRequest{
				Container: tt.args.c,
				CmdPrefix: tt.args.cmdPrefix,
				CmdSuffix: tt.args.cmdSuffix,
				SrcIPs:    tt.args.srcIPs,
				DstIPs:    tt.args.dstIPs,
				SPorts:    tt.args.sports,
				DPorts:    tt.args.dports,
				Duration:  tt.args.duration,
				Sidecar:   ctr.SidecarSpec{Image: tt.args.image, Pull: tt.args.pull},
				DryRun:    tt.args.dryrun,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.IPTablesContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}

func TestIPTablesForSimpleCases(t *testing.T) {
	api := NewMockEngine(t)
	client := dockerClient{containerAPI: api, imageAPI: api}
	ctx := context.Background()
	container := &ctr.Container{
		ContainerID: "container123", ContainerName: "test-container",
	}

	// Dry-run mode: no iptables commands expected
	err := client.StopIPTablesContainer(ctx, &ctr.IPTablesRequest{
		Container: container,
		CmdPrefix: []string{"-A", "INPUT"},
		CmdSuffix: []string{"-j", "DROP"},
		DryRun:    true,
	})
	assert.NoError(t, err, "StopIPTablesContainer in dry-run mode should not return error")

	t.Run("ipTablesExecCommand_integration", func(t *testing.T) {
		if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
			t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
		}

		api := NewMockEngine(t)
		api.EXPECT().ContainerExecCreate(mock.Anything, mock.Anything, mock.Anything).Return(ctypes.ExecCreateResponse{ID: "exec-id"}, nil)
		api.EXPECT().ContainerExecAttach(mock.Anything, mock.Anything, mock.Anything).Return(fakeExecAttach(), nil)
		api.EXPECT().ContainerExecInspect(mock.Anything, "exec-id").Return(ctypes.ExecInspect{}, nil)

		client := dockerClient{containerAPI: api}
		err := client.runSidecarExec(ctx, "container-id", "iptables", []string{"-L"})
		assert.NoError(t, err)
	})
}
