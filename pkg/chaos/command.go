package chaos

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	// Re2Prefix re2 regexp string prefix
	Re2Prefix = "re2:"
)

var (
	// ContainerClient container runtime client instance
	// TODO(Phase 4): remove this global and inject client via dependency injection
	ContainerClient container.Client
)

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
		for _, part := range strings.Split(l, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

// ParseGlobalParams parse global parameters
func ParseGlobalParams(c *cli.Context) (*GlobalParams, error) {
	// get random flag
	random := c.GlobalBool("random")
	// get labels; support both --label k=v --label k2=v2 and --label k=v,k2=v2
	labels := splitLabels(c.GlobalStringSlice("label"))
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get skip error flag
	skipError := c.GlobalBool("skip-error")
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// get global chaos interval
	interval := c.GlobalDuration("interval")
	return &GlobalParams{
		Random:     random,
		Labels:     labels,
		Pattern:    pattern,
		Names:      names,
		DryRun:     dryRun,
		SkipErrors: skipError,
		Interval:   interval,
	}, nil
}

// get names list of filter pattern from command line
func getNamesOrPattern(c *cli.Context) ([]string, string) {
	var names []string
	pattern := ""
	// get container names or pattern: no Args means ALL containers
	if c.Args().Present() {
		// more than one argument, assume that this a list of names
		if len(c.Args()) > 1 {
			names = c.Args()
			log.WithField("names", names).Debug("using names")
		} else {
			first := c.Args().First()
			if strings.HasPrefix(first, Re2Prefix) {
				pattern = strings.TrimPrefix(first, Re2Prefix)
				log.WithField("pattern", pattern).Debug("using pattern")
			} else {
				names = append(names, first)
				log.WithField("names", names).Debug("using names")
			}
		}
	}
	return names, pattern
}

// RunChaosCommand run chaos command in go routine
func RunChaosCommand(topContext context.Context, command Command, params *GlobalParams) error {
	// create Time channel for specified interval
	var tick <-chan time.Time
	if params.Interval == 0 {
		tick = time.NewTimer(params.Interval).C
	} else {
		tick = time.NewTicker(params.Interval).C
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
