package containerd

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFaultExecFinishesBeforeCancellationCleanup(t *testing.T) {
	t.Parallel()
	for _, sidecar := range []bool{false, true} {
		for _, kind := range []string{ctr.SidecarKindNetem, ctr.SidecarKindIPTables} {
			t.Run(map[bool]string{false: "target/", true: "sidecar/"}[sidecar]+kind, func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				started, exited := make(chan struct{}), make(chan struct{})
				exitCh := make(chan containerd.ExitStatus, 1)
				proc := NewMockProcess(t)
				proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil).Once()
				proc.EXPECT().Start(mock.Anything).Run(func(execCtx context.Context) {
					cancel()
					assert.NoError(t, execCtx.Err(), "caller cancellation must not interrupt a mutating exec")
					deadline, ok := execCtx.Deadline()
					assert.True(t, ok, "the detached exec must have its own deadline")
					assert.WithinDuration(t, time.Now().Add(2*time.Minute), deadline, time.Second)
					ns, ok := namespaces.Namespace(execCtx)
					assert.True(t, ok)
					assert.Equal(t, "test-ns", ns)
					close(started)
				}).Return(nil).Once()
				proc.EXPECT().IO().Return(nil).Maybe()
				proc.EXPECT().Delete(mock.Anything).Run(func(deleteCtx context.Context, _ ...containerd.ProcessDeleteOpts) {
					assert.True(t, isClosed(exited), "exec deletion must follow completion, not caller cancellation")
					assert.NoError(t, deleteCtx.Err())
				}).Return(nil, nil).Once()

				var api *MockapiClient
				var task *MockTask
				spec := ctr.SidecarSpec{}
				if sidecar {
					api, task = setupNetSidecar(t, ctr.SidecarLabels("", "c1", kind))
					spec.Image = "nettools"
					task.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Unset()
					task.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Run(func(cleanupCtx context.Context, _ syscall.Signal, _ ...containerd.KillOpts) {
						assert.True(t, isClosed(exited), "sidecar removal cannot race an installing exec")
						assert.NoError(t, cleanupCtx.Err())
					}).Return(nil).Once()
				} else {
					api, task = NewMockapiClient(t), newRunningTask(t)
					setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))
				}
				setupExec(task, proc)
				client := newTestClient(api)
				result := make(chan error, 1)
				go func() {
					if kind == ctr.SidecarKindNetem {
						result <- client.NetemContainer(ctx, &ctr.NetemRequest{Container: testContainer("c1"), Interface: "eth0", Command: []string{"delay", "100ms"}, Sidecar: spec})
					} else {
						result <- client.IPTablesContainer(ctx, &ctr.IPTablesRequest{Container: testContainer("c1"), CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}, Sidecar: spec})
					}
				}()
				select {
				case <-started:
				case <-time.After(5 * time.Second):
					t.Fatal("fault command did not start")
				}
				var returned bool
				select {
				case err := <-result:
					returned = true
					t.Errorf("fault returned before exec completed: %v", err)
				case <-time.After(20 * time.Millisecond):
				}
				close(exited)
				exitCh <- *containerd.NewExitStatus(0, time.Now(), nil)
				if !returned {
					select {
					case err := <-result:
						require.NoError(t, err)
					case <-time.After(5 * time.Second):
						t.Fatal("completed fault did not return")
					}
				}
			})
		}
	}
}

