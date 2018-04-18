package docker

import (
	"context"
	"errors"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

const (
	// DeafultWaitTime time to wait before stopping container (in seconds)
	DeafultWaitTime = 10
)

// StopCommand `docker stop` command
type StopCommand struct {
	client   container.Client
	names    []string
	pattern  string
	restart  bool
	duration time.Duration
	waitTime int
	limit    int
	dryRun   bool
}

// NewStopCommand create new Stop Command instance
func NewStopCommand(client container.Client, names []string, pattern string, restart bool, intervalStr string, durationStr string, waitTime int, limit int, dryRun bool) (chaos.Command, error) {
	if waitTime <= 0 {
		waitTime = DeafultWaitTime
	}
	// get interval
	interval, err := getIntervalValue(intervalStr)
	if err != nil {
		return nil, err
	}
	// get duration
	var duration time.Duration
	if durationStr == "" && restart {
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
	return &StopCommand{client, names, pattern, restart, duration, waitTime, limit, dryRun}, nil
}

// Run stop command
func (s *StopCommand) Run(ctx context.Context, random bool) error {
	log.Debug("stopping all matching containers")
	log.WithFields(log.Fields{
		"names":    s.names,
		"pattern":  s.pattern,
		"duration": s.duration,
		"waitTime": s.waitTime,
		"limit":    s.limit,
	}).Debug("listing matching containers")
	containers, err := listNContainers(ctx, s.client, s.names, s.pattern, s.limit)
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
		if c := randomContainer(containers); c != nil {
			containers = []container.Container{*c}
		}
	}

	// keep stopped containers
	stoppedContainers := []container.Container{}
	// pause containers
	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"waitTime":  s.waitTime,
		}).Debug("stopping container")
		err = s.client.StopContainer(ctx, container, s.waitTime, s.dryRun)
		if err != nil {
			log.WithError(err).Error("failed to stop container")
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
	if err != nil {
		log.WithError(err).Error("failed to start stopped containers")
	}
	return err
}

// start previously stopped containers after duration on exit
func (s *StopCommand) startStoppedContainers(ctx context.Context, containers []container.Container) error {
	var err error
	for _, container := range containers {
		log.WithField("container", container).Debug("start stopped container")
		if e := s.client.StartContainer(ctx, container, s.dryRun); e != nil {
			log.WithError(e).Error("failed to start stopped container")
			err = e
		}
	}
	return err // last non nil error
}
