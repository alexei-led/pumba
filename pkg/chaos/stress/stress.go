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
	"golang.org/x/sync/errgroup"
)

// `stress-ng` command
type stressCommand struct {
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

const (
	defaultStopTimeout = 5 * time.Second
)

// NewStressCommand create new Kill stressCommand instance
func NewStressCommand(client container.Client, globalParams *chaos.GlobalParams, image string, pull bool, stressors, duration string, limit int) (chaos.Command, error) {
	// get duration
	d, err := util.GetDurationValue(duration, globalParams.Interval)
	if err != nil {
		return nil, err
	}
	stress := &stressCommand{
		client:    client,
		names:     globalParams.Names,
		pattern:   globalParams.Pattern,
		labels:    globalParams.Labels,
		image:     image,
		pull:      pull,
		stressors: strings.Fields(stressors),
		duration:  d,
		limit:     limit,
		dryRun:    globalParams.DryRun,
	}
	return stress, nil
}

// Run stress command
func (s *stressCommand) Run(ctx context.Context, random bool) error {
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
			containers = []*container.Container{c}
		}
	}

	// stress containers
	var eg errgroup.Group
	for _, c := range containers {
		cntr := c
		eg.Go(func() error {
			return s.stressContainer(ctx, cntr)
		})
	}
	// wait till all stress tests complete
	if err := eg.Wait(); err != nil {
		return errors.Wrap(err, "one or more stress test failed")
	}
	return nil
}

func (s *stressCommand) stressContainer(ctx context.Context, c *container.Container) error {
	log.WithFields(log.Fields{
		"container":       c.ID(),
		"duration":        s.duration,
		"stressors":       s.stressors,
		"stress-ng image": s.image,
		"pull image":      s.pull,
	}).Debug("stress testing container for duration")
	stress, output, outerr, err := s.client.StressContainer(ctx, c, s.stressors, s.image, s.pull, s.duration, s.dryRun)
	if err != nil {
		return err
	}
	select {
	case out := <-output:
		log.WithField("stdout", out).Debug("stress-ng completed")
		break
	case e := <-outerr:
		return errors.Wrap(e, "stress-ng failed with error")
	case <-ctx.Done():
		log.Debug("stop stress test on containers by stop event")
		// NOTE: use different context to stop netem since parent context is canceled
		err = s.client.StopContainerWithID(context.Background(), stress, defaultStopTimeout, s.dryRun)
		if err != nil {
			return errors.Wrap(err, "failed to stop stress-ng container")
		}
		break
	case <-time.After(s.duration):
		log.WithField("duration", s.duration).Debug("stop stress containers after duration")
		err = s.client.StopContainerWithID(ctx, stress, defaultStopTimeout, s.dryRun)
		if err != nil {
			return errors.Wrap(err, "failed to stop stress-ng container")
		}
		break
	}
	return nil
}
