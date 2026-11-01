//go:build !windows

package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	webhookPipeTestURL = "PUMBA_TEST_WEBHOOK_PIPE_URL"
	webhookPipeCleanup = "PUMBA_TEST_WEBHOOK_PIPE_CLEANUP"
)

func TestWebhookClosedStderrCannotBypassCleanup(t *testing.T) {
	if url := os.Getenv(webhookPipeTestURL); url != "" {
		signal.Reset(syscall.SIGPIPE)
		o := newEventOutput(io.Discard, os.Stderr, "test")
		require.NoError(t, o.enableWebhook(url))
		defer o.hook.flush(nil, time.Second)
		defer func() {
			require.NoError(t, os.WriteFile(os.Getenv(webhookPipeCleanup), []byte("reverted"), 0o600))
		}()
		o.hook.Emit(appliedEvent(1))
		select {
		case <-o.hook.warningDone:
		case <-time.After(3 * time.Second):
			t.Fatal("warning was not attempted")
		}
		return
	}
	server := newReceiver(t, http.StatusInternalServerError)
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	t.Cleanup(func() { _ = writer.Close() })
	cleanup := filepath.Join(t.TempDir(), "cleanup")
	executable, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.Command(executable, "-test.run=^TestWebhookClosedStderrCannotBypassCleanup$")
	cmd.Env = append(os.Environ(), webhookPipeTestURL+"="+server.URL, webhookPipeCleanup+"="+cleanup)
	cmd.Stderr = writer
	require.NoError(t, cmd.Run(), "closed stderr must not kill the process before its deferred cleanup")
	result, err := os.ReadFile(cleanup)
	require.NoError(t, err)
	require.Equal(t, "reverted", string(result))
}
