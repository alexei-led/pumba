package docker

import (
	"context"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRunExecRejectsCanceledContext(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelDeadline := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelDeadline()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		err  error
	}{
		{name: "canceled", ctx: canceled, err: context.Canceled},
		{name: "deadline", ctx: expired, err: context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine(t)
			opts := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Privileged: true, Cmd: []string{"sh", "-ec", "true"}}
			api.EXPECT().ContainerExecCreate(mock.Anything, "target", opts).
				Return(ctypes.ExecCreateResponse{}, assert.AnError).Maybe()
			client := dockerClient{containerAPI: api}

			out, errOut, err := client.runExec(tt.ctx, "target", opts.Cmd, true)

			require.ErrorIs(t, err, tt.err)
			assert.Empty(t, out)
			assert.Empty(t, errOut)
			api.AssertNumberOfCalls(t, "ContainerExecCreate", 0)
		})
	}
}

func TestExecOnContainerCancellationAfterProbe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api := NewMockEngine(t)
	expectRawExec(api, "target", []string{"which", "sh"}, false, nil, ctypes.ExecInspect{})
	api.EXPECT().ContainerExecInspect(mock.Anything, "raw-exec").Unset()
	api.EXPECT().ContainerExecInspect(mock.Anything, "raw-exec").
		Run(func(context.Context, string) { cancel() }).Return(ctypes.ExecInspect{}, nil).Once()
	install := ctypes.ExecOptions{AttachStdout: true, AttachStderr: true, Privileged: true, Cmd: []string{"sh", "-ec", "true"}}
	api.EXPECT().ContainerExecCreate(mock.Anything, "target", install).
		Return(ctypes.ExecCreateResponse{}, assert.AnError).Maybe()
	client := dockerClient{containerAPI: api}

	err := client.execOnContainer(ctx, &ctr.Container{ContainerID: "target"}, "sh", []string{"-ec", "true"}, true)

	require.ErrorIs(t, err, context.Canceled)
	api.AssertNotCalled(t, "ContainerExecCreate", mock.Anything, "target", install)
}

func TestRunExecCleanupWithCanceledParent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	api := NewMockEngine(t)
	expectRawExec(api, "target", []string{"iptables", "-D", "INPUT"}, true, nil, ctypes.ExecInspect{})
	client := dockerClient{containerAPI: api}

	_, _, err := client.runExec(context.WithoutCancel(ctx), "target", []string{"iptables", "-D", "INPUT"}, true)

	require.NoError(t, err)
}
