package stress

import (
	"context"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// StressCommand `stress-ng` command
type StressCommand struct {
	client  container.Client
	names   []string
	pattern string
	labels  []string
	args    string
	limit   int
	dryRun  bool
}

// NewStressCommand create new Kill Command instance
func NewStressCommand(client container.Client, names []string, pattern string, labels []string, args string, limit int, dryRun bool) (chaos.Command, error) {
	stress := &StressCommand{client, names, pattern, labels, args, limit, dryRun}
	return stress, nil
}

// Run stress command
func (s *StressCommand) Run(ctx context.Context, random bool) error {
	log.Debug("stress testing all matching containers")
	log.WithFields(log.Fields{
		"names":   s.names,
		"pattern": s.pattern,
		"labels":  s.labels,
		"limit":   s.limit,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, s.client, s.names, s.pattern, s.labels, s.limit)
	if err != nil {
		log.WithError(err).Error("failed to list containers")
		return err
	}
	if len(containers) == 0 {
		log.Warning("no containers to stress test")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		log.Debug("selecting single random container")
		if c := container.RandomContainer(containers); c != nil {
			containers = []container.Container{*c}
		}
	}

	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
		}).Debug("stress testing container")
		err := s.client.StressContainer(ctx, container, []string{"--cpu", "4", "--timeout", "60s"}, "alexeiled/stress-ng", true, 200*time.Second, s.dryRun)
		if err != nil {
			log.WithError(err).Error("failed to kill container")
			return err
		}
	}
	return nil
}
