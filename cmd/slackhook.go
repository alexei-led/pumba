package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	slackTimeout     = 5 * time.Second
	slackMaxInFlight = 16
	slackShortField  = 20
)

type slackHook struct {
	url, channel string
	threshold    log.Level
	client       *http.Client
	inFlight     chan struct{}
	warnOnce     sync.Once
}

type slackMessage struct {
	Text        string            `json:"text"`
	Username    string            `json:"username"`
	IconURL     string            `json:"icon_url"`
	IconEmoji   string            `json:"icon_emoji"`
	Channel     string            `json:"channel"`
	UnfurlLinks bool              `json:"unfurl_links"`
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Title    string       `json:"title"`
	Fallback string       `json:"fallback"`
	Text     string       `json:"text"`
	Pretext  string       `json:"pretext"`
	Color    string       `json:"color"`
	Fields   []slackField `json:"fields"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func newSlackHook(url, channel string, threshold log.Level) *slackHook {
	return &slackHook{
		url: url, channel: channel, threshold: threshold,
		client: &http.Client{
			Timeout: slackTimeout,
			// Do not forward log data to a redirect destination.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
		inFlight: make(chan struct{}, slackMaxInFlight),
	}
}

func (h *slackHook) Levels() []log.Level {
	return log.AllLevels[:int(h.threshold)+1]
}

func (h *slackHook) Fire(entry *log.Entry) error {
	// Best effort: bound concurrent sends rather than blocking chaos or growing
	// an unbounded queue. Normal shutdown drains pending sends.
	select {
	case h.inFlight <- struct{}{}:
	default:
		h.warn()
		return nil
	}
	attachment := slackAttachment{Text: entry.Message, Fallback: entry.Message, Color: "warning"}
	switch entry.Level {
	case log.DebugLevel:
		attachment.Color = "#9B30FF"
	case log.InfoLevel:
		attachment.Color = "good"
	case log.ErrorLevel, log.FatalLevel, log.PanicLevel:
		attachment.Color = "danger"
	case log.TraceLevel, log.WarnLevel:
	}
	if len(entry.Data) > 0 {
		attachment.Text, attachment.Pretext = "Message fields", entry.Message
		for key, value := range entry.Data {
			field := slackField{Title: key, Value: fmt.Sprint(value)}
			field.Short = len(field.Value) <= slackShortField
			attachment.Fields = append(attachment.Fields, field)
		}
	}
	// Snapshot entry data before returning: logrus may reuse the entry.
	body, err := json.Marshal(slackMessage{
		Username: "pumba_bot", Channel: h.channel, IconEmoji: ":boar:",
		Attachments: []slackAttachment{attachment},
	})
	if err != nil {
		<-h.inFlight
		h.warn()
		return err
	}
	go h.send(body)
	return nil
}

func (h *slackHook) send(body []byte) {
	defer func() { <-h.inFlight }()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		h.warn()
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		h.warn()
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.warn()
	}
}

func (h *slackHook) warn() {
	h.warnOnce.Do(func() {
		// Never log through logrus here, or expose a webhook URL or response body.
		fmt.Fprintln(os.Stderr, "warning: Slack log event could not be delivered (further warnings suppressed)")
	})
}

// flushSlackHooks runs after chaos has stopped, so one-shot commands retain
// notifications without waiting for the network during chaos execution.
func flushSlackHooks() {
	for _, hook := range log.StandardLogger().Hooks[log.PanicLevel] {
		if slack, ok := hook.(*slackHook); ok {
			slack.flush()
		}
	}
}

func (h *slackHook) flush() {
	timer := time.NewTimer(slackTimeout)
	defer timer.Stop()
	acquired := 0
	defer func() {
		for range acquired {
			<-h.inFlight
		}
	}()
	for range cap(h.inFlight) {
		select {
		case h.inFlight <- struct{}{}:
			acquired++
		case <-timer.C:
			h.warn()
			return
		}
	}
}
