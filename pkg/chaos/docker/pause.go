package docker

import (
	"context"
	"errors"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// PauseCommand `docker pause` command
type PauseCommand struct {
	client   container.Client
	names    []string
	pattern  string
	duration time.Duration
	limit    int
	dryRun   bool
}

// NewPauseCommand create new Pause Command instance
func NewPauseCommand(client container.Client, names []string, pattern string, intervalStr string, durationStr string, limit int, dryRun bool) (chaos.Command, error) {
	// get interval
	interval, err := container.GetIntervalValue(intervalStr)
	if err != nil {
		return nil, err
	}
	// get duration
	var duration time.Duration
	if durationStr == "" {
		return nil, errors.New("undefined duration")
	}
	if durationStr != "" {
		duration, err = time.ParseDuration(durationStr)
		if err != nil {
			return nil, err
		}
	}
	if interval != 0 && duration >= interval {
		return nil, errors.New("duration must be shorter than interval")
	}
	return &PauseCommand{client, names, pattern, duration, limit, dryRun}, nil
}

// Run pause command
func (p *PauseCommand) Run(ctx context.Context, random bool) error {
	log.Debug("pausing all matching containers")
	log.WithFields(log.Fields{
		"names":    p.names,
		"pattern":  p.pattern,
		"duration": p.duration,
		"limit":    p.limit,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, p.client, p.names, p.pattern, p.limit)
	if err != nil {
		log.WithError(err).Error("failed to list containers")
		return err
	}
	if len(containers) == 0 {
		log.Warning("no containers to stop")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		log.Debug("selecting single random container")
		if c := container.RandomContainer(containers); c != nil {
			containers = []container.Container{*c}
		}
	}

	// keep paused containers
	pausedContainers := []container.Container{}
	// pause containers
	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"duration":  p.duration,
		}).Debug("pausing container for duration")
		err = p.client.PauseContainer(ctx, container, p.dryRun)
		if err != nil {
			log.WithError(err).Error("failed to pause container")
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
	if err != nil {
		log.WithError(err).Error("failed to unpause paused containers")
	}
	return err
}

// unpause containers
func (p *PauseCommand) unpauseContainers(ctx context.Context, containers []container.Container) error {
	var err error
	for _, container := range containers {
		log.WithField("container", container).Debug("unpause container")
		if e := p.client.UnpauseContainer(ctx, container, p.dryRun); e != nil {
			log.WithError(e).Error("failed to unpause container")
			err = e
		}
	}
	return err // last non nil error
}
