package containerd

import (
	"context"
	"sync"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// heldIO models output held open by a child after exec exits. Tests may
// complete the copy by closing done; otherwise only Cancel ends the copy.
type heldIO struct {
	done     chan struct{}
	canceled chan struct{}
	once     sync.Once
}

func newHeldIO() *heldIO {
	return &heldIO{done: make(chan struct{}), canceled: make(chan struct{})}
}

func (h *heldIO) Config() cio.Config { return cio.Config{} }
func (h *heldIO) Cancel()            { h.once.Do(func() { close(h.canceled) }) }
func (h *heldIO) Close() error       { return nil }
func (h *heldIO) Wait() {
	select {
	case <-h.done:
	case <-h.canceled:
	}
}

func TestWaitIO(t *testing.T) {
	t.Parallel()

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name     string
		ctx      context.Context
		complete bool
		wantErr  error
	}{
		{name: "complete_output", ctx: context.Background(), complete: true},
		{name: "output_held_open_past_exit_is_incomplete", ctx: context.Background(), wantErr: errIncompleteOutput},
		{name: "ctx_canceled_returns_ctx_error", ctx: canceledCtx, wantErr: context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pio := newHeldIO()
			if tt.complete {
				close(pio.done)
			}
			p := NewMockProcess(t)
			p.EXPECT().IO().Return(pio)

			errCh := make(chan error, 1)
			go func() { errCh <- waitIO(tt.ctx, p, 10*time.Millisecond) }()

			select {
			case err := <-errCh:
				if tt.wantErr != nil {
					require.ErrorIs(t, err, tt.wantErr)
				} else {
					require.NoError(t, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("waitIO blocked on IO that never reaches EOF")
			}
			assert.Equal(t, !tt.complete, isClosed(pio.canceled), "only incomplete IO copies must be canceled")
		})
	}
}

func TestExecContainerIncompleteOutput(t *testing.T) {
	t.Parallel()
	for _, code := range []uint32{0, 1} {
		t.Run(map[uint32]string{0: "successful_exit", 1: "nonzero_exit"}[code], func(t *testing.T) {
			t.Parallel()
			proc := newFailProcess(t, code)
			proc.EXPECT().IO().Unset()
			proc.EXPECT().IO().Return(newHeldIO()).Once()
			task := newRunningTask(t)
			setupExec(task, proc)
			api := NewMockapiClient(t)
			setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))

			err := newTestClient(api).ExecContainer(context.Background(), testContainer("c1"), "sh", []string{"-c", "sleep 60 & exit"}, false)

			if code == 0 {
				require.NoError(t, err, "a successful user command may leave a child holding stdout open")
			} else {
				require.ErrorContains(t, err, "exited with code 1")
			}
		})
	}
}

func TestExecContainerCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proc := NewMockProcess(t)
	proc.EXPECT().Wait(mock.Anything).Return(make(<-chan containerd.ExitStatus), nil).Once()
	proc.EXPECT().Start(mock.Anything).Run(func(context.Context) { cancel() }).Return(nil).Once()
	proc.EXPECT().Delete(mock.Anything).Return(nil, nil).Once()
	task := newRunningTask(t)
	setupExec(task, proc)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))

	err := newTestClient(api).ExecContainer(ctx, testContainer("c1"), "sleep", []string{"60"}, false)

	require.ErrorIs(t, err, context.Canceled)
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestExecContainerIncompleteOutputDoesNotHideCleanupFailure(t *testing.T) {
	t.Parallel()
	proc := newSuccessProcess(t)
	proc.EXPECT().IO().Unset()
	proc.EXPECT().IO().Return(newHeldIO()).Once()
	proc.EXPECT().Delete(mock.Anything).Unset()
	proc.EXPECT().Delete(mock.Anything).Return(nil, assert.AnError).Once()
	task := newRunningTask(t)
	setupExec(task, proc)
	api := NewMockapiClient(t)
	setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))

	err := newTestClient(api).ExecContainer(context.Background(), testContainer("c1"), "sh", []string{"-c", "sleep 60 & exit"}, false)

	require.ErrorIs(t, err, assert.AnError)
	assert.ErrorIs(t, err, errExecCleanup)
}

func TestExecTaskOutputClosedWaitChannel(t *testing.T) {
	t.Parallel()
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing_exit_status", true: "cancellation_closed_wait"}[canceled], func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			exitCh := make(chan containerd.ExitStatus)
			proc := NewMockProcess(t)
			proc.EXPECT().Wait(mock.Anything).Return((<-chan containerd.ExitStatus)(exitCh), nil).Once()
			proc.EXPECT().Start(mock.Anything).Run(func(context.Context) {
				if canceled {
					cancel()
				}
				close(exitCh)
			}).Return(nil).Once()
			proc.EXPECT().IO().Return(nil).Maybe()
			proc.EXPECT().Delete(mock.Anything).Return(nil, nil).Once()
			task := NewMockTask(t)
			setupExec(task, proc)

			out, errOut, err := execTaskOutput(ctx, task, &specs.Process{}, "exec-id", "inspection")

			require.Error(t, err, "closed wait channel is not evidence of successful exit")
			assert.Empty(t, out)
			assert.Empty(t, errOut)
			if canceled {
				assert.ErrorIs(t, err, context.Canceled)
			} else {
				assert.ErrorContains(t, err, "wait channel closed without exit status")
			}
		})
	}
}
