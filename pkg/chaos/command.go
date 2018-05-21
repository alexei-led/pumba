package chaos

import (
	"context"
	"strings"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	// Re2Prefix re2 regexp string prefix
	Re2Prefix = "re2:"
)

var (
	// Docker client instance
	DockerClient container.Client
)

// Command chaos command
type Command interface {
	Run(ctx context.Context, random bool) error
}

// GetNamesOrPattern get names list of filter pattern from command line
func GetNamesOrPattern(c *cli.Context) ([]string, string) {
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
func RunChaosCommand(topContext context.Context, command Command, intervalStr string, random bool) error {
	// parse interval
	interval, err := util.GetIntervalValue(intervalStr)
	if err != nil {
		log.WithError(err).Error("failed to parse interval")
		return err
	}

	// create Time channel for specified interval
	var tick <-chan time.Time
	if interval == 0 {
		tick = time.NewTimer(interval).C
	} else {
		tick = time.NewTicker(interval).C
	}

	// handle the 'chaos' command
	ctx, cancel := context.WithCancel(topContext)
	// cancel current context on exit
	defer cancel()
	// run chaos command
	for {
		// run chaos function
		if err := command.Run(ctx, random); err != nil {
			log.WithError(err).Error("failed to run chaos command")
			return err
		}
		// wait for next timer tick or cancel
		select {
		case <-topContext.Done():
			return nil // not to leak the goroutine
		case <-tick:
			if interval == 0 {
				return nil // not to leak the goroutine
			}
			log.Debug("next chaos execution (tick) ...")
		}
	}
}
