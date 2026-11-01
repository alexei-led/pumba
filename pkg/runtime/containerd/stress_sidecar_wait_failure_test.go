package containerd

import (
	"context"
	"syscall"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestStressSidecarWaitFailureStopsOwnedTask(t *testing.T) {
	waitErr := grpcstatus.Error(codes.Unavailable, "sidecar wait unavailable")
	task, sidecar := NewMockTask(t), NewMockContainer(t)
	initial, reaped := make(chan containerd.ExitStatus, 1), make(chan containerd.ExitStatus, 1)
	initial <- *containerd.NewExitStatus(0, time.Now(), waitErr)
	task.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(reaped), nil).Once()
	task.EXPECT().Kill(mock.Anything, syscall.SIGKILL).
		Run(func(context.Context, syscall.Signal, ...containerd.KillOpts) {
			reaped <- *containerd.NewExitStatus(137, time.Now(), nil)
		}).Return(nil).Once()
	task.EXPECT().Delete(mock.Anything).Return(containerd.NewExitStatus(137, time.Now(), nil), nil).Once()
	sidecar.EXPECT().Delete(mock.Anything, mock.MatchedBy(func(option containerd.DeleteOpts) bool { return option != nil })).Return(nil).Once()
	output, failures := make(chan string, 1), make(chan error, 1)

	newTestClient(nil).waitStressSidecar(context.Background(), "owned-sidecar", sidecar, task, initial, output, failures)

	require.ErrorIs(t, requireStressError(t, output, failures, time.Second), waitErr)
}

func TestStressSidecarAmbiguousStartFailureStopsOwnedTask(t *testing.T) { //nolint:paralleltest // mutates package-level cgroupReader
	startErr := grpcstatus.Error(codes.Unavailable, "start response unavailable")
	task, sidecar := NewMockTask(t), NewMockContainer(t)
	initial, reaped := make(chan containerd.ExitStatus), make(chan containerd.ExitStatus, 1)
	task.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(initial), nil).Once()
	task.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(reaped), nil).Once()
	task.EXPECT().Start(mock.Anything).Return(startErr).Once()
	task.EXPECT().Kill(mock.Anything, syscall.SIGKILL).
		Run(func(context.Context, syscall.Signal, ...containerd.KillOpts) {
			reaped <- *containerd.NewExitStatus(137, time.Now(), nil)
		}).Return(nil).Once()
	task.EXPECT().Delete(mock.Anything).Return(containerd.NewExitStatus(137, time.Now(), nil), nil).Once()
	sidecar.EXPECT().NewTask(mock.Anything, mock.MatchedBy(func(creator cio.Creator) bool { return creator != nil })).Return(task, nil).Once()
	sidecar.EXPECT().Delete(mock.Anything, mock.MatchedBy(func(option containerd.DeleteOpts) bool { return option != nil })).Return(nil).Once()
	api := setupStressTarget(t, 42, "0::/system.slice/docker-c1.scope\n")
	setupStressSidecar(t, api, "stress-ng", sidecar)

	result, err := newTestClient(api).StressContainer(context.Background(),
		stressReq(testContainer("c1"), []string{"--cpu", "1"}, "stress-ng", false, time.Hour, false, false))

	require.Nil(t, result)
	require.ErrorIs(t, err, startErr)
}
