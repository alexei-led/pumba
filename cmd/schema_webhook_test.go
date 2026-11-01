package main

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSchemaClosesWebhookWithoutEvents(t *testing.T) {
	for _, mode := range []string{outputText, outputNDJSON} {
		t.Run(mode, func(t *testing.T) {
			noRuntime(t)
			receiver := newReceiver(t, http.StatusNoContent)
			var stdout bytes.Buffer
			out := newEventOutput(&stdout, io.Discard, version)
			err := newApp(out).Run([]string{"pumba", "-o", mode, "--webhook", receiver.URL, "schema"})
			require.NoError(t, err)
			t.Cleanup(func() {
				if out.hook != nil && !out.hook.closed {
					out.hook.flush(nil, time.Second)
				}
				if out.stream != nil && !out.stream.closed {
					out.stream.flush(nil, time.Second)
				}
			})
			require.Zero(t, out.finish(nil, false))
			require.Equal(t, cliSchema, decodeSchema(t, stdout.String()).Schema)
			require.NotContains(t, stdout.String(), receiver.URL)
			require.Empty(t, receiver.received())
			select {
			case <-out.hook.done:
			default:
				t.Fatal("schema left the unused webhook sender open")
			}
			if out.stream != nil {
				select {
				case <-out.stream.done:
				default:
					t.Fatal("schema left the unused NDJSON writer open")
				}
			}
		})
	}
}
