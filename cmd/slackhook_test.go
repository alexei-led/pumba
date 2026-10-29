package main

import (
	"encoding/json"
	"io"
	"sync/atomic"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackHookPayload(t *testing.T) {
	for _, tc := range []struct {
		level log.Level
		color string
	}{
		{log.TraceLevel, "warning"},
		{log.DebugLevel, "#9B30FF"},
		{log.InfoLevel, "good"},
		{log.WarnLevel, "warning"},
		{log.ErrorLevel, "danger"},
		{log.FatalLevel, "danger"},
		{log.PanicLevel, "danger"},
	} {
		for _, withFields := range []bool{false, true} {
			name := tc.level.String()
			if withFields {
				name += "/fields"
			}
			t.Run(name, func(t *testing.T) {
				bodies := make(chan []byte, 1)
				requests := make(chan [2]string, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					bodies <- body
					requests <- [2]string{r.Method, r.Header.Get("Content-Type")}
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()
				hook := newSlackHook(server.URL, "#chaos", log.TraceLevel)
				defer hook.flush()
				logger := log.New()
				logger.SetOutput(io.Discard)
				entry := log.NewEntry(logger)
				entry.Level, entry.Message = tc.level, "chaos event"
				if withFields {
					entry.Data = log.Fields{"id": 42, "long": "longer than twenty characters"}
				}
				require.NoError(t, hook.Fire(entry))
				waitSlackHook(t, hook)
				var body []byte
				select {
				case body = <-bodies:
				case <-time.After(time.Second):
					t.Fatal("webhook payload not received")
				}
				assert.Equal(t, [2]string{http.MethodPost, "application/json"}, <-requests)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(body, &payload))
				fields := "null"
				text, pretext := "chaos event", ""
				if withFields {
					text, pretext = "Message fields", "chaos event"
					fields = `[{"title":"id","value":"42","short":true},{"title":"long","value":"longer than twenty characters","short":false}]`
					attachments := payload["attachments"].([]any)
					attachment := attachments[0].(map[string]any)
					actualFields := attachment["fields"].([]any)
					assert.ElementsMatch(t, []any{
						map[string]any{"title": "id", "value": "42", "short": true},
						map[string]any{"title": "long", "value": "longer than twenty characters", "short": false},
					}, actualFields)
					attachment["fields"] = json.RawMessage(fields)
				}
				expected := `{"text":"","username":"pumba_bot","icon_url":"","icon_emoji":":boar:","channel":"#chaos","unfurl_links":false,"attachments":[{"title":"","fallback":"chaos event","text":"` + text + `","pretext":"` + pretext + `","color":"` + tc.color + `","fields":` + fields + `}]} `
				actual, err := json.Marshal(payload)
				require.NoError(t, err)
				assert.JSONEq(t, expected, string(actual))
				if withFields {
					assert.Equal(t, log.Fields{"id": 42, "long": "longer than twenty characters"}, entry.Data)
				}
			})
		}
	}
}

func waitSlackHook(t *testing.T, hook *slackHook) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for range cap(hook.inFlight) {
		select {
		case hook.inFlight <- struct{}{}:
		case <-timer.C:
			t.Fatal("webhook sends did not finish within one second")
		}
	}
	for range cap(hook.inFlight) {
		<-hook.inFlight
	}
}

func TestSlackHookThreshold(t *testing.T) {
	for _, threshold := range log.AllLevels {
		t.Run(threshold.String(), func(t *testing.T) {
			var posted atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				posted.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			hook := newSlackHook(server.URL, "#pumba", threshold)
			logger := log.New()
			logger.AddHook(hook)
			for _, level := range log.AllLevels {
				t.Run(level.String(), func(t *testing.T) {
					before := posted.Load()
					entry := log.NewEntry(logger)
					entry.Level, entry.Message = level, "event"
					require.NoError(t, logger.Hooks.Fire(level, entry))
					waitSlackHook(t, hook)
					want := before
					if level <= threshold {
						want++
					}
					assert.Equal(t, want, posted.Load())
				})
			}
		})
	}
}

func TestSlackHookFailures(t *testing.T) {
	for _, mode := range []string{"500", "closed", "timeout", "redirect", "invalid-url"} {
		t.Run(mode, func(t *testing.T) {
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				switch mode {
				case "500":
					http.Error(w, "secret response body", http.StatusInternalServerError)
				case "closed":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
				case "timeout":
					<-release
				case "redirect":
					w.Header().Set("Location", "/redirected")
					w.WriteHeader(http.StatusFound)
				}
			}))
			defer server.Close()
			defer close(release)
			hook := newSlackHook(server.URL, "#pumba", log.DebugLevel)
			hook.client.Timeout = 50 * time.Millisecond
			if mode == "invalid-url" {
				hook.url = "://invalid"
			}
			entry := log.NewEntry(log.New())
			entry.Level, entry.Message = log.ErrorLevel, "event"
			returned := make(chan error, 1)
			go func() { returned <- hook.Fire(entry) }()
			select {
			case err := <-returned:
				require.NoError(t, err)
			case <-time.After(time.Second):
				t.Fatal("webhook blocked logging")
			}
			waitSlackHook(t, hook)
		})
	}
}

func TestSlackHookBoundedSends(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, slackMaxInFlight)
	var posted atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		posted.Add(1)
		started <- struct{}{}
		<-release
	}))
	defer server.Close()
	t.Cleanup(func() { server.CloseClientConnections() })
	hook := newSlackHook(server.URL, "#pumba", log.DebugLevel)
	entry := log.NewEntry(log.New())
	entry.Level = log.InfoLevel
	for range slackMaxInFlight {
		require.NoError(t, hook.Fire(entry))
	}
	for range slackMaxInFlight {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("webhook did not start")
		}
	}
	require.NoError(t, hook.Fire(entry))
	close(release)
	waitSlackHook(t, hook)
	assert.EqualValues(t, slackMaxInFlight, posted.Load())
}

func TestSetupLoggingWithoutSlackURL(t *testing.T) {
	logger := log.StandardLogger()
	oldHooks := logger.ReplaceHooks(make(log.LevelHooks))
	oldLevel, oldFormatter := logger.GetLevel(), logger.Formatter
	t.Cleanup(func() {
		logger.ReplaceHooks(oldHooks)
		logger.SetLevel(oldLevel)
		logger.SetFormatter(oldFormatter)
	})
	setupLogging(cliflags.NewV1(newRuntimeTestContext(t, map[string]string{})))
	assert.Empty(t, logger.Hooks)
}

func TestSlackHookFlush(t *testing.T) {
	posted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posted <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	logger := log.StandardLogger()
	oldHooks := logger.ReplaceHooks(make(log.LevelHooks))
	t.Cleanup(func() { logger.ReplaceHooks(oldHooks) })
	hook := newSlackHook(server.URL, "#pumba", log.WarnLevel)
	logger.AddHook(hook)
	entry := log.NewEntry(logger)
	entry.Level, entry.Message = log.WarnLevel, "one shot"
	require.NoError(t, hook.Fire(entry))
	flushSlackHooks()
	select {
	case <-posted:
	default:
		t.Fatal("shutdown did not drain the webhook")
	}
	assert.Empty(t, hook.inFlight)
}
