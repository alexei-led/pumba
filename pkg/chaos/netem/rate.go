package netem

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// Parse rate
func parseRate(rate string) (string, error) {
	reRate := regexp.MustCompile(`\d+[gmk]?bit`)
	validRate := reRate.FindString(rate)
	if rate != validRate {
		return "", fmt.Errorf("invalid rate, must match '%s'", reRate.String())
	}
	return rate, nil
}

// `netem rate` command
type rateCommand struct {
	netemCommand
	rate           string
	packetOverhead int
	cellSize       int
	cellOverhead   int
}

// NewRateCommand create new netem rate command
func NewRateCommand(client netemClient,
	globalParams *chaos.GlobalParams,
	netemParams *Params,
	rate string, // delay outgoing packets; in common units
	packetOverhead, // per packet overhead; in bytes
	cellSize, // cell size of the simulated link layer scheme
	cellOverhead int, // per cell overhead; in bytes
) (chaos.Command, error) {
	// validate target egress rate
	if rate == "" {
		return nil, errors.New("undefined rate limit")
	}
	rate, err := parseRate(rate)
	if err != nil {
		return nil, fmt.Errorf("invalid rate: %w", err)
	}

	// validate cell size
	if cellSize < 0 {
		return nil, errors.New("invalid cell size: must be a non-negative integer")
	}

	return &rateCommand{
		netemCommand:   newNetemCommand(client, globalParams, netemParams),
		rate:           rate,
		packetOverhead: packetOverhead,
		cellSize:       cellSize,
		cellOverhead:   cellOverhead,
	}, nil
}

// Run netem rate command
func (n *rateCommand) Run(ctx context.Context, random bool) error {
	log.Debug("setting network rate to all matching containers")
	log.WithFields(log.Fields{
		"names":   n.names,
		"pattern": n.pattern,
		"labels":  n.labels,
		"limit":   n.limit,
		"random":  random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, n.client, n.names, n.pattern, n.labels, n.limit)
	if err != nil {
		return fmt.Errorf("error listing containers: %w", err)
	}
	if len(containers) == 0 {
		log.Warning("no containers found")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []*container.Container{c}
		}
	}

	// prepare netem rate command
	netemCmd := []string{"rate", n.rate}
	if n.packetOverhead != 0 {
		netemCmd = append(netemCmd, strconv.Itoa(n.packetOverhead))
	}
	if n.cellSize > 0 {
		netemCmd = append(netemCmd, strconv.Itoa(n.cellSize))
	}
	if n.cellOverhead != 0 {
		netemCmd = append(netemCmd, strconv.Itoa(n.cellOverhead))
	}

	// run netem loss command for selected containers
	var wg sync.WaitGroup
	errs := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
			"command":   netemCmd,
		}).Debug("setting network rate for container")
		netemCtx, cancel := context.WithTimeout(ctx, n.duration)
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, c *container.Container) {
			defer wg.Done()
			errs[i] = runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.sports, n.dports, n.duration, n.image, n.pull, n.dryRun)
			if errs[i] != nil {
				log.WithError(errs[i]).Warn("failed to set network rate for container")
			}
		}(i, c)
	}

	// Wait for all netem delay commands to complete
	wg.Wait()

	// cancel context to avoid leaks
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	// scan through all errors in goroutines
	for _, err = range errs {
		// take first found error
		if err != nil {
			return fmt.Errorf("failed to set network rate for one or more containers: %w", err)
		}
	}

	return nil
}
