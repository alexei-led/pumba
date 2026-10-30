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
	gp      *chaos.GlobalParams
	timeout time.Duration
	limit   int
}

// NewRestartCommand create new Restart Command instance
func NewRestartCommand(client restartClient, params *chaos.GlobalParams, timeout time.Duration, limit int) chaos.Command {
	return &restartCommand{
		client:  client,
		gp:      params,
		timeout: timeout,
		limit:   limit,
	}
}

// Run restart command
func (k *restartCommand) Run(ctx context.Context, random bool) error {
	log.Debug("restarting all matching containers")
	log.WithFields(log.Fields{
		"names":   k.gp.Names,
		"pattern": k.gp.Pattern,
		"labels":  k.gp.Labels,
		"limit":   k.limit,
		"random":  random,
	}).Debug("listing matching containers")
	return chaos.RunOnContainers(ctx, k.client, k.gp, k.limit, random, false,
		func(ctx context.Context, c *container.Container, t *chaos.Target) error {
			log.WithFields(log.Fields{"container": c, "timeout": k.timeout}).Debug("restarting container")
			if err := k.client.RestartContainer(ctx, c, k.timeout, k.gp.DryRun); err != nil {
				return fmt.Errorf("failed to restart container: %w", err)
			}
			t.Applied()
			return nil
		})
}
