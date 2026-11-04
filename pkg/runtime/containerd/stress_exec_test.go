package containerd

import (
	"context"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTerminateStressExecGracefulAndEscalation(t *testing.T) {
	for _, tt := range []struct {
		name     string
		code     uint32
		escalate bool
		wantErr  bool
	}{
		{name: "graceful", code: 0},
		{name: "signal exit", code: 143},
		{name: "workload failure", code: 7, wantErr: true},
		{name: "term ignored", code: 137, escalate: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				proc := NewMockProcess(t)
				graceful, forced := make(chan containerd.ExitStatus, 1), make(chan containerd.ExitStatus, 1)
				proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(graceful), nil).Once()
				proc.EXPECT().Kill(mock.Anything, syscall.SIGTERM).
					Run(func(context.Context, syscall.Signal, ...containerd.KillOpts) {
						if !tt.escalate {
							graceful <- *containerd.NewExitStatus(tt.code, time.Now(), nil)
						}
					}).Return(nil).Once()
				if tt.escalate {
					proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(forced), nil).Once()
					proc.EXPECT().Kill(mock.Anything, syscall.SIGKILL).
						Run(func(context.Context, syscall.Signal, ...containerd.KillOpts) {
							forced <- *containerd.NewExitStatus(tt.code, time.Now(), nil)
						}).Return(nil).Once()
				}
				start := time.Now()
				result := terminateStressExec(context.Background(), proc)
				require.NoError(t, result.err)
				require.NotNil(t, result.exit)
				if tt.escalate {
					require.Equal(t, killedExitCode, result.expectedExitCode)
					require.Equal(t, sidecarKillTimeout, time.Since(start))
				} else {
					wantExitCode := uint32(signalExitBase + syscall.SIGTERM)
					require.Equal(t, wantExitCode, result.expectedExitCode)
				}
				err := ownedExecResultError("stress", result)
				if tt.wantErr {
					require.ErrorContains(t, err, "exited with code 7")
				} else {
					require.NoError(t, err)
				}
			})
		})
	}
}
