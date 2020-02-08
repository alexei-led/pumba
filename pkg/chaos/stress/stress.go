package stress

import (
	"context"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"
	log "github.com/sirupsen/logrus"
)

// StressCommand `stress-ng` command
type StressCommand struct {
	client   container.Client
	names    []string
	pattern  string
	labels   []string
	args     string
	duration time.Duration
	limit    int
	dryRun   bool
}

type execContainer struct {
	ID        string // exec ID
	container container.Container
}

// NewStressCommand create new Kill Command instance
func NewStressCommand(client container.Client, names []string, pattern string, labels []string, args, interval, duration string, limit int, dryRun bool) (chaos.Command, error) {
	// get interval
	i, err := util.GetIntervalValue(interval)
	if err != nil {
		return nil, err
	}
	// get duration
	d, err := util.GetDurationValue(duration, i)
	if err != nil {
		return nil, err
	}
	stress := &StressCommand{client, names, pattern, labels, args, d, limit, dryRun}
	return stress, nil
}

// Run stress command
func (s *StressCommand) Run(ctx context.Context, random bool) error {
	log.Debug("stress testing all matching containers")
	log.WithFields(log.Fields{
		"names":    s.names,
		"pattern":  s.pattern,
		"labels":   s.labels,
		"duration": s.duration,
		"limit":    s.limit,
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

	// keep stressed containers
	stressedContainers := []execContainer{}
	// pause containers
	for _, container := range containers {
		log.WithFields(log.Fields{
			"container": container,
			"duration":  s.duration,
		}).Debug("stress testing container for duration")
		execID, err := s.client.StressContainer(ctx, container, []string{"--cpu", "4", "--timeout", "60s"}, "alexeiled/stress-ng", true, s.duration, s.dryRun)
		if err != nil {
			log.WithError(err).Error("failed to stress container")
			break
		}
		stressedContainers = append(stressedContainers, execContainer{execID, container})
	}

	// if there are stressed containers stop stress-ng and remove it from containers
	if len(stressedContainers) > 0 {
		// wait for specified duration and then stop stress-ng containers or stop stress-ng on ctx.Done()
		select {
		case <-ctx.Done():
			log.Debug("stop stress test on containers by stop event")
			// NOTE: use different context to stop netem since parent context is canceled
			err = s.stopStressContainers(context.Background(), stressedContainers)
		case <-time.After(s.duration):
			log.WithField("duration", s.duration).Debug("stop stress containers after duration")
			err = s.stopStressContainers(ctx, stressedContainers)
		}
	}
	if err != nil {
		log.WithError(err).Error("failed to unpause paused containers")
	}
	return err
}

// stop stress test on containers
func (s *StressCommand) stopStressContainers(ctx context.Context, containers []execContainer) error {
	var err error
	for _, exec := range containers {
		log.WithField("container", exec.container.ID).Debug("stop stress on container")
		if e := s.client.StopStressContainer(ctx, exec.container, exec.ID, s.dryRun); e != nil {
			log.WithError(e).Error("failed to stop stress on container")
			err = e
		}
	}
	return err // last non nil error
}
