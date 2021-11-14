package chaos

import (
	"context"
	"strings"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	// Re2Prefix re2 regexp string prefix
	Re2Prefix = "re2:"
)

var (
	// DockerClient Docker client instance
	DockerClient container.Client
)

// Command chaos command
type Command interface {
	Run(ctx context.Context, random bool) error
}

type GlobalParams struct {
	Random     bool
	Labels     []string
	Pattern    string
	Names      []string
	Interval   time.Duration
	DryRun     bool
	SkipErrors bool
}

func ParseGlobalParams(c *cli.Context) (*GlobalParams, error) {
	// get random flag
	random := c.GlobalBool("random")
	// get labels
	labels := c.GlobalStringSlice("label")
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get skip error flag
	skipError := c.GlobalBool("skip-error")
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// get global chaos interval
	interval, err := getIntervalValue(c.GlobalString("interval"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interval value")
	}
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

// get interval value from string duration
func getIntervalValue(interval string) (time.Duration, error) {
	// get recurrent time interval
	if interval == "" {
		return 0, nil
	}
	i, err := time.ParseDuration(interval)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse interval")
	}
	return i, nil
}

// get names list of filter pattern from command line
func getNamesOrPattern(c *cli.Context) ([]string, string) {
	names := []string{}
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
				pattern = strings.Trim(first, Re2Prefix)
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
				return errors.Wrap(err, "error running chaos command")
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
