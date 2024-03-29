package docker

import (
	"context"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// `docker restart` command
type restartCommand struct {
	client  container.Client
	names   []string
	pattern string
	labels  []string
	timeout time.Duration
	limit   int
	dryRun  bool
}

// NewRestartCommand create new Restart Command instance
func NewRestartCommand(client container.Client, params *chaos.GlobalParams, timeout time.Duration, limit int) chaos.Command {
	return &restartCommand{
		client:  client,
		names:   params.Names,
		pattern: params.Pattern,
		labels:  params.Labels,
		timeout: timeout,
		limit:   limit,
		dryRun:  params.DryRun,
	}
}

// Run restart command
func (k *restartCommand) Run(ctx context.Context, random bool) error {
	log.Debug("restarting all matching containers")
	log.WithFields(log.Fields{
		"names":   k.names,
		"pattern": k.pattern,
		"labels":  k.labels,
		"limit":   k.limit,
		"random":  random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, k.client, k.names, k.pattern, k.labels, k.limit)
	if err != nil {
		return errors.Wrap(err, "error listing containers")
	}
	if len(containers) == 0 {
		log.Warning("no containers to restart")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []*container.Container{c}
		}
	}

	for _, c := range containers {
		log.WithFields(log.Fields{
			"container": c,
			"timeout":   k.timeout,
		}).Debug("restarting container")
		err = k.client.RestartContainer(ctx, c, k.timeout, k.dryRun)
		if err != nil {
			return errors.Wrap(err, "failed to restart container")
		}
	}
	return nil
}
