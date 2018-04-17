package docker

import (
	"context"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

const (
	// DeafultWaitTime time to wait before stopping container (in seconds)
	DeafultWaitTime = 10
)

// StopCommand `docker kill` command
type StopCommand struct {
	client   container.Client
	names    []string
	pattern  string
	waitTime int
	limit    int
	dryRun   bool
}

// NewStopCommand create new Stop Command instance
func NewStopCommand(client container.Client, names []string, pattern string, waitTime int, limit int, dryRun bool) ChaosCommand {
	if waitTime <= 0 {
		waitTime = DeafultWaitTime
	}
	return &StopCommand{client, names, pattern, waitTime, limit, dryRun}
}

// Run stop command
func (s *StopCommand) Run(ctx context.Context, random bool) error {
	log.Debug("stopping all matching containers")
	log.WithFields(log.Fields{
		"names":   s.names,
		"pattern": s.pattern,
		"limit":   s.limit,
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

	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"waitTime":  s.waitTime,
		}).Debug("stopping container")
		err := s.client.StopContainer(ctx, container, s.waitTime, s.dryRun)
		if err != nil {
			log.WithError(err).Error("failed to stop container")
			return err
		}
	}

	return nil
}
