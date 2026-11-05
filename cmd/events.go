package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// --output values.
const (
	outputText   = "text"
	outputNDJSON = "ndjson"
)

const (
	// eventQueueSize bounds the events buffered for stdout. A consumer that
	// falls further behind loses events (counted) instead of stalling chaos.
	eventQueueSize = 1024
	// eventFlushTimeout bounds the wait for run.end to reach stdout at exit.
	eventFlushTimeout = 2 * time.Second
)

// eventFanout hands every event to each subscriber. Subscribers are added in
// before(), ahead of the first Emit; they must not block or modify the event.
type eventFanout struct {
	subs []chaos.EventSink
}

func (f *eventFanout) Emit(ev *chaos.Event) {
	for _, s := range f.subs {
		s.Emit(ev)
	}
}

// streamSink delivers events from its own goroutine through a bounded queue,
// in order, so a stalled consumer never blocks Emit. Events that do not fit,
// arrive after stop, or fail to deliver are counted as dropped.
type streamSink struct {
	deliver     func(*chaos.Event) error
	onDrop      func()          // optional, must not block; called on every drop
	warningDone <-chan struct{} // optional asynchronous warning completion
	queue       chan *chaos.Event
	done        chan struct{}
	dropped     atomic.Int64

	mu     sync.Mutex
	closed bool
	last   *chaos.Event
}

// newStreamSink writes events to w as NDJSON.
func newStreamSink(w io.Writer, size int) *streamSink {
	enc := newEventEncoder(w)
	return startSink(func(ev *chaos.Event) error { return enc.Encode(ev) }, nil, size, nil)
}

func newEventEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

func startSink(deliver func(*chaos.Event) error, onDrop func(), size int, warningDone <-chan struct{}) *streamSink {
	s := &streamSink{deliver: deliver, onDrop: onDrop, warningDone: warningDone, queue: make(chan *chaos.Event, size), done: make(chan struct{})}
	go s.write()
	return s
}

func (s *streamSink) Emit(ev *chaos.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		s.drop()
		return
	}
	select {
	case s.queue <- ev:
	default:
		s.drop()
	}
}

func (s *streamSink) drop() {
	s.dropped.Add(1)
	if s.onDrop != nil {
		s.onDrop()
	}
}

func (s *streamSink) write() {
	defer close(s.done)
	for ev := range s.queue {
		s.send(ev)
	}
	// flush sets last before closing the queue, so it is visible here. The
	// drop count is read only now so failures while draining the queue are in
	// it; a failure delivering this event itself cannot be.
	if s.last != nil {
		if s.last.End != nil {
			s.last.End.EventsDropped = s.dropped.Load()
		}
		s.send(s.last)
	}
	// Normal flush observes its warning; a blocked stderr remains bounded by
	// the caller's existing flush timeout, never the event producer.
	if s.warningDone != nil && s.dropped.Load() > 0 {
		<-s.warningDone
	}
}

func (s *streamSink) send(ev *chaos.Event) {
	if err := s.deliver(ev); err != nil {
		s.drop()
	}
}

// flush rejects further events, writes last after every queued event with
// End.EventsDropped set to the drops counted by then, and waits up to timeout
// for the writer to finish. It reports whether the writer finished. Call it once.
func (s *streamSink) flush(last *chaos.Event, timeout time.Duration) bool {
	s.mu.Lock()
	s.closed = true
	s.last = last
	close(s.queue)
	s.mu.Unlock()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-s.done:
		return true
	case <-timer.C:
		return false
	}
}

// eventOutput owns result events for one process: the Recorder that counts
// results before delivery, the fan-out to delivery sinks, and run.end.
type eventOutput struct {
	stdout, stderr io.Writer
	fan            eventFanout
	rec            *chaos.Recorder
	stream         *streamSink // stdout sink; nil in text mode
	hook           *streamSink // --webhook sink; nil without --webhook
	runID          string
	flushTimeout   time.Duration
	hookTimeout    time.Duration // per request and for the exit flush
}

func newEventOutput(stdout, stderr io.Writer, version string) *eventOutput {
	o := &eventOutput{stdout: stdout, stderr: stderr, flushTimeout: eventFlushTimeout, hookTimeout: webhookTimeout}
	o.rec = chaos.NewRecorder(&o.fan, version)
	return o
}

