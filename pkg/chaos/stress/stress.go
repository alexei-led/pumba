package stress

import (
	"context"
	"errors"
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
	stressCleanupTimeout = 45 * time.Second
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
		RunID:        s.gp.RunID,
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	result, err := s.client.StressContainer(workCtx, req)
	if err != nil {
		if cleanup, ok := errors.AsType[*container.StressCleanupError](err); ok {
			t.Reverted(cleanup.Err)
		}
		return fmt.Errorf("stress test failed: %w", err)
	}
	if s.gp.DryRun {
		return nil
	}
	t.Applied()
	resultErr, cleanupErr := waitStress(workCtx, result, s.duration, stressCleanupTimeout, cancel)
	t.Reverted(cleanupErr)
	if resultErr == nil {
		log.Debug("stress-ng completed")
	}
	return resultErr
}

// Both channels close only after runtime cleanup. Output (or EOF on just one
// channel) is not proof that the owned workload has been reaped.
func waitStress(ctx context.Context, result *container.StressResult, duration, cleanupTimeout time.Duration, cancel context.CancelFunc) (error, error) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	output, errs := result.Output, result.Errors
	done := ctx.Done()
	var cleanupDone <-chan struct{}
	var resultErr, cleanupErr error
	var cleanupCancel context.CancelFunc
	defer func() {
		if cleanupCancel != nil {
			cleanupCancel()
		}
	}()
	for output != nil || errs != nil {
		select {
		case out, ok := <-output:
			if !ok {
				output = nil
			} else {
				log.WithField("stdout", out).Debug("stress-ng output")
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			resultErr = errors.Join(resultErr, err)
			if cleanup, ok := errors.AsType[*container.StressCleanupError](err); ok {
				cleanupErr = errors.Join(cleanupErr, cleanup.Err)
			}
		case <-timer.C:
			cancel()
		case <-done:
			done = nil
			cleanupCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
			cleanupCancel, cleanupDone = stop, cleanupCtx.Done()
		case <-cleanupDone:
			err := fmt.Errorf("timed out waiting for stress workload cleanup: %w", context.DeadlineExceeded)
			return errors.Join(resultErr, err), errors.Join(cleanupErr, err)
		}
	}
	return resultErr, cleanupErr
}
