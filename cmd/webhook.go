package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
)

const (
	// webhookTimeout bounds each POST and the exit flush.
	webhookTimeout = 5 * time.Second
	// webhookDrainLimit caps the response body read so the connection can be
	// reused without trusting the receiver's response size.
	webhookDrainLimit = 64 << 10
)

var (
	// Neither error names the URL: it may carry a token.
	errWebhookURL    = errors.New("invalid --webhook: use an absolute http:// or https:// URL")
	errWebhookStatus = errors.New("webhook answered with a non-2xx status")
)

// webhook POSTs each event as its pumba.event/v1 JSON line. Delivery is best
// effort: no retries, redirects are not followed, and the first drop or failure
// requests one best-effort warning that names neither the URL nor a body.
type webhook struct {
	url         string
	client      *http.Client
	stderr      io.Writer
	warnOnce    sync.Once
	warningDone chan struct{}
}

// newWebhookSink returns a sink that delivers events to rawURL in order from
// one goroutine, so a slow receiver drops events instead of blocking chaos.
func newWebhookSink(rawURL string, timeout time.Duration, stderr io.Writer, size int) *streamSink {
	h := &webhook{
		url: rawURL,
		client: &http.Client{
			Timeout: timeout,
			// Do not forward events to a redirect destination.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
		stderr:      stderr,
		warningDone: make(chan struct{}),
	}
	return startSink(h.post, h.warn, size, h.warningDone)
}

// enableWebhook validates --webhook and subscribes its sink to the fan-out.
func (o *eventOutput) enableWebhook(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	if err := validateWebhookURL(rawURL); err != nil {
		return chaos.NewUsageError(err)
	}
	// A broken stderr pipe must fail its warning write, not kill text-mode
	// chaos with SIGPIPE before detached cleanup can run.
	signal.Ignore(syscall.SIGPIPE)
	o.hook = newWebhookSink(rawURL, o.hookTimeout, o.stderr, eventQueueSize)
	o.fan.subs = append(o.fan.subs, o.hook)
	return nil
}

func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return errWebhookURL
	}
	return nil
}

func (h *webhook) post(ev *chaos.Event) error {
	var body bytes.Buffer
	if err := newEventEncoder(&body).Encode(ev); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, webhookDrainLimit))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return errWebhookStatus
	}
	return nil
}

func (h *webhook) warn() {
	h.warnOnce.Do(func() {
		// At most one process-lifetime writer: arbitrary stderr cannot be
		// canceled, but it must never hold Emit or extend bounded shutdown.
		go func() {
			defer close(h.warningDone)
			// Never print the delivery error: url.Error embeds the URL.
			_, _ = fmt.Fprintln(h.stderr, "warning: result events could not be delivered to --webhook (further warnings suppressed)")
		}()
	})
}
