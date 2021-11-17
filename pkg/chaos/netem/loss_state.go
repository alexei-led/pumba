package netem

import (
	"context"
	"strconv"
	"sync"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// `netem loss state` command
type lossStateCommand struct {
	netemCommand
	p13 float64
	p31 float64
	p32 float64
	p23 float64
	p14 float64
}

// NewLossStateCommand create new netem loss state command
func NewLossStateCommand(client container.Client,
	globalParams *chaos.GlobalParams,
	netemParams *Params,
	p13, // probability to go from state (1) to state (3)
	p31, // probability to go from state (3) to state (1)
	p32, // probability to go from state (3) to state (2)
	p23, // probability to go from state (2) to state (3)
	p14 float64, // probability to go from state (1) to state (4)
) (chaos.Command, error) {
	// validate p13
	if p13 < 0.0 || p13 > 100.0 {
		return nil, errors.New("invalid p13 percentage: : must be between 0.0 and 100.0")
	}
	// validate p31
	if p31 < 0.0 || p31 > 100.0 {
		return nil, errors.New("invalid p31 percentage: : must be between 0.0 and 100.0")
	}
	// validate p32
	if p32 < 0.0 || p32 > 100.0 {
		return nil, errors.New("invalid p32 percentage: : must be between 0.0 and 100.0")
	}
	// vaidate p23
	if p23 < 0.0 || p23 > 100.0 {
		return nil, errors.New("invalid p23 percentage: : must be between 0.0 and 100.0")
	}
	// validate p14
	if p14 < 0.0 || p14 > 100.0 {
		return nil, errors.New("invalid p14 percentage: : must be between 0.0 and 100.0")
	}

	return &lossStateCommand{
		netemCommand: newNetemCommand(client, globalParams, netemParams),
		p13:          p13,
		p31:          p31,
		p32:          p32,
		p23:          p23,
		p14:          p14,
	}, nil
}

// Run netem loss state command
func (n *lossStateCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network packet loss according 4-state Markov model to all matching containers")
	log.WithFields(log.Fields{
		"names":   n.names,
		"pattern": n.pattern,
		"labels":  n.labels,
		"limit":   n.limit,
		"random":  random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, n.client, n.names, n.pattern, n.labels, n.limit)
	if err != nil {
		return err
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

	// prepare netem loss state command
	netemCmd := []string{"loss", "state", strconv.FormatFloat(n.p13, 'f', 2, 64)}
	netemCmd = append(netemCmd, strconv.FormatFloat(n.p31, 'f', 2, 64), strconv.FormatFloat(n.p32, 'f', 2, 64), strconv.FormatFloat(n.p23, 'f', 2, 64), strconv.FormatFloat(n.p14, 'f', 2, 64))

	// run netem loss command for selected containers
	var wg sync.WaitGroup
	errs := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
		}).Debug("adding network 4-state packet loss for container")
		netemCtx, cancel := context.WithTimeout(ctx, n.duration)
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, c *container.Container) {
			defer wg.Done()
			errs[i] = runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.sports, n.dports, n.duration, n.image, n.pull, n.dryRun)
			if errs[i] != nil {
				log.WithError(errs[i]).Warn("failed to set packet loss for container")
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
			return errors.Wrap(err, "failed to add packet loss (4-state Markov model) for one or more containers")
		}
	}

	return nil
}
