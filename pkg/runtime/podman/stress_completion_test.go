package podman

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func completionRuntime(t *testing.T) (*mocks.APIClient, func(context.Context) (*ctr.StressResult, error)) {
	t.Helper()
	api := mocks.NewAPIClient(t)
	api.EXPECT().ContainerInspect(mock.Anything, "target").Return(runningInspect(42), nil).Once()
	stubCgroupReader(t, func(int) ([]byte, error) { return []byte(v2SystemdCgroup), nil })
	api.EXPECT().ContainerCreate(mock.Anything, mock.MatchedBy(func(cfg *ctypes.Config) bool { return cfg.Image == "stress-ng" }), mock.MatchedBy(func(hc *ctypes.HostConfig) bool { return !hc.AutoRemove }), (*networktypes.NetworkingConfig)(nil), (*v1.Platform)(nil), "").Return(ctypes.CreateResponse{ID: "owned-stress"}, nil).Once()
	return api, func(ctx context.Context) (*ctr.StressResult, error) {
		client := &podmanClient{api: api}
		return client.StressContainer(ctx, &ctr.StressRequest{Container: &ctr.Container{ContainerID: "target"}, Sidecar: ctr.SidecarSpec{Image: "stress-ng"}})
	}
}

// A failed start must not launch any reader, even if the attachment never EOFs.
type unstartedStressReader struct{}

func (unstartedStressReader) Read([]byte) (int, error) {
	panic("drained attachment after failed start")
}

type cancelingStressReader struct {
	cancel context.CancelFunc
	err    error
}

func (r cancelingStressReader) Read([]byte) (int, error) { r.cancel(); return 0, r.err }

type failedStressReader struct{ err error }

func (r failedStressReader) Read([]byte) (int, error) { return 0, r.err }

func collectStressResult(t *testing.T, result *ctr.StressResult) (string, error) {
	t.Helper()
	out, errs := result.Output, result.Errors
	var text string
	var failure error
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for out != nil || errs != nil {
		select {
		case value, ok := <-out:
			if !ok {
				out = nil
			} else {
				text += value
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil
			} else {
				failure = errors.Join(failure, err)
			}
		case <-timer.C:
			t.Fatal("stress result did not finish after cleanup")
		}
	}
	return text, failure
}

