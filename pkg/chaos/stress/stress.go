package stress

import (
	"context"
	"strings"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// StressCommand `stress-ng` command
type StressCommand struct {
	client    container.Client
	names     []string
	pattern   string
	labels    []string
	image     string
	pull      bool
	stressors []string
	duration  time.Duration
	limit     int
	dryRun    bool
}

type stressedContainer struct {
	stress    string              // stress container ID
	container container.Container // target container
}

const (
	defaultStopTimeout = 5 * time.Second
)

// NewStressCommand create new Kill Command instance
func NewStressCommand(client container.Client, names []string, pattern string, labels []string, image string, pull bool, stressors, interval, duration string, limit int, dryRun bool) (chaos.Command, error) {
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
	stress := &StressCommand{client, names, pattern, labels, image, pull, strings.Fields(stressors), d, limit, dryRun}
	return stress, nil
}

// Run stress command
func (s *StressCommand) Run(ctx context.Context, random bool) error {
	log.Debug("stress testing all matching containers")
	log.WithFields(log.Fields{
		"names":     s.names,
		"pattern":   s.pattern,
		"labels":    s.labels,
		"duration":  s.duration,
		"stressors": s.stressors,
		"limit":     s.limit,
		"random":    random,
	}).Debug("listing matching containers")
	containers, err := container.ListNContainers(ctx, s.client, s.names, s.pattern, s.labels, s.limit)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		log.Warning("no containers to stress test")
		return nil
	}

	// select single random container from matching container and replace list with selected item
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []container.Container{*c}
		}
	}

	// keep stressed containers
	stressedContainers := []stressedContainer{}
	// stress containers
	for _, container := range containers {
		log.WithFields(log.Fields{
			"container":       container.ID(),
			"duration":        s.duration,
			"stressors":       s.stressors,
			"stress-ng image": s.image,
			"pull image":      s.pull,
		}).Debug("stress testing container for duration")
		stress, err := s.client.StressContainer(ctx, container, s.stressors, s.image, s.pull, s.duration, s.dryRun)
		if err != nil {
			log.WithError(err).Warn("failed to stress container")
			break
		}
		stressedContainers = append(stressedContainers, stressedContainer{stress, container})
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
	return err
}

// stop stress-ng containers, one by one
func (s *StressCommand) stopStressContainers(ctx context.Context, containers []stressedContainer) error {
	var err error
	for _, exec := range containers {
		log.WithField("container", exec.container.ID).Debug("stop stress for container")
		if e := s.client.StopContainerWithID(ctx, exec.stress, defaultStopTimeout, s.dryRun); e != nil {
			log.WithError(e).Warn("failed to stop stress-ng container")
			err = errors.Wrap(e, "failed to stop stress-ng container")
		}
	}
	return err // last non nil error
}
