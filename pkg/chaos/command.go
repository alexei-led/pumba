package chaos

import (
	"context"
	"fmt"
	"strings"
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

// GlobalParams global parameters passed through CLI flags
type GlobalParams struct {
	Random     bool
	Labels     []string
	Pattern    string
	Names      []string
	Interval   time.Duration
	DryRun     bool
	SkipErrors bool
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
// subcommand-level Flags directly.
func ParseGlobalParams(c cliflags.Flags) *GlobalParams {
	g := c.Global()
	names, pattern := getNamesOrPattern(c)
	return &GlobalParams{
		Random:     g.Bool("random"),
		Labels:     splitLabels(g.StringSlice("label")),
		Pattern:    pattern,
		Names:      names,
		DryRun:     g.Bool("dry-run"),
		SkipErrors: g.Bool("skip-error"),
		Interval:   g.Duration("interval"),
	}
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

// RunChaosCommand run chaos command in go routine
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
	// cancel current context on exit
	defer cancel()
	// run chaos command
	for {
		// run chaos function
		if err := command.Run(ctx, params.Random); err != nil {
			if !params.SkipErrors {
				return fmt.Errorf("error running chaos command: %w", err)
			}
			log.WithError(err).Warn("skipping error")
		}
		// wait for next timer tick or cancel
		select {
		case <-topContext.Done():
			return nil // not to leak the goroutine
		case <-tick:
			if params.Interval == 0 {
				return nil // not to leak the goroutine
			}
			log.Debug("next chaos execution (tick) ...")
		}
	}
}
