package chaos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

const (
	// Re2Prefix re2 regexp string prefix
	Re2Prefix = "re2:"
)

// Runtime returns the container client to use for chaos execution. Builders
// receive a Runtime factory rather than a client value so that client
// construction can be deferred until after global flag parsing while still
// keeping the dependency visible in every constructor signature.
type Runtime func() container.Client

// Command chaos command
type Command interface {
	Run(ctx context.Context, random bool) error
}

// GlobalParams global parameters passed through CLI flags. Actions keep the
// pointer they receive (never a copy): it carries the run identity, the event
// sink and the per-run tick counter.
type GlobalParams struct {
	MaxTargets int
	Random     bool
	Labels     []string
	Pattern    string
	Names      []string
	Interval   time.Duration
	// MaxDuration bounds active chaos and interval scheduling (0: unlimited).
	MaxDuration time.Duration
	DryRun      bool
	SkipErrors  bool
	// RunID names this invocation; shared by every interval tick.
	RunID string
	// Runtime is the --runtime name, reported in run.start.
	Runtime string
	// Action is the CLI command path (e.g. "netem delay") stamped on events.
	Action string
	// Events receives result events. ParseGlobalParams sets a no-op sink; a
	// nil sink discards events.
	Events EventSink

	ticks atomic.Int64
}

// splitLabels splits comma-separated label values into individual labels.
// This supports both "--label k1=v1 --label k2=v2" and "--label k1=v1,k2=v2" syntax.
func splitLabels(raw []string) []string {
	var result []string
	for _, l := range raw {
		for part := range strings.SplitSeq(l, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

// ParseGlobalParams parses application-level flags from any cliflags.Flags
// implementation. Reads global flags via c.Global() so the caller may pass the
// subcommand-level Flags directly. An empty --run-id gets a generated one; an
// invalid one is a usage error.
func ParseGlobalParams(c cliflags.Flags) (*GlobalParams, error) {
	g := c.Global()
	runID, err := ResolveRunID(g.String("run-id"))
	if err != nil {
		return nil, NewUsageError(err)
	}
	maxDuration := g.Duration("max-duration")
	if maxDuration < 0 {
		return nil, NewUsageError(errors.New("--max-duration must be non-negative"))
	}
	names, pattern := getNamesOrPattern(c)
	return &GlobalParams{
		MaxTargets:  g.Int("max-targets"),
		Random:      g.Bool("random"),
		Labels:      splitLabels(g.StringSlice("label")),
		Pattern:     pattern,
		Names:       names,
		DryRun:      g.Bool("dry-run"),
		SkipErrors:  g.Bool("skip-error"),
		Interval:    g.Duration("interval"),
		MaxDuration: maxDuration,
		RunID:       runID,
		Runtime:     g.String("runtime"),
		Events:      NopSink{},
	}, nil
}

// getNamesOrPattern splits CLI arguments into exact names and RE2 patterns.
// Multiple patterns and names may be mixed. Patterns are joined into one RE2
// expression so GlobalParams.Pattern remains the canonical, compatible
// selection path for all chaos commands.
func getNamesOrPattern(c cliflags.Flags) ([]string, string) {
	var names, patterns []string
	for _, arg := range c.Args() {
		if pattern, found := strings.CutPrefix(arg, Re2Prefix); found {
			patterns = append(patterns, pattern)
			continue
		}
		names = append(names, arg)
	}

	pattern := joinPatterns(patterns)
	if len(names) > 0 && len(patterns) == 1 && patterns[0] == "" {
		// Preserve the empty RE2 selector's match-all semantics when it is mixed
		// with exact names. An empty Pattern means "no pattern" to the filter.
		pattern = "()"
	}
	if len(names) > 0 {
		log.WithField("names", names).Debug("using names")
	}
	if pattern != "" {
		log.WithField("pattern", pattern).Debug("using pattern")
	}
	return names, pattern
}

// joinPatterns creates a regular-expression union while preserving each
// selector's boundaries. A single pattern is left untouched for compatibility.
func joinPatterns(patterns []string) string {
	switch len(patterns) {
	case 0:
		return ""
	case 1:
		return patterns[0]
	default:
		return "(" + strings.Join(patterns, ")|(") + ")"
	}
}

// ErrMaxDurationExceeded marks a run stopped by its --max-duration budget.
var ErrMaxDurationExceeded = errors.New("--max-duration exceeded")

// RunChaosCommand runs command once, or on every --interval tick until the
// top context is canceled. A positive MaxDuration puts a deadline on the
// context the command runs with: in-flight work sees it as cancellation (its
// cleanup runs on detached contexts), no new tick starts after it, and the run
// returns ErrMaxDurationExceeded.
func RunChaosCommand(topContext context.Context, command Command, params *GlobalParams) error {
	// create Time channel for specified interval
	var tick <-chan time.Time
	if params.Interval == 0 {
		timer := time.NewTimer(params.Interval)
		defer timer.Stop()
		tick = timer.C
	} else {
		ticker := time.NewTicker(params.Interval)
		defer ticker.Stop()
		tick = ticker.C
	}

	// handle the 'chaos' command
	ctx, cancel := context.WithCancel(topContext)
	if params.MaxDuration > 0 {
		ctx, cancel = context.WithTimeoutCause(topContext, params.MaxDuration, ErrMaxDurationExceeded)
	}
	// cancel current context on exit
	defer cancel()
	budgetErr := func(err error) error {
		return errors.Join(fmt.Errorf("%w (%s)", ErrMaxDurationExceeded, params.MaxDuration), err)
	}
	// run chaos command
	for {
		// no new tick starts once the budget is spent
		if errors.Is(context.Cause(ctx), ErrMaxDurationExceeded) {
			return budgetErr(nil)
		}
		// run chaos function
		err := command.Run(ctx, params.Random)
		if errors.Is(context.Cause(ctx), ErrMaxDurationExceeded) {
			return budgetErr(err)
		}
		if err != nil {
			if !params.SkipErrors || errors.Is(err, ErrMaxTargetsExceeded) {
				return fmt.Errorf("error running chaos command: %w", err)
			}
			log.WithError(err).Warn("skipping error")
			if r, ok := params.Events.(skippedErrorRecorder); ok {
				r.RecordSkipped(err)
			}
		}
		// wait for next timer tick or cancel
		select {
		case <-ctx.Done():
			if errors.Is(context.Cause(ctx), ErrMaxDurationExceeded) {
				return budgetErr(nil)
			}
			return nil // not to leak the goroutine
		case <-tick:
			if params.Interval == 0 {
				return nil // not to leak the goroutine
			}
			log.Debug("next chaos execution (tick) ...")
		}
	}
}
