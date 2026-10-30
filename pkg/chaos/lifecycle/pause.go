package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// pauseClient is the narrow interface needed by the pause command.
type pauseClient interface {
	container.Lister
	PauseContainer(context.Context, *container.Container, bool) error
	UnpauseContainer(context.Context, *container.Container, bool) error
}

// `docker pause` command
type pauseCommand struct {
	client   pauseClient
	gp       *chaos.GlobalParams
	duration time.Duration
	limit    int
}

// NewPauseCommand create new Pause Command instance
func NewPauseCommand(client pauseClient, params *chaos.GlobalParams, duration time.Duration, limit int) chaos.Command {
	return &pauseCommand{
		client:   client,
		gp:       params,
		duration: duration,
		limit:    limit,
	}
}

// Run pause command
func (p *pauseCommand) Run(ctx context.Context, random bool) error {
	log.Debug("pausing all matching containers")
	log.WithFields(log.Fields{
		"names":    p.gp.Names,
		"pattern":  p.gp.Pattern,
		"labels":   p.gp.Labels,
		"duration": p.duration,
		"limit":    p.limit,
		"random":   random,
	}).Debug("listing matching containers")
	var paused []heldTarget
	err := chaos.RunOnContainers(ctx, p.client, p.gp, p.limit, random, false,
		func(ctx context.Context, c *container.Container, t *chaos.Target) error {
			log.WithFields(log.Fields{"container": c, "duration": p.duration}).Debug("pausing container for duration")
			if pErr := p.client.PauseContainer(ctx, c, p.gp.DryRun); pErr != nil {
				log.WithError(pErr).Warn("failed to pause container")
				return revertInterrupted(ctx, c, t, pErr, p.unpauseContainer)
			}
			t.Applied()
			paused = append(paused, holdTarget(c, t))
			return nil
		})
	// unpause paused containers after duration or on ctx.Done()
	return errors.Join(err, revertAfter(ctx, p.duration, p.gp.DryRun, paused, p.unpauseContainer))
}

func (p *pauseCommand) unpauseContainer(ctx context.Context, c *container.Container) error {
	log.WithField("container", c).Debug("unpause container")
	if err := p.client.UnpauseContainer(ctx, c, p.gp.DryRun); err != nil {
		return fmt.Errorf("failed to unpause container: %w", err)
	}
	return nil
}
