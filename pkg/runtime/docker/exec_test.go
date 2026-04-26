package docker

import (
	"bufio"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
)

func Test_dockerClient_execOnContainer(t *testing.T) {
	type args struct {
		c          *ctr.Container
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
				c:        &ctr.Container{ContainerID: "abc123"},
				ctx:      context.TODO(),
				execCmd:  "test-app",
				execArgs: []string{"one", "two", "three"},
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", cmd}}
				engine.EXPECT().ContainerExecCreate(ctx, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)
				execConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{cmd}, args...), Privileged: false}
				engine.EXPECT().ContainerExecCreate(ctx, cID, execConfig).Return(ctypes.ExecCreateResponse{ID: "cmdID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "cmdID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(ctx, "cmdID").Return(ctypes.ExecInspect{}, nil)
			},
		},
		{
			name: "run privileged command with args",
			args: args{
				c:          &ctr.Container{ContainerID: "abc123"},
				ctx:        context.TODO(),
				execCmd:    "test-app",
				execArgs:   []string{"one", "two", "three"},
				privileged: true,
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", cmd}}
				engine.EXPECT().ContainerExecCreate(ctx, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)
				execConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{cmd}, args...), Privileged: true}
				engine.EXPECT().ContainerExecCreate(ctx, cID, execConfig).Return(ctypes.ExecCreateResponse{ID: "cmdID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "cmdID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(ctx, "cmdID").Return(ctypes.ExecInspect{}, nil)
			},
		},
		{
			name: "fail to find command",
			args: args{
				c:        &ctr.Container{ContainerID: "abc123"},
				ctx:      context.TODO(),
				execCmd:  "test-app",
				execArgs: []string{"one", "two", "three"},
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", cmd}}
				engine.EXPECT().ContainerExecCreate(ctx, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{ExitCode: 1}, nil)
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecCreate 'which'",
			args: args{
				c:       &ctr.Container{ContainerID: "abc123"},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", cmd}}
				engine.EXPECT().ContainerExecCreate(ctx, cID, checkConfig).Return(ctypes.ExecCreateResponse{}, errors.New("which error"))
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecStart 'which'",
			args: args{
				c:       &ctr.Container{ContainerID: "abc123"},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", cmd}}
				engine.EXPECT().ContainerExecCreate(ctx, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(types.HijackedResponse{}, errors.New("which error"))
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecInspect 'which'",
			args: args{
				c:       &ctr.Container{ContainerID: "abc123"},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", cmd}}
				engine.EXPECT().ContainerExecCreate(ctx, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, errors.New("which error"))
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecCreate 'test-app'",
			args: args{
				c:       &ctr.Container{ContainerID: "abc123"},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", cmd}}
				engine.EXPECT().ContainerExecCreate(ctx, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)
				execConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{cmd}, args...), Privileged: false}
				engine.EXPECT().ContainerExecCreate(ctx, cID, execConfig).Return(ctypes.ExecCreateResponse{}, errors.New("cmd error"))
			},
			wantErr: true,
		},
		{
			name: "fail to ContainerExecStart 'test-app'",
			args: args{
				c:       &ctr.Container{ContainerID: "abc123"},
				ctx:     context.TODO(),
				execCmd: "test-app",
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, cID, cmd string, args []string) {
				checkConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: []string{"which", cmd}}
				engine.EXPECT().ContainerExecCreate(ctx, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(ctx, "whichID").Return(ctypes.ExecInspect{}, nil)
				execConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{cmd}, args...), Privileged: false}
				engine.EXPECT().ContainerExecCreate(ctx, cID, execConfig).Return(ctypes.ExecCreateResponse{ID: "cmdID"}, nil)
				engine.EXPECT().ContainerExecAttach(ctx, "cmdID", ctypes.ExecAttachOptions{}).Return(types.HijackedResponse{}, errors.New("cmd error"))
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
			tt.mockInit(tt.args.ctx, mockClient, tt.args.c.ID(), tt.args.execCmd, tt.args.execArgs)
			err := client.execOnContainer(tt.args.ctx, tt.args.c, tt.args.execCmd, tt.args.execArgs, tt.args.privileged)
			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.execOnContainer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestExecContainer(t *testing.T) {
	type args struct {
		ctx      context.Context
		c        *ctr.Container
		command  string
		execArgs []string
		dryrun   bool
	}
	tests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, context.Context, *ctr.Container, string, []string, bool)
		wantErr bool
	}{
		{
			name: "execute command in container dry run",
			args: args{
				ctx:      context.TODO(),
				c:        &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				command:  "echo",
				execArgs: []string{"hello"},
				dryrun:   true,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, command string, execArgs []string, dryrun bool) {
			},
			wantErr: false,
		},
		{
			name: "execute command in container success",
			args: args{
				ctx:      context.TODO(),
				c:        &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				command:  "echo",
				execArgs: []string{"hello"},
				dryrun:   false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, command string, execArgs []string, dryrun bool) {
				cmdWithArgs := append([]string{command}, execArgs...)
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{
					User:         "root",
					AttachStdout: true,
					AttachStderr: true,
					Cmd:          cmdWithArgs,
				}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				mockReader := strings.NewReader("hello\n")
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(func() types.HijackedResponse {
					conn := &mockConn{}
					conn.On("Close").Return(nil)
					return types.HijackedResponse{
						Conn:   conn,
						Reader: bufio.NewReader(mockReader),
					}
				}(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{ExitCode: 0}, nil)
			},
			wantErr: false,
		},
		{
			name: "execute command with multiple arguments",
			args: args{
				ctx:      context.TODO(),
				c:        &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				command:  "ls",
				execArgs: []string{"-la", "/var/log"},
				dryrun:   false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, command string, execArgs []string, dryrun bool) {
				cmdWithArgs := append([]string{command}, execArgs...)
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{
					User:         "root",
					AttachStdout: true,
					AttachStderr: true,
					Cmd:          cmdWithArgs,
				}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				mockReader := strings.NewReader("total 0\n")
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(func() types.HijackedResponse {
					conn := &mockConn{}
					conn.On("Close").Return(nil)
					return types.HijackedResponse{
						Conn:   conn,
						Reader: bufio.NewReader(mockReader),
					}
				}(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{ExitCode: 0}, nil)
			},
			wantErr: false,
		},
		{
			name: "execute command with no arguments",
			args: args{
				ctx:      context.TODO(),
				c:        &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				command:  "pwd",
				execArgs: []string{},
				dryrun:   false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, command string, execArgs []string, dryrun bool) {
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{
					User:         "root",
					AttachStdout: true,
					AttachStderr: true,
					Cmd:          []string{command},
				}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				mockReader := strings.NewReader("/\n")
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(func() types.HijackedResponse {
					conn := &mockConn{}
					conn.On("Close").Return(nil)
					return types.HijackedResponse{
						Conn:   conn,
						Reader: bufio.NewReader(mockReader),
					}
				}(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{ExitCode: 0}, nil)
			},
			wantErr: false,
		},
		{
			name: "execute command with non-zero exit code",
			args: args{
				ctx:      context.TODO(),
				c:        &ctr.Container{ContainerID: "abc123", ContainerName: "test-container"},
				command:  "ls",
				execArgs: []string{"/nonexistent"},
				dryrun:   false,
			},
			mockSet: func(api *mocks.APIClient, ctx context.Context, c *ctr.Container, command string, execArgs []string, dryrun bool) {
				cmdWithArgs := append([]string{command}, execArgs...)
				api.EXPECT().ContainerExecCreate(ctx, c.ID(), ctypes.ExecOptions{
					User:         "root",
					AttachStdout: true,
					AttachStderr: true,
					Cmd:          cmdWithArgs,
				}).Return(ctypes.ExecCreateResponse{ID: "execID"}, nil)
				mockReader := strings.NewReader("ls: /nonexistent: No such file or directory\n")
				api.EXPECT().ContainerExecAttach(ctx, "execID", ctypes.ExecAttachOptions{}).Return(func() types.HijackedResponse {
					conn := &mockConn{}
					conn.On("Close").Return(nil)
					return types.HijackedResponse{
						Conn:   conn,
						Reader: bufio.NewReader(mockReader),
					}
				}(), nil)
				api.EXPECT().ContainerExecInspect(ctx, "execID").Return(ctypes.ExecInspect{ExitCode: 1}, nil)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine()
			tt.mockSet(api, tt.args.ctx, tt.args.c, tt.args.command, tt.args.execArgs, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			err := client.ExecContainer(tt.args.ctx, tt.args.c, tt.args.command, tt.args.execArgs, tt.args.dryrun)

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.ExecContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}
