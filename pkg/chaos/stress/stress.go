package stress

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
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
	gp           *chaos.GlobalParams
	image        string
	pull         bool
	stressors    []string
	duration     time.Duration
	limit        int
	injectCgroup bool
}

const (
	defaultStopTimeout = 5 * time.Second
)

// NewStressCommand create new Kill stressCommand instance
func NewStressCommand(client stressClient, globalParams *chaos.GlobalParams, image string, pull bool, stressors string, duration time.Duration, limit int, injectCgroup bool) chaos.Command {
	stress := &stressCommand{
		client:       client,
		gp:           globalParams,
		image:        image,
		pull:         pull,
		stressors:    strings.Fields(stressors),
		duration:     duration,
		limit:        limit,
		injectCgroup: injectCgroup,
	}
	return stress
}

// Run stress command
func (s *stressCommand) Run(ctx context.Context, random bool) error {
	log.WithFields(log.Fields{
		"names":     s.gp.Names,
		"pattern":   s.gp.Pattern,
		"labels":    s.gp.Labels,
		"duration":  s.duration,
		"stressors": s.stressors,
		"limit":     s.limit,
		"random":    random,
	}).Debug("stress testing all matching containers")
	if err := chaos.RunOnContainers(ctx, s.client, s.gp, s.limit, random, true, s.stressContainer); err != nil {
		return fmt.Errorf("one or more stress test failed: %w", err)
	}
	return nil
}

func (s *stressCommand) stressContainer(ctx context.Context, c *container.Container, t *chaos.Target) error {
	log.WithFields(log.Fields{
		"container":       c.ID(),
		"duration":        s.duration,
		"stressors":       s.stressors,
		"stress-ng image": s.image,
		"pull image":      s.pull,
	}).Debug("stress testing container for duration")
	req := &container.StressRequest{
		Container:    c,
		Stressors:    s.stressors,
		Duration:     s.duration,
		Sidecar:      container.SidecarSpec{Image: s.image, Pull: s.pull},
		InjectCgroup: s.injectCgroup,
		DryRun:       s.gp.DryRun,
	}
	result, err := s.client.StressContainer(ctx, req)
	if err != nil {
		return fmt.Errorf("stress test failed: %w", err)
	}
	if s.gp.DryRun {
		return nil
	}
	t.Applied()
	timer := time.NewTimer(s.duration)
	defer timer.Stop()
	select {
	case out := <-result.Output:
		log.WithField("stdout", out).Debug("stress-ng completed")
	case e := <-result.Errors:
		return fmt.Errorf("stress-ng failed with error: %w", e)
	case <-ctx.Done():
		log.Debug("stop stress test on containers by stop event")
		return s.stopStress(ctx, result.SidecarID, t)
	case <-timer.C:
		log.WithField("duration", s.duration).Debug("stop stress containers after duration")
		return s.stopStress(ctx, result.SidecarID, t)
	}
	return nil
}

// stopStress stops the stress-ng sidecar and reports the outcome to t.
func (s *stressCommand) stopStress(ctx context.Context, sidecarID string, t *chaos.Target) error {
	// cleanup must run even when parent ctx is canceled (or cancels simultaneously
	// with the timer); preserve values but strip cancellation
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), defaultStopTimeout)
	defer cleanupCancel()
	err := s.client.StopContainerWithID(cleanupCtx, sidecarID, defaultStopTimeout, s.gp.DryRun)
	t.Reverted(err)
	if err != nil {
		return fmt.Errorf("failed to stop stress-ng container: %w", err)
	}
	return nil
}
