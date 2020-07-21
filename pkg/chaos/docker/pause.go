package docker

import (
	"context"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// PauseCommand `docker pause` command
type PauseCommand struct {
	client   container.Client
	names    []string
	pattern  string
	labels   []string
	duration time.Duration
	limit    int
	dryRun   bool
}

// NewPauseCommand create new Pause Command instance
func NewPauseCommand(client container.Client, names []string, pattern string, labels []string, intervalStr, durationStr string, limit int, dryRun bool) (chaos.Command, error) {
	// get interval
	interval, err := util.GetIntervalValue(intervalStr)
	if err != nil {
		return nil, err
	}
	// get duration
	duration, err := util.GetDurationValue(durationStr, interval)
	if err != nil {
		return nil, err
	}
	return &PauseCommand{client, names, pattern, labels, duration, limit, dryRun}, nil
}

// Run pause command
func (p *PauseCommand) Run(ctx context.Context, random bool) error {
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
		return err
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
	pausedContainers := []*container.Container{}
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
func (p *PauseCommand) unpauseContainers(ctx context.Context, containers []*container.Container) error {
	var err error
	for _, container := range containers {
		log.WithField("container", container).Debug("unpause container")
		c := container
		if e := p.client.UnpauseContainer(ctx, c, p.dryRun); e != nil {
			err = errors.Wrap(e, "failed to unpause container")
		}
	}
	return err // last non nil error
}
