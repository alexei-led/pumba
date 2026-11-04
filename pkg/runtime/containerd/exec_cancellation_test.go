package containerd

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRunTaskExecCancellationRetainsTerminalEvidence(t *testing.T) {
	const raceSamples = 64
	for _, tc := range []struct {
		name       string
		code       uint32
		closedWait bool
		heldOutput bool
		waitFails  bool
	}{
		{name: "completed"},
		{name: "failed", code: 7},
		{name: "missing_terminal_status", closedWait: true},
		{name: "interrupted_output", heldOutput: true},
		{name: "wait_failed", waitFails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Either ready select route must retain the same terminal evidence.
			// Fixed sampling exercises both routes without timing assertions.
			for range raceSamples {
				ctx, cancel := context.WithCancel(context.Background())
				t.Cleanup(cancel)
				proc := NewMockProcess(t)
				exitCh := make(chan containerd.ExitStatus, 1)
				waitErr := errors.New("exec wait failed")
				var firstWaitErr error
				if tc.waitFails {
					firstWaitErr = waitErr
				}
				proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), firstWaitErr).Once()
				if !tc.waitFails {
					proc.EXPECT().Start(mock.Anything).Run(func(context.Context) {
						if tc.closedWait {
							close(exitCh)
						} else {
							exitCh <- *containerd.NewExitStatus(tc.code, time.Time{}, nil)
						}
						cancel()
					}).Return(nil).Once()
				}
				if tc.closedWait || tc.waitFails {
					cleanupExit := make(chan containerd.ExitStatus, 1)
					cleanupExit <- *containerd.NewExitStatus(killedExitCode, time.Time{}, nil)
					proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(cleanupExit), nil).Once()
					proc.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Return(nil).Once()
				} else if tc.heldOutput {
					proc.EXPECT().IO().Return(newHeldIO()).Once()
				} else {
					proc.EXPECT().IO().Return(nil).Once()
				}
				proc.EXPECT().Delete(mock.Anything).Return(nil, nil).Once()
				task := NewMockTask(t)
				setupExec(task, proc)

				out, errOut, err := runTaskExec(ctx, task, &specs.Process{}, "exec-id", "owned exec", terminateOwnedExec)

				assert.Empty(t, out)
				assert.Empty(t, errOut)
				switch {
				case tc.waitFails:
					require.ErrorIs(t, err, waitErr)
				case tc.closedWait || tc.heldOutput:
					require.ErrorIs(t, err, context.Canceled)
				case tc.code != 0:
					require.ErrorContains(t, err, "exited with code 7")
				default:
					require.NoError(t, err)
				}
			}
		})
	}
}
