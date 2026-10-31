package docker

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
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
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(mock.Anything, "whichID").Return(ctypes.ExecInspect{}, nil)
				execConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{cmd}, args...), Privileged: false}
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, execConfig).Return(ctypes.ExecCreateResponse{ID: "cmdID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "cmdID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(mock.Anything, "cmdID").Return(ctypes.ExecInspect{}, nil)
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
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(mock.Anything, "whichID").Return(ctypes.ExecInspect{}, nil)
				execConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{cmd}, args...), Privileged: true}
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, execConfig).Return(ctypes.ExecCreateResponse{ID: "cmdID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "cmdID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(mock.Anything, "cmdID").Return(ctypes.ExecInspect{}, nil)
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
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(mock.Anything, "whichID").Return(ctypes.ExecInspect{ExitCode: 1}, nil)
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
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, checkConfig).Return(ctypes.ExecCreateResponse{}, errors.New("which error"))
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
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "whichID", ctypes.ExecAttachOptions{}).Return(types.HijackedResponse{}, errors.New("which error"))
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
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(mock.Anything, "whichID").Return(ctypes.ExecInspect{}, errors.New("which error"))
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
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(mock.Anything, "whichID").Return(ctypes.ExecInspect{}, nil)
				execConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{cmd}, args...), Privileged: false}
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, execConfig).Return(ctypes.ExecCreateResponse{}, errors.New("cmd error"))
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
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, checkConfig).Return(ctypes.ExecCreateResponse{ID: "whichID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "whichID", ctypes.ExecAttachOptions{}).Return(fakeExecAttach(), nil)
				engine.EXPECT().ContainerExecInspect(mock.Anything, "whichID").Return(ctypes.ExecInspect{}, nil)
				execConfig := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Cmd: append([]string{cmd}, args...), Privileged: false}
				engine.EXPECT().ContainerExecCreate(mock.Anything, cID, execConfig).Return(ctypes.ExecCreateResponse{ID: "cmdID"}, nil)
				engine.EXPECT().ContainerExecAttach(mock.Anything, "cmdID", ctypes.ExecAttachOptions{}).Return(types.HijackedResponse{}, errors.New("cmd error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockEngine(t)
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
			api := NewMockEngine(t)
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

func Test_readAttached_interruptedByContext(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	resp := types.HijackedResponse{Conn: client, Reader: bufio.NewReader(client)}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	errc := make(chan error, 1)
	go func() {
		_, err := readAttached(ctx, resp)
		errc <- err
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want deadline exceeded, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("readAttached did not return after ctx deadline")
	}
}

func Test_readAttached_readsToEOF(t *testing.T) {
	resp := types.HijackedResponse{Conn: &net.TCPConn{}, Reader: bufio.NewReader(strings.NewReader("hello"))}
	out, err := readAttached(context.Background(), resp)
	if err != nil || string(out) != "hello" {
		t.Fatalf("got %q, %v", out, err)
	}
}

// blockedAttach returns an attach stream that yields nothing until the test
// writes to or closes the returned server end.
func blockedAttach(t *testing.T) (types.HijackedResponse, net.Conn) {
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close() })
	return types.HijackedResponse{Conn: client, Reader: bufio.NewReader(client)}, server
}

func Test_execOnContainer_probeIgnoresCancellation(t *testing.T) {
	resp, server := blockedAttach(t)
	api := NewMockEngine(t)
	c := &ctr.Container{ContainerID: "abc"}
	notCancelled := func(ctx context.Context) {
		if ctx.Err() != nil {
			t.Errorf("probe got a cancelled context: %v", ctx.Err())
		}
	}
	api.EXPECT().ContainerExecCreate(mock.Anything, "abc", mock.AnythingOfType("container.ExecOptions")).
		RunAndReturn(func(ctx context.Context, _ string, _ ctypes.ExecOptions) (ctypes.ExecCreateResponse, error) {
			notCancelled(ctx)
			return ctypes.ExecCreateResponse{ID: "whichID"}, nil
		}).Once()
	api.EXPECT().ContainerExecAttach(mock.Anything, "whichID", ctypes.ExecAttachOptions{}).
		RunAndReturn(func(ctx context.Context, _ string, _ ctypes.ExecAttachOptions) (types.HijackedResponse, error) {
			notCancelled(ctx)
			return resp, nil
		}).Once()
	api.EXPECT().ContainerExecInspect(mock.Anything, "whichID").Return(ctypes.ExecInspect{ExitCode: 1}, nil).Once()
	client := dockerClient{containerAPI: api, imageAPI: api}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errc := make(chan error, 1)
	go func() { errc <- client.execOnContainer(ctx, c, "tool", nil, false) }()
	time.Sleep(50 * time.Millisecond)
	server.Close()

	select {
	case err := <-errc:
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("want a not-found error from the probe, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("execOnContainer did not return after EOF")
	}
}

func Test_ExecContainer_interruptedByCancellation(t *testing.T) {
	resp, _ := blockedAttach(t)
	api := NewMockEngine(t)
	c := &ctr.Container{ContainerID: "abc"}
	api.EXPECT().ContainerExecCreate(mock.Anything, "abc", mock.AnythingOfType("container.ExecOptions")).
		Return(ctypes.ExecCreateResponse{ID: "execID"}, nil).Once()
	api.EXPECT().ContainerExecAttach(mock.Anything, "execID", ctypes.ExecAttachOptions{}).Return(resp, nil).Once()
	client := dockerClient{containerAPI: api, imageAPI: api}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	errc := make(chan error, 1)
	go func() { errc <- client.ExecContainer(ctx, c, "sleep", []string{"infinity"}, false) }()

	select {
	case err := <-errc:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want deadline exceeded, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ExecContainer did not return after ctx deadline")
	}
}

func Test_runSidecarExec_ignoresCancellation(t *testing.T) {
	resp, server := blockedAttach(t)
	api := NewMockEngine(t)
	attached := make(chan struct{})
	inspected := make(chan struct{}, 1)
	eofReleased := make(chan struct{})
	notCancelled := func(ctx context.Context) {
		if ctx.Err() != nil {
			t.Errorf("fault exec got a cancelled context: %v", ctx.Err())
		}
	}
	api.EXPECT().ContainerExecCreate(mock.Anything, "sidecar", mock.AnythingOfType("container.ExecOptions")).
		RunAndReturn(func(ctx context.Context, _ string, _ ctypes.ExecOptions) (ctypes.ExecCreateResponse, error) {
			notCancelled(ctx)
			return ctypes.ExecCreateResponse{ID: "execID"}, nil
		}).Once()
	api.EXPECT().ContainerExecAttach(mock.Anything, "execID", ctypes.ExecAttachOptions{}).
		RunAndReturn(func(ctx context.Context, _ string, _ ctypes.ExecAttachOptions) (types.HijackedResponse, error) {
			notCancelled(ctx)
			close(attached)
			return resp, nil
		}).Once()
	api.EXPECT().ContainerExecInspect(mock.Anything, "execID").
		Run(func(context.Context, string) {
			select {
			case <-eofReleased:
			default:
				t.Error("fault exec inspected before EOF was released")
			}
			inspected <- struct{}{}
		}).Return(ctypes.ExecInspect{ExitCode: 0}, nil).Once()
	client := dockerClient{containerAPI: api, imageAPI: api}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errc := make(chan error, 1)
	go func() {
		err := client.runSidecarExec(ctx, "sidecar", "sh", []string{"-ec", "true"})
		errc <- err
	}()
	select {
	case <-attached:
	case <-time.After(5 * time.Second):
		t.Fatal("fault exec did not attach")
	}
	select {
	case <-inspected:
		t.Error("fault exec inspected while its stream was still open")
	case err := <-errc:
		t.Fatalf("fault exec returned before EOF: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(eofReleased)
	server.Close() // the script finishes after the cancellation

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("want the fault exec to run to completion, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runSidecarExec did not return after EOF")
	}
}
