package netem

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

const (
	duplicateCmd = "duplicate"
)

// `netem duplicate` command
type duplicateCommand struct {
	client      container.Client
	names       []string
	pattern     string
	labels      []string
	iface       string
	ips         []*net.IPNet
	sports      []string
	dports      []string
	duration    time.Duration
	percent     float64
	correlation float64
	image       string
	pull        bool
	limit       int
	dryRun      bool
}

// NewDuplicateCommand create new netem duplicate command
//
//nolint:dupl
func NewDuplicateCommand(client container.Client,
	globalParams *chaos.GlobalParams,
	netemParams *Params,
	percent, // duplicate percent
	correlation float64, // duplicate correlation
) (chaos.Command, error) {
	// get netem duplicate percent
	if percent < 0.0 || percent > 100.0 {
		return nil, errors.New("invalid duplicate percent: must be between 0.0 and 100.0")
	}
	// get netem duplicate variation
	if correlation < 0.0 || correlation > 100.0 {
		return nil, errors.New("invalid duplicate correlation: must be between 0.0 and 100.0")
	}
	return &duplicateCommand{
		client:      client,
		names:       globalParams.Names,
		pattern:     globalParams.Pattern,
		labels:      globalParams.Labels,
		iface:       netemParams.Iface,
		ips:         netemParams.Ips,
		sports:      netemParams.Sports,
		dports:      netemParams.Dports,
		duration:    netemParams.Duration,
		percent:     percent,
		correlation: correlation,
		image:       netemParams.Image,
		limit:       netemParams.Limit,
		pull:        netemParams.Pull,
		dryRun:      globalParams.DryRun,
	}, nil
}

// Run netem duplicate command
func (n *duplicateCommand) Run(ctx context.Context, random bool) error {
	log.Debug("adding network random packet duplicates to all matching containers")
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

	// prepare netem duplicate command
	netemCmd := []string{duplicateCmd, strconv.FormatFloat(n.percent, 'f', 2, 64)}
	if n.correlation > 0 {
		netemCmd = append(netemCmd, strconv.FormatFloat(n.correlation, 'f', 2, 64))
	}

	// run netem duplicate command for selected containers
	var wg sync.WaitGroup
	errs := make([]error, len(containers))
	cancels := make([]context.CancelFunc, len(containers))

	//nolint:dupl
	for i, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
		}).Debug("adding network random packet duplicates for container")
		netemCtx, cancel := context.WithTimeout(ctx, n.duration)
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, c *container.Container) {
			defer wg.Done()
			errs[i] = runNetem(netemCtx, n.client, c, n.iface, netemCmd, n.ips, n.sports, n.dports, n.duration, n.image, n.pull, n.dryRun)
			if errs[i] != nil {
				log.WithError(errs[i]).Warn("failed to set packet duplicates for container")
			}
		}(i, c)
	}

	// Wait for all netem commands to complete
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
			return fmt.Errorf("failed to set packet duplicates for one or more containers: %w", err)
		}
	}

	return nil
}
