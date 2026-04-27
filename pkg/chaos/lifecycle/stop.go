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

const (
	// DeafultWaitTime time to wait before stopping container (in seconds)
	DeafultWaitTime = 5
)

// stopClient is the narrow interface needed by the stop command.
type stopClient interface {
	container.Lister
	StopContainer(context.Context, *container.Container, int, bool) error
	StartContainer(context.Context, *container.Container, bool) error
}

// `docker stop` command
type stopCommand struct {
	client   stopClient
	names    []string
	pattern  string
	labels   []string
	restart  bool
	duration time.Duration
	waitTime int
	limit    int
	dryRun   bool
}

// NewStopCommand create new Stop Command instance
func NewStopCommand(client stopClient, params *chaos.GlobalParams, restart bool, duration time.Duration, waitTime, limit int) chaos.Command {
	if waitTime <= 0 {
		waitTime = DeafultWaitTime
	}
	return &stopCommand{
		client:   client,
		names:    params.Names,
		pattern:  params.Pattern,
		labels:   params.Labels,
		dryRun:   params.DryRun,
		restart:  restart,
		duration: duration,
		waitTime: waitTime,
		limit:    limit}
}

// Run stop command
func (s *stopCommand) Run(ctx context.Context, random bool) error {
	log.WithFields(log.Fields{
		"names":    s.names,
		"pattern":  s.pattern,
		"labels":   s.labels,
		"duration": s.duration,
		"waitTime": s.waitTime,
		"limit":    s.limit,
		"random":   random,
	}).Debug("stopping all matching containers")
	gp := &chaos.GlobalParams{Names: s.names, Pattern: s.pattern, Labels: s.labels}
	stoppedContainers := make([]*container.Container, 0)
	err := chaos.RunOnContainers(ctx, s.client, gp, s.limit, random, false,
		func(ctx context.Context, c *container.Container) error {
			log.WithFields(log.Fields{"container": c, "waitTime": s.waitTime}).Debug("stopping container")
			if sErr := s.client.StopContainer(ctx, c, s.waitTime, s.dryRun); sErr != nil {
				log.WithError(sErr).Warn("failed to stop container")
				return sErr
			}
			stoppedContainers = append(stoppedContainers, c)
			return nil
		})

	// if there are stopped containers and want to (re)start ...
	if len(stoppedContainers) > 0 && s.restart {
		// wait for specified duration and then start containers or start on ctx.Done()
		durationTimer := time.NewTimer(s.duration)
		defer durationTimer.Stop()
		var restartErr error
		select {
		case <-ctx.Done():
			log.Debug("start stopped containers by stop event")
			// use context.WithoutCancel so cleanup succeeds even if the parent ctx is canceled
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.duration)
			defer cancel()
			restartErr = s.startStoppedContainers(cleanupCtx, stoppedContainers)
		case <-durationTimer.C:
			log.WithField("duration", s.duration).Debug("start stopped containers after duration")
			restartErr = s.startStoppedContainers(ctx, stoppedContainers)
		}
		err = errors.Join(err, restartErr)
	}
	return err
}

// start previously stopped containers after duration on exit
func (s *stopCommand) startStoppedContainers(ctx context.Context, containers []*container.Container) error {
	var err error
	for _, container := range containers {
		c := container
		log.WithField("container", c).Debug("start stopped container")
		if e := s.client.StartContainer(ctx, c, s.dryRun); e != nil {
			err = errors.Join(err, fmt.Errorf("failed to start stopped container: %w", e))
		}
	}
	return err
}
