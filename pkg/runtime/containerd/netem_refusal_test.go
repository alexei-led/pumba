package containerd

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	"github.com/alexei-led/pumba/pkg/tc"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNetemStartupRefusalStatus(t *testing.T) {
	t.Parallel()
	for _, image := range []string{"", "nettools"} {
		for _, failure := range []string{"refused", "partial", "read", "status", "exec cleanup", "partial exec cleanup", "read exec cleanup", "contradictory exec cleanup", "contradictory delete status", "matching delete status", "sidecar cleanup", "partial sidecar cleanup", "read sidecar cleanup", "stop"} {
			cleanupFailure := failure == "sidecar cleanup" || failure == "partial sidecar cleanup" || failure == "read sidecar cleanup"
			if image == "" && cleanupFailure {
				continue
			}
			t.Run(image+"/"+failure, func(t *testing.T) {
				t.Parallel()
				req := &ctr.NetemRequest{Container: testContainer("c1"), Interface: "eth0", Command: []string{"delay", "100ms"}, Sidecar: ctr.SidecarSpec{Image: image}}
				argv, err := netns.NetemStart(req)
				require.NoError(t, err)
				if failure == "stop" {
					argv, err = netns.NetemStop(req)
					require.NoError(t, err)
				}
				cleanupErr := errors.New("cleanup failed")
				var api *MockapiClient
				var task *MockTask
				if image != "" {
					var sidecar *MockContainer
					api, task, sidecar = setupNetSidecarContainer(t, ctr.SidecarLabels("", "c1", ctr.SidecarKindNetem))
					if cleanupFailure {
						sidecar.EXPECT().Delete(mock.Anything, mock.Anything).Unset()
						sidecar.EXPECT().Delete(mock.Anything, mock.Anything).Return(cleanupErr).Once()
					}
				} else {
					api, task = NewMockapiClient(t), newRunningTask(t)
					setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))
				}
				code := uint32(tc.NetemRefusedExitCode)
				if failure == "partial" || failure == "partial sidecar cleanup" || failure == "partial exec cleanup" {
					code = 1
				}
				proc := newFailProcess(t, code)
				switch failure {
				case "read", "read sidecar cleanup", "read exec cleanup":
					proc.EXPECT().IO().Unset()
					proc.EXPECT().IO().Return(newHeldIO()).Once()
				case "status":
					statuses := make(chan containerd.ExitStatus, 2)
					statuses <- *containerd.NewExitStatus(code, time.Now(), errors.New("unknown status"))
					statuses <- *containerd.NewExitStatus(137, time.Now(), nil)
					proc.EXPECT().Wait(mock.Anything).Unset()
					proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(statuses), nil).Twice()
					proc.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Return(nil).Once()
				}
				execCleanupFailure := failure == "exec cleanup" || failure == "partial exec cleanup" || failure == "read exec cleanup" || failure == "contradictory exec cleanup"
				if execCleanupFailure {
					var terminal *containerd.ExitStatus
					if failure == "contradictory exec cleanup" {
						terminal = containerd.NewExitStatus(1, time.Now(), nil)
					}
					proc.EXPECT().Delete(mock.Anything).Unset()
					proc.EXPECT().Delete(mock.Anything).Return(terminal, cleanupErr).Once()
				}
				if failure == "contradictory delete status" || failure == "matching delete status" {
					deleteCode := code
					if failure == "contradictory delete status" {
						deleteCode = 1
					}
					proc.EXPECT().Delete(mock.Anything).Unset()
					proc.EXPECT().Delete(mock.Anything).Return(containerd.NewExitStatus(deleteCode, time.Now(), nil), nil).Once()
				}
				task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs(argv...), mock.Anything).Return(proc, nil).Once()
				client := newTestClient(api)
				if failure == "stop" {
					err = client.StopNetemContainer(context.Background(), req)
				} else {
					err = client.NetemContainer(context.Background(), req)
				}
				require.Error(t, err)
				require.Equal(t, failure == "refused" || failure == "sidecar cleanup" || failure == "exec cleanup" || failure == "matching delete status", errors.Is(err, ctr.ErrNetemNotApplied))
				if execCleanupFailure {
					require.ErrorIs(t, err, errExecCleanup)
					require.ErrorIs(t, err, cleanupErr)
					require.Contains(t, err.Error(), "failed to delete exec process")
				}
				if cleanupFailure {
					require.ErrorIs(t, err, cleanupErr, "helper removal failure must remain visible")
				}
				if failure == "refused" || failure == "partial" || failure == "exec cleanup" || failure == "sidecar cleanup" {
					var exited *netns.ExecExitError
					require.ErrorAs(t, err, &exited)
					require.Equal(t, int(code), exited.Code)
					require.Equal(t, argv, exited.Argv)
					require.Contains(t, err.Error(), argv[2])
				}
			})
		}
	}
}
