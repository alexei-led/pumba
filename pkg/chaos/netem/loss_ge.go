package netem

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// netem loss gemodel` (Gilbert-Elliot model) command
type lossGECommand struct {
	netemCommand
	pg   float64
	pb   float64
	oneH float64
	oneK float64
}

// NewLossGECommand create new netem loss gemodel (Gilbert-Elliot) command
func NewLossGECommand(client container.Client,
	globalParams *chaos.GlobalParams,
	netemParams *Params,
	pg, // Good State transition probability
	pb, // Bad State transition probability
	oneH, // loss probability in Bad state
	oneK float64, // loss probability in Good state
) (chaos.Command, error) {
	// get pg - Good State transition probability
	if pg < 0.0 || pg > 100.0 {
		return nil, errors.New("invalid pg (Good State) transition probability: must be between 0.0 and 100.0")
	}
	// get pb - Bad State transition probability
	if pb < 0.0 || pb > 100.0 {
		return nil, errors.New("invalid pb (Bad State) transition probability: must be between 0.0 and 100.0")
	}
	// get (1-h) - loss probability in Bad state
	if oneH < 0.0 || oneH > 100.0 {
		return nil, errors.New("invalid loss probability: must be between 0.0 and 100.0")
	}
	// get (1-k) - loss probability in Good state
	if oneK < 0.0 || oneK > 100.0 {
		return nil, errors.New("invalid loss probability: must be between 0.0 and 100.0")
	}

	return &lossGECommand{
		netemCommand: newNetemCommand(client, globalParams, netemParams),
		pg:           pg,
		pb:           pb,
		oneH:         oneH,
		oneK:         oneK,
	}, nil
}

// Run netem loss state command
func (n *lossGECommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network packet loss according Gilbert-Elliot model to all matching containers")
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

	// prepare netem loss gemodel command
	netemCmd := []string{"loss", "gemodel", strconv.FormatFloat(n.pg, 'f', 2, 64)}
	netemCmd = append(netemCmd,
		strconv.FormatFloat(n.pb, 'f', 2, 64),
		strconv.FormatFloat(n.oneH, 'f', 2, 64),
		strconv.FormatFloat(n.oneK, 'f', 2, 64))

	// run netem loss command for selected containers
	var wg sync.WaitGroup
	errs := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))

	//nolint:dupl
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
		}).Debug("adding network random packet loss for container")
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
	for _, err := range errs {
		// take first found error
		if err != nil {
			return fmt.Errorf("failed to add packet loss (Gilbert-Elliot model) for one or more containers: %w", err)
		}
	}

	return nil
}
