package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

const (
	// DeafultWaitTime time to wait before stopping container (in seconds)
	DeafultWaitTime = 5
)

// stopClient is the narrow interface needed by the stop command.
type stopClient interface {
	container.Lister
	StopContainer(context.Context, *container.Container, int, bool) error
	StartContainer(context.Context, *container.Container, bool) error
}

// `docker stop` command
type stopCommand struct {
	client   stopClient
	gp       *chaos.GlobalParams
	restart  bool
	duration time.Duration
	waitTime int
	limit    int
}

// NewStopCommand create new Stop Command instance
func NewStopCommand(client stopClient, params *chaos.GlobalParams, restart bool, duration time.Duration, waitTime, limit int) chaos.Command {
	if waitTime <= 0 {
		waitTime = DeafultWaitTime
	}
	return &stopCommand{
		client:   client,
		gp:       params,
		restart:  restart,
		duration: duration,
		waitTime: waitTime,
		limit:    limit}
}

// Run stop command
func (s *stopCommand) Run(ctx context.Context, random bool) error {
	log.WithFields(log.Fields{
		"names":    s.gp.Names,
		"pattern":  s.gp.Pattern,
		"labels":   s.gp.Labels,
		"duration": s.duration,
		"waitTime": s.waitTime,
		"limit":    s.limit,
		"random":   random,
	}).Debug("stopping all matching containers")
	var stopped []heldTarget
	err := chaos.RunOnContainers(ctx, s.client, s.gp, s.limit, random, false,
		func(ctx context.Context, c *container.Container, t *chaos.Target) error {
			log.WithFields(log.Fields{"container": c, "waitTime": s.waitTime}).Debug("stopping container")
			if sErr := s.client.StopContainer(ctx, c, s.waitTime, s.gp.DryRun); sErr != nil {
				log.WithError(sErr).Warn("failed to stop container")
				if s.restart {
					return revertInterrupted(ctx, s.restartTimeout(), c, t, sErr, s.restartInterrupted)
				}
				return sErr
			}
			t.Applied()
			if s.restart {
				stopped = append(stopped, holdTarget(c, t))
			}
			return nil
		})
	// (re)start stopped containers after duration or on ctx.Done()
	return errors.Join(err, revertAfter(ctx, s.duration, s.gp.DryRun, stopped, s.startStoppedContainer))
}

// stopBound is the longest StopContainer can take: the graceful wait plus
// the wait after the SIGKILL fallback.
func (s *stopCommand) stopBound() time.Duration {
	return 2 * time.Duration(s.waitTime) * time.Second
}

// restartTimeout bounds restartInterrupted: finishing the stop, then the start.
func (s *stopCommand) restartTimeout() time.Duration {
	return s.stopBound() + revertTimeout
}

// restartInterrupted reverts a stop that ctx cut off mid-call. The runtime may
// have sent the stop signal and then given up waiting (and on the SIGKILL
// fallback), so the container can still be running or exiting; a start now is
// a no-op and the container stays stopped once it exits. So finish the stop
// first, with the same wait time and SIGKILL escalation, then start it. ctx
// must already be detached from the cancelled one.
func (s *stopCommand) restartInterrupted(ctx context.Context, c *container.Container) error {
	stopCtx, cancel := context.WithTimeout(ctx, s.stopBound())
	defer cancel()
	if err := s.client.StopContainer(stopCtx, c, s.waitTime, s.gp.DryRun); err != nil {
		// Expected when the container already exited (kill of a stopped
		// container fails); whether the start succeeds decides cleanup.
		log.WithError(err).WithField("container", c).Debug("failed to finish interrupted stop")
	}
	return s.startStoppedContainer(ctx, c)
}

// start a previously stopped container after duration on exit
func (s *stopCommand) startStoppedContainer(ctx context.Context, c *container.Container) error {
	log.WithField("container", c).Debug("start stopped container")
	if err := s.client.StartContainer(ctx, c, s.gp.DryRun); err != nil {
		return fmt.Errorf("failed to start stopped container: %w", err)
	}
	return nil
}
