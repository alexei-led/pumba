package cmd

import (
	"context"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/alexei-led/pumba/pkg/container"
)

const (
	// Re2Prefix re2 regexp string prefix
	Re2Prefix = "re2:"
)

type commandContext struct {
	client  container.Client
	context context.Context
}

func getIntervalValue(c *cli.Context) (time.Duration, error) {
	// get recurrent time interval
	if intervalString := c.GlobalString("interval"); intervalString == "" {
		log.Debug("no interval specified, running only once")
		return 0, nil
	} else if interval, err := time.ParseDuration(intervalString); err == nil {
		return interval, nil
	} else {
		return 0, err
	}
}

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

func runChaosCommandX(topContext context.Context, command docker.ChaosCommand, interval time.Duration, random bool) {
	// create Time channel for specified interval
	var tick <-chan time.Time
	if interval == 0 {
		tick = time.NewTimer(interval).C
	} else {
		tick = time.NewTicker(interval).C
	}

	// handle the 'chaos' command
	ctx, cancel := context.WithCancel(topContext)
	for {
		// cancel current context on exit
		defer cancel()
		// run chaos function
		if err := command.Run(ctx, random); err != nil {
			log.WithError(err).Error("failed to run chaos command")
		}
		// wait for next timer tick or cancel
		select {
		case <-topContext.Done():
			return // not to leak the goroutine
		case <-tick:
			if interval == 0 {
				return // not to leak the goroutine
			}
			log.Debug("next chaos execution (tick) ...")
		}
	}
}
