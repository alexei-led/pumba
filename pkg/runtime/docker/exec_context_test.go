package docker

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRunExecContextStopsReadingAtOperationDeadline(t *testing.T) {
	api := NewMockEngine(t)
	attached, peer := net.Pipe()
	defer peer.Close()
	api.EXPECT().ContainerExecCreate(mock.Anything, "target", ctypes.ExecOptions{
		AttachStdout: true, AttachStderr: true, Cmd: []string{"iptables-nft", "-D", "INPUT"},
	}).Return(ctypes.ExecCreateResponse{ID: "bounded"}, nil).Once()
	api.EXPECT().ContainerExecAttach(mock.Anything, "bounded", ctypes.ExecAttachOptions{}).
		Return(types.HijackedResponse{Conn: attached, Reader: bufio.NewReader(attached)}, nil).Once()
	api.EXPECT().ContainerExecInspect(mock.Anything, "bounded").Return(ctypes.ExecInspect{}, nil).Maybe()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := (dockerClient{containerAPI: api}).runExecContext(ctx, "target", []string{"iptables-nft", "-D", "INPUT"}, false)
		done <- err
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		require.NoError(t, peer.Close())
		<-done
		t.Fatal("exec ignored the recovery operation deadline")
	}
}

func TestRunExecContextRejectsExpiredOperation(t *testing.T) {
	api := NewMockEngine(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, _, err := (dockerClient{containerAPI: api}).runExecContext(ctx, "target", []string{"iptables-nft", "-D", "INPUT"}, false)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