// configure applies --output, resolves --run-id, writing a generated ID back
// so chaos.ParseGlobalParams sees the same value, and applies --webhook.
// before() calls it first, so later usage errors still reach event consumers.
func (o *eventOutput) configure(c *cli.Context) error {
	switch mode := c.GlobalString("output"); mode {
	case outputText:
		o.fan.subs = append(o.fan.subs, statusText{w: o.stdout}, &planText{w: o.stdout}, recoveryText{w: o.stdout})
	case outputNDJSON:
		o.enableNDJSON(c.App)
	default:
		return chaos.NewUsageError(fmt.Errorf("invalid --output %q: use %s or %s", mode, outputText, outputNDJSON))
	}
	id, err := chaos.ResolveRunID(c.GlobalString("run-id"))
	if err != nil {
		return chaos.NewUsageError(err)
	}
	o.runID = id
	if err := c.GlobalSet("run-id", id); err != nil {
		return err
	}
	if err := o.enableWebhook(c.GlobalString("webhook")); err != nil {
		return err
	}
	if o.hook != nil {
		c.App.ExitErrHandler = func(*cli.Context, error) {}
	}
	return nil
}

// enableNDJSON reserves stdout for events: help and usage text move to
// stderr, and urfave's exit-on-error is disabled so its errors still end in
// run.end. SIGPIPE is ignored so a closed stdout fails writes (counted as
// drops) instead of killing Pumba with faults still active.
func (o *eventOutput) enableNDJSON(app *cli.App) {
	app.Writer = o.stderr
	app.ExitErrHandler = func(*cli.Context, error) {}
	signal.Ignore(syscall.SIGPIPE)
	o.stream = newStreamSink(o.stdout, eventQueueSize)
	o.fan.subs = append(o.fan.subs, o.stream)
}

// onUsageError tags flag parse errors as usage errors and prints the same
// usage text urfave/cli prints by default. App-level parse errors happen
// before before(), so --output and --run-id are applied here from whatever
// was parsed ahead of the bad flag.
func (o *eventOutput) onUsageError(c *cli.Context, err error, isSubcommand bool) error {
	w := c.App.Writer
	switch {
	case isSubcommand:
		_, _ = fmt.Fprintf(w, "%s %s\n\n", "Incorrect Usage.", err.Error())
		_ = cli.ShowSubcommandHelp(c)
	case c.Command.Name != "":
		_, _ = fmt.Fprintln(w, "Incorrect Usage:", err.Error())
		_, _ = fmt.Fprintln(w)
		_ = cli.ShowCommandHelp(c, c.Command.Name)
	default:
		_ = o.configure(c) // best effort: the parse error is what gets reported
		// configure may have moved the writer to stderr (ndjson)
		_, _ = fmt.Fprintf(c.App.Writer, "%s %s\n\n", "Incorrect Usage.", err.Error())
		_ = cli.ShowAppHelp(c)
	}
	return chaos.NewUsageError(err)
}

// finish computes the outcome from the recorded results and err, delivers
// run.end best-effort, and returns the exit code. Delivery never changes the
// outcome. Successful help, version and schema output are not runs: no run.end.
func (o *eventOutput) finish(err error, aborted bool) int {
	if _, ok := errors.AsType[cli.ExitCoder](err); ok { // urfave help errors, e.g. an unknown command
		err = chaos.NewUsageError(err)
	}
	ev, out := o.rec.RunEnd(err, aborted)
	var last *chaos.Event
	if ev.RunID != "" || err != nil {
		if ev.RunID == "" {
			ev.RunID = o.runID
		}
		last = &ev
	}
	o.flush(last)
	return out.ExitCode
}

// flush delivers run.end to every sink concurrently, each within its own
// bound. A nil end closes unused sinks without emitting an event. The webhook
// gets its own copy: each sink writes its own drop count into run.end.
func (o *eventOutput) flush(end *chaos.Event) {
	var wg sync.WaitGroup
	if o.hook != nil {
		var hookEnd *chaos.Event
		if end != nil {
			event := *end
			if end.End != nil {
				body := *end.End
				event.End = &body
			}
			hookEnd = &event
		}
		wg.Go(func() {
			if !o.hook.flush(hookEnd, o.hookTimeout) {
				o.hook.onDrop()
			}
		})
	}
	if o.stream != nil && !o.stream.flush(end, o.flushTimeout) {
		// One best-effort warning per run: stderr may share stdout's stalled
		// destination, so its write must not extend bounded shutdown.
		go log.Warn("timed out writing result events to stdout")
	}
	wg.Wait()
}

// setUsageErrorHandler installs h on every command and subcommand.
func setUsageErrorHandler(cmds []cli.Command, h cli.OnUsageErrorFunc) {
	for i := range cmds {
		cmds[i].OnUsageError = h
		setUsageErrorHandler(cmds[i].Subcommands, h)
	}
}
