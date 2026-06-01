package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// restartClient is the narrow interface needed by the restart command.
type restartClient interface {
	container.Lister
	RestartContainer(context.Context, *container.Container, time.Duration, bool) error
}

// `docker restart` command
type restartCommand struct {
	client  restartClient
	names   []string
	patterns []string
	labels  []string
	timeout time.Duration
	limit   int
	dryRun  bool
}

// NewRestartCommand create new Restart Command instance
func NewRestartCommand(client restartClient, params *chaos.GlobalParams, timeout time.Duration, limit int) chaos.Command {
	return &restartCommand{
		client:  client,
		names:   params.Names,
		patterns: params.Patterns,
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
		"patterns": k.patterns,
		"labels":  k.labels,
		"limit":   k.limit,
		"random":  random,
	}).Debug("listing matching containers")
	gp := &chaos.GlobalParams{Names: k.names, Patterns: k.patterns, Labels: k.labels}
	return chaos.RunOnContainers(ctx, k.client, gp, k.limit, random, false,
		func(ctx context.Context, c *container.Container) error {
			log.WithFields(log.Fields{"container": c, "timeout": k.timeout}).Debug("restarting container")
			if err := k.client.RestartContainer(ctx, c, k.timeout, k.dryRun); err != nil {
				return fmt.Errorf("failed to restart container: %w", err)
			}
			return nil
		})
}