func TestStressContainer_OperationAndRemovalFailures(t *testing.T) {
	for _, stage := range []string{"attach", "start", "read", "read_cancel", "wait", "exit"} {
		t.Run(stage, func(t *testing.T) {
			api, start := completionRuntime(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			operationErr, removeErr := errors.New("stress operation failed"), errors.New("stress removal failed")
			conn, peer := net.Pipe()
			defer peer.Close()
			defer conn.Close()
			var reader io.Reader = strings.NewReader("done")
			if stage == "read" {
				reader = failedStressReader{operationErr}
			}
			if stage == "start" {
				reader = unstartedStressReader{}
			}
			if stage == "read_cancel" {
				reader = cancelingStressReader{cancel: cancel, err: errors.Join(operationErr, context.Canceled)}
			}
			attach := types.HijackedResponse{Conn: conn, Reader: bufio.NewReader(reader)}
			if stage == "attach" {
				api.EXPECT().ContainerAttach(mock.Anything, "owned-stress", ctypes.AttachOptions{Stdout: true, Stderr: true, Stream: true}).Return(types.HijackedResponse{}, operationErr).Once()
			} else {
				api.EXPECT().ContainerAttach(mock.Anything, "owned-stress", ctypes.AttachOptions{Stdout: true, Stderr: true, Stream: true}).Return(attach, nil).Once()
				var startErr error
				if stage == "start" {
					startErr = operationErr
				}
				api.EXPECT().ContainerStart(mock.Anything, "owned-stress", ctypes.StartOptions{}).Return(startErr).Once()
				if stage != "start" {
					exited, errs := make(chan ctypes.WaitResponse, 1), make(chan error, 1)
					if stage == "wait" {
						errs <- operationErr
					} else if stage == "exit" {
						exited <- ctypes.WaitResponse{StatusCode: 7}
					} else {
						exited <- ctypes.WaitResponse{}
					}
					api.EXPECT().ContainerWait(mock.Anything, "owned-stress", ctypes.WaitConditionNotRunning).Return((<-chan ctypes.WaitResponse)(exited), (<-chan error)(errs)).Once()
				}
			}
			api.EXPECT().ContainerRemove(mock.Anything, "owned-stress", ctypes.RemoveOptions{Force: true}).Run(func(ctx context.Context, _ string, _ ctypes.RemoveOptions) {
				assert.NoError(t, ctx.Err())
				deadline, ok := ctx.Deadline()
				assert.True(t, ok)
				assert.LessOrEqual(t, time.Until(deadline), sidecarRemoveTimeout)
			}).Return(removeErr).Once()
			result, err := start(ctx)
			if err == nil {
				_, err = collectStressResult(t, result)
			} else {
				require.Nil(t, result)
			}
			require.ErrorIs(t, err, removeErr)
			var cleanup *ctr.StressCleanupError
			require.ErrorAs(t, err, &cleanup)
			if stage == "exit" {
				require.ErrorContains(t, err, "exited with code 7")
			} else {
				require.ErrorIs(t, err, operationErr)
			}
		})
	}
}

func TestStressContainer_ExitAndRemovalPrecedeCompletion(t *testing.T) {
	api, start := completionRuntime(t)
	conn, peer := net.Pipe()
	defer peer.Close()
	defer conn.Close()
	api.EXPECT().ContainerAttach(mock.Anything, "owned-stress", ctypes.AttachOptions{Stdout: true, Stderr: true, Stream: true}).Return(types.HijackedResponse{Conn: conn, Reader: bufio.NewReader(strings.NewReader("done"))}, nil).Once()
	started := api.EXPECT().ContainerStart(mock.Anything, "owned-stress", ctypes.StartOptions{}).Return(nil).Once()
	exited := make(chan ctypes.WaitResponse, 1)
	waiting := make(chan struct{})
	api.EXPECT().ContainerWait(mock.Anything, "owned-stress", ctypes.WaitConditionNotRunning).Run(func(context.Context, string, ctypes.WaitCondition) { close(waiting) }).Return((<-chan ctypes.WaitResponse)(exited), make(<-chan error)).NotBefore(started).Once()
	removing, release := make(chan struct{}), make(chan struct{})
	api.EXPECT().ContainerRemove(mock.Anything, "owned-stress", ctypes.RemoveOptions{Force: true}).Run(func(context.Context, string, ctypes.RemoveOptions) { close(removing); <-release }).Return(nil).Once()
	result, err := start(context.Background())
	require.NoError(t, err)
	<-waiting
	select {
	case <-removing:
		t.Fatal("removed sidecar before actual exit")
	case <-time.After(10 * time.Millisecond):
	}
	exited <- ctypes.WaitResponse{}
	<-removing
	select {
	case <-result.Output:
		t.Fatal("output finished before removal")
	case <-result.Errors:
		t.Fatal("errors finished before removal")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	text, err := collectStressResult(t, result)
	require.NoError(t, err)
	require.Equal(t, "done", text)
}

func TestStressContainer_CancellationClosesAttachAndRemovesOwnedSidecar(t *testing.T) {
	api, start := completionRuntime(t)
	conn, peer := net.Pipe()
	defer peer.Close()
	defer conn.Close()
	api.EXPECT().ContainerAttach(mock.Anything, "owned-stress", ctypes.AttachOptions{Stdout: true, Stderr: true, Stream: true}).Return(types.HijackedResponse{Conn: conn, Reader: bufio.NewReader(conn)}, nil).Once()
	api.EXPECT().ContainerStart(mock.Anything, "owned-stress", ctypes.StartOptions{}).Return(nil).Once()
	waiting := make(chan struct{})
	waitErrors := make(chan error, 1)
	api.EXPECT().ContainerWait(mock.Anything, "owned-stress", ctypes.WaitConditionNotRunning).
		Run(func(waitCtx context.Context, _ string, _ ctypes.WaitCondition) {
			context.AfterFunc(waitCtx, func() { waitErrors <- waitCtx.Err() })
			close(waiting)
		}).Return(make(<-chan ctypes.WaitResponse), (<-chan error)(waitErrors)).Once()
	removeErr := errors.New("cancel cleanup failed")
	api.EXPECT().ContainerRemove(mock.Anything, "owned-stress", ctypes.RemoveOptions{Force: true}).Run(func(ctx context.Context, _ string, _ ctypes.RemoveOptions) {
		assert.NoError(t, ctx.Err())
		_, ok := ctx.Deadline()
		assert.True(t, ok)
	}).Return(removeErr).Once()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := start(ctx)
	require.NoError(t, err)
	<-waiting
	cancel()
	_, err = collectStressResult(t, result)
	require.ErrorIs(t, err, removeErr)
	require.NotErrorIs(t, err, context.Canceled)
}

func TestStressWaitExit_CancellationPreservesObservedFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 100 {
		exited := make(chan ctypes.WaitResponse, 1)
		exited <- ctypes.WaitResponse{StatusCode: 1}
		waitErrors := make(chan error, 1)
		waitErrors <- context.Canceled
		require.ErrorContains(t, waitStressExit(ctx, exited, waitErrors), "exited with code 1")
	}
}

func TestStressWaitExit_CancellationRetainsJoinedFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	genuine := errors.New("genuine wait failure")
	waitErrors := make(chan error, 1)
	waitErrors <- errors.Join(genuine, context.Canceled)
	require.ErrorIs(t, waitStressExit(ctx, make(<-chan ctypes.WaitResponse), waitErrors), genuine)
}

