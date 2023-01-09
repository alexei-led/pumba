package docker

import (
	"context"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	// DeafultWaitTime time to wait before stopping container (in seconds)
	DeafultWaitTime = 5
)

// `docker stop` command
type stopCommand struct {
	client   container.Client
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
func NewStopCommand(client container.Client, params *chaos.GlobalParams, restart bool, duration time.Duration, waitTime, limit int) chaos.Command {
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
	log.Debug("stopping all matching containers")
	log.WithFields(log.Fields{
		"names":    s.names,
		"pattern":  s.pattern,
		"labels":   s.labels,
		"duration": s.duration,
		"waitTime": s.waitTime,
		"limit":    s.limit,
		"random":   random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, s.client, s.names, s.pattern, s.labels, s.limit)
	if err != nil {
		return errors.Wrap(err, "error listing containers")
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

	// keep stopped containers
	stoppedContainers := make([]*container.Container, 0, len(containers))
	// pause containers
	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"waitTime":  s.waitTime,
		}).Debug("stopping container")
		c := container
		err = s.client.StopContainer(ctx, c, s.waitTime, s.dryRun)
		if err != nil {
			log.WithError(err).Warn("failed to stop container")
			break
		}
		stoppedContainers = append(stoppedContainers, container)
	}

	// if there are stopped containers and want to (re)start ...
	if len(stoppedContainers) > 0 && s.restart {
		// wait for specified duration and then unpause containers or unpause on ctx.Done()
		select {
		case <-ctx.Done():
			log.Debug("start stopped containers by stop event")
			// NOTE: use different context to stop netem since parent context is canceled
			err = s.startStoppedContainers(context.Background(), stoppedContainers)
		case <-time.After(s.duration):
			log.WithField("duration", s.duration).Debug("start stopped containers after duration")
			err = s.startStoppedContainers(ctx, stoppedContainers)
		}
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
			err = errors.Wrap(e, "failed to start stopped container")
		}
	}
	return err // last non nil error
}
