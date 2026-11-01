package containerd

import (
	"context"
	"syscall"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestStressDirectExecWaitFailureReapsOwnedProcess(t *testing.T) {
	waitErr := grpcstatus.Error(codes.Unavailable, "wait unavailable")
	proc := NewMockProcess(t)
	running, reaped := make(chan containerd.ExitStatus, 1), make(chan containerd.ExitStatus, 1)
	running <- *containerd.NewExitStatus(0, time.Now(), waitErr)
	proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(running), nil).Once()
	proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(reaped), nil).Once()
	proc.EXPECT().Start(mock.Anything).Return(nil).Once()
	proc.EXPECT().Kill(mock.Anything, syscall.SIGTERM).
		Run(func(context.Context, syscall.Signal, ...containerd.KillOpts) {
			reaped <- *containerd.NewExitStatus(143, time.Now(), nil)
		}).Return(nil).Once()
	proc.EXPECT().Delete(mock.Anything).Return(containerd.NewExitStatus(143, time.Now(), nil), nil).Once()
	task := newRunningTask(t)
	setupExec(task, proc)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))

	result, err := newTestClient(api).StressContainer(context.Background(),
		stressReq(testContainer("c1"), []string{"--cpu", "1"}, "", false, time.Hour, false, false))

	require.NoError(t, err)
	require.Empty(t, result.SidecarID)
	require.ErrorIs(t, requireStressError(t, result.Output, result.Errors, time.Second), waitErr)
}