func TestStressContainerDrainsUnbufferedWait(t *testing.T) {
	for _, mode := range []string{"read_error", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			api, start := completionRuntime(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			readErr := errors.New("attachment read failed")
			var reader io.Reader = failedStressReader{readErr}
			if mode == "canceled" {
				reader = cancelingStressReader{cancel: cancel, err: net.ErrClosed}
			}
			conn, peer := net.Pipe()
			defer peer.Close()
			defer conn.Close()
			api.EXPECT().ContainerAttach(mock.Anything, "owned-stress", ctypes.AttachOptions{Stdout: true, Stderr: true, Stream: true}).
				Return(types.HijackedResponse{Conn: conn, Reader: bufio.NewReader(reader)}, nil).Once()
			api.EXPECT().ContainerStart(mock.Anything, "owned-stress", ctypes.StartOptions{}).Return(nil).Once()
			exited, waitErrors := make(chan ctypes.WaitResponse), make(chan error, 1)
			publish, delivered, abort, publisherDone := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
			if mode == "read_error" {
				close(publish)
			} else {
				context.AfterFunc(ctx, func() { close(publish) })
			}
			go func() {
				defer close(publisherDone)
				select {
				case <-publish:
					select {
					case exited <- ctypes.WaitResponse{}:
						close(delivered)
					case <-abort:
					}
				case <-abort:
				}
			}()
			t.Cleanup(func() { close(abort); <-publisherDone })
			api.EXPECT().ContainerWait(mock.Anything, "owned-stress", ctypes.WaitConditionNotRunning).
				Return((<-chan ctypes.WaitResponse)(exited), (<-chan error)(waitErrors)).Once()
			api.EXPECT().ContainerRemove(mock.Anything, "owned-stress", ctypes.RemoveOptions{Force: true}).Return(nil).Once()

			result, err := start(ctx)
			require.NoError(t, err)
			_, err = collectStressResult(t, result)
			if mode == "read_error" {
				require.ErrorIs(t, err, readErr)
			} else {
				require.NoError(t, err)
			}
			select {
			case <-delivered:
			case <-time.After(time.Second):
				t.Fatal("SDK wait sender remained blocked after stress completion")
			}
		})
	}
}

func TestStressContainerCancellationWaitsForRelayedFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api, start := completionRuntime(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		conn, peer := net.Pipe()
		defer peer.Close()
		defer conn.Close()
		api.EXPECT().ContainerAttach(mock.Anything, "owned-stress", ctypes.AttachOptions{Stdout: true, Stderr: true, Stream: true}).
			Return(types.HijackedResponse{Conn: conn, Reader: bufio.NewReader(strings.NewReader("done"))}, nil).Once()
		api.EXPECT().ContainerStart(mock.Anything, "owned-stress", ctypes.StartOptions{}).Return(nil).Once()
		exited, waitErrors := make(chan ctypes.WaitResponse), make(chan error, 1)
		waiting, release := make(chan struct{}), make(chan struct{})
		defer func() {
			select {
			case <-release:
			default:
				close(release)
			}
		}()
		go func() {
			<-release
			exited <- ctypes.WaitResponse{StatusCode: 7}
		}()
		api.EXPECT().ContainerWait(mock.Anything, "owned-stress", ctypes.WaitConditionNotRunning).
			Run(func(context.Context, string, ctypes.WaitCondition) { close(waiting) }).
			Return((<-chan ctypes.WaitResponse)(exited), (<-chan error)(waitErrors)).Once()
		api.EXPECT().ContainerRemove(mock.Anything, "owned-stress", ctypes.RemoveOptions{Force: true}).Return(nil).Once()

		result, err := start(ctx)
		require.NoError(t, err)
		<-waiting
		synctest.Wait()
		cancel()
		synctest.Wait()
		select {
		case <-result.Output:
			t.Fatal("output completed before the SDK terminal result")
		case <-result.Errors:
			t.Fatal("errors completed before the SDK terminal result")
		default:
		}
		close(release)
		_, err = collectStressResult(t, result)
		require.ErrorContains(t, err, "exited with code 7")
	})
}
