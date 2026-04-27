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
	names    []string
	pattern  string
	labels   []string
	duration time.Duration
	limit    int
	dryRun   bool
}

// NewPauseCommand create new Pause Command instance
func NewPauseCommand(client pauseClient, params *chaos.GlobalParams, duration time.Duration, limit int) chaos.Command {
	return &pauseCommand{
		client:   client,
		names:    params.Names,
		pattern:  params.Pattern,
		labels:   params.Labels,
		duration: duration,
		limit:    limit,
		dryRun:   params.DryRun}
}

// Run pause command
func (p *pauseCommand) Run(ctx context.Context, random bool) error {
	log.Debug("pausing all matching containers")
	log.WithFields(log.Fields{
		"names":    p.names,
		"pattern":  p.pattern,
		"labels":   p.labels,
		"duration": p.duration,
		"limit":    p.limit,
		"random":   random,
	}).Debug("listing matching containers")
	gp := &chaos.GlobalParams{Names: p.names, Pattern: p.pattern, Labels: p.labels}
	pausedContainers := make([]*container.Container, 0)
	err := chaos.RunOnContainers(ctx, p.client, gp, p.limit, random, false,
		func(ctx context.Context, c *container.Container) error {
			log.WithFields(log.Fields{"container": c, "duration": p.duration}).Debug("pausing container for duration")
			if pErr := p.client.PauseContainer(ctx, c, p.dryRun); pErr != nil {
				log.WithError(pErr).Warn("failed to pause container")
				return pErr
			}
			pausedContainers = append(pausedContainers, c)
			return nil
		})

	// if there are paused containers unpause them
	if len(pausedContainers) > 0 {
		// wait for specified duration and then unpause containers or unpause on ctx.Done()
		durationTimer := time.NewTimer(p.duration)
		defer durationTimer.Stop()
		var unpauseErr error
		select {
		case <-ctx.Done():
			log.Debug("unpause containers by stop event")
			// use context.WithoutCancel so cleanup succeeds even if the parent ctx is canceled
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.duration)
			defer cancel()
			unpauseErr = p.unpauseContainers(cleanupCtx, pausedContainers)
		case <-durationTimer.C:
			log.WithField("duration", p.duration).Debug("unpause containers after duration")
			unpauseErr = p.unpauseContainers(ctx, pausedContainers)
		}
		err = errors.Join(err, unpauseErr)
	}
	return err
}

// unpause containers
func (p *pauseCommand) unpauseContainers(ctx context.Context, containers []*container.Container) error {
	var err error
	for _, container := range containers {
		log.WithField("container", container).Debug("unpause container")
		c := container
		if e := p.client.UnpauseContainer(ctx, c, p.dryRun); e != nil {
			err = fmt.Errorf("failed to unpause container: %w", e)
		}
	}
	return err // last non nil error
}
