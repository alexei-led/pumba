package docker

import (
	"context"
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
	containers, err := container.ListNContainers(ctx, p.client, p.names, p.pattern, p.labels, p.limit)
	if err != nil {
		return fmt.Errorf("error listing containers: %w", err)
	}
	if len(containers) == 0 {
		log.Warning("no containers to stop")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []*container.Container{c}
		}
	}

	// keep paused containers
	pausedContainers := make([]*container.Container, 0, len(containers))
	// pause containers
	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"duration":  p.duration,
		}).Debug("pausing container for duration")
		c := container
		err = p.client.PauseContainer(ctx, c, p.dryRun)
		if err != nil {
			log.WithError(err).Warn("failed to pause container")
			break
		}
		pausedContainers = append(pausedContainers, container)
	}

	// if there are paused containers unpause them
	if len(pausedContainers) > 0 {
		// wait for specified duration and then unpause containers or unpause on ctx.Done()
		select {
		case <-ctx.Done():
			log.Debug("unpause containers by stop event")
			// NOTE: use different context to stop netem since parent context is canceled
			err = p.unpauseContainers(context.Background(), pausedContainers)
		case <-time.After(p.duration):
			log.WithField("duration", p.duration).Debug("unpause containers after duration")
			err = p.unpauseContainers(ctx, pausedContainers)
		}
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
