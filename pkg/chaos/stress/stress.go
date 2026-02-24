package stress

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// stressClient is the narrow interface needed by the stress command.
type stressClient interface {
	container.Lister
	container.Stressor
	StopContainerWithID(context.Context, string, time.Duration, bool) error
}

// `stress-ng` command
type stressCommand struct {
	client       stressClient
	names        []string
	pattern      string
	labels       []string
	image        string
	pull         bool
	stressors    []string
	duration     time.Duration
	limit        int
	injectCgroup bool
	dryRun       bool
}

const (
	defaultStopTimeout = 5 * time.Second
)

// NewStressCommand create new Kill stressCommand instance
func NewStressCommand(client stressClient, globalParams *chaos.GlobalParams, image string, pull bool, stressors string, duration time.Duration, limit int, injectCgroup bool) chaos.Command {
	stress := &stressCommand{
		client:       client,
		names:        globalParams.Names,
		pattern:      globalParams.Pattern,
		labels:       globalParams.Labels,
		image:        image,
		pull:         pull,
		stressors:    strings.Fields(stressors),
		duration:     duration,
		limit:        limit,
		injectCgroup: injectCgroup,
		dryRun:       globalParams.DryRun,
	}
	return stress
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
		return fmt.Errorf("error listing containers: %w", err)
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
		return fmt.Errorf("one or more stress test failed: %w", err)
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
	stress, output, outerr, err := s.client.StressContainer(ctx, c, s.stressors, s.image, s.pull, s.duration, s.injectCgroup, s.dryRun)
	if err != nil {
		return fmt.Errorf("stress test failed: %w", err)
	}
	if s.dryRun {
		return nil
	}
	timer := time.NewTimer(s.duration)
	defer timer.Stop()
	select {
	case out := <-output:
		log.WithField("stdout", out).Debug("stress-ng completed")
	case e := <-outerr:
		return fmt.Errorf("stress-ng failed with error: %w", e)
	case <-ctx.Done():
		log.Debug("stop stress test on containers by stop event")
		// NOTE: use different context to stop netem since parent context is canceled
		err = s.client.StopContainerWithID(context.Background(), stress, defaultStopTimeout, s.dryRun)
		if err != nil {
			return fmt.Errorf("failed to stop stress-ng container: %w", err)
		}
	case <-timer.C:
		log.WithField("duration", s.duration).Debug("stop stress containers after duration")
		// NOTE: use background context since parent context may be canceled when timer and context fire simultaneously
		err = s.client.StopContainerWithID(context.Background(), stress, defaultStopTimeout, s.dryRun)
		if err != nil {
			return fmt.Errorf("failed to stop stress-ng container: %w", err)
		}
	}
	return nil
}