func TestInspectNetnsSidecarCleanupFailure(t *testing.T) {
	t.Parallel()
	for _, operationFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "successful_inspection", true: "failed_inspection"}[operationFails], func(t *testing.T) {
			t.Parallel()
			cleanupErr, operationErr := errors.New("snapshot deletion failed"), errors.New("exec failed")
			api, task, sidecar := setupNetSidecarContainer(t, ctr.SidecarLabels("", "c1", ctr.SidecarKindInspect))
			sidecar.EXPECT().Delete(mock.Anything, mock.Anything).Unset()
			sidecar.EXPECT().Delete(mock.Anything, mock.Anything).Return(cleanupErr).Once()
			if operationFails {
				task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).Return(nil, operationErr).Once()
			} else {
				task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("tc", "qdisc", "show"), mock.Anything).Return(newSuccessProcess(t), nil).Once()
				task.EXPECT().Exec(mock.Anything, mock.Anything, execArgs("iptables", "-S"), mock.Anything).Return(newSuccessProcess(t), nil).Once()
			}

			state, err := newTestClient(api).InspectNetns(context.Background(), &ctr.InspectRequest{Container: testContainer("c1"), Sidecar: ctr.SidecarSpec{Image: "nettools"}})

			require.ErrorIs(t, err, cleanupErr)
			if operationFails {
				assert.ErrorIs(t, err, operationErr)
			}
			assert.Nil(t, state, "inspection must not report usable state when it leaves a helper behind")
		})
	}
}

func TestCleanupSidecarFailures(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"task_lookup", "wait", "kill", "task_delete", "container_delete", "not_found"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			cntr := NewMockContainer(t)
			if stage == "task_lookup" || stage == "not_found" {
				err := error(assert.AnError)
				if stage == "not_found" {
					err = errdefs.ErrNotFound
				}
				cntr.EXPECT().Task(mock.Anything, mock.Anything).Return(nil, err).Once()
			} else {
				task := NewMockTask(t)
				exitCh := make(chan containerd.ExitStatus, 1)
				exitCh <- *containerd.NewExitStatus(137, time.Now(), nil)
				cntr.EXPECT().Task(mock.Anything, mock.Anything).Return(task, nil).Once()
				var waitErr, killErr, deleteErr error
				switch stage {
				case "wait":
					waitErr = assert.AnError
				case "kill":
					killErr = assert.AnError
				case "task_delete":
					deleteErr = assert.AnError
				}
				task.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), waitErr).Once()
				task.EXPECT().Kill(mock.Anything, syscall.SIGKILL).Return(killErr).Once()
				task.EXPECT().Delete(mock.Anything).Return(nil, deleteErr).Once()
			}
			var deleteErr error
			if stage == "container_delete" {
				deleteErr = assert.AnError
			} else if stage == "not_found" {
				deleteErr = errdefs.ErrNotFound
			}
			cntr.EXPECT().Delete(mock.Anything, mock.Anything).Return(deleteErr).Once()

			err := newTestClient(nil).cleanupSidecar(context.Background(), cntr)

			if stage == "not_found" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, assert.AnError)
			}
		})
	}
}

func TestIPTablesCancellationDoesNotStartNextCommand(t *testing.T) {
	t.Parallel()
	for _, sidecar := range []bool{false, true} {
		t.Run(map[bool]string{false: "target", true: "sidecar"}[sidecar], func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var api *MockapiClient
			var task *MockTask
			req := &ctr.IPTablesRequest{Container: testContainer("c1"), CmdPrefix: []string{"-I", "INPUT"}, CmdSuffix: []string{"-j", "DROP"}, SrcIPs: []*net.IPNet{
				{IP: net.ParseIP("10.0.0.1").To4(), Mask: net.CIDRMask(32, 32)},
				{IP: net.ParseIP("10.0.0.2").To4(), Mask: net.CIDRMask(32, 32)},
			}}
			if sidecar {
				api, task = setupNetSidecar(t, ctr.SidecarLabels("", "c1", ctr.SidecarKindIPTables))
				req.Sidecar.Image = "nettools"
			} else {
				api, task = NewMockapiClient(t), newRunningTask(t)
				setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))
			}
			proc := newSuccessProcess(t)
			proc.EXPECT().Delete(mock.Anything).Unset()
			proc.EXPECT().Delete(mock.Anything).Run(func(context.Context, ...containerd.ProcessDeleteOpts) { cancel() }).Return(nil, nil).Once()
			task.EXPECT().Exec(mock.Anything, mock.Anything,
				execArgs("iptables", "-I", "INPUT", "-s", "10.0.0.1/32", "-j", "DROP"), mock.Anything).Return(proc, nil).Once()

			err := newTestClient(api).IPTablesContainer(ctx, req)

			require.ErrorIs(t, err, context.Canceled)
		})
	}
}
