package lifecycle

import (
	"context"
	"errors"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// heldTarget is a container a batch action (pause, stop --restart) changed and
// must revert after fanout, with the func that emits its terminal result.
type heldTarget struct {
	c    *container.Container
	done func(revertErr error)
}

func holdTarget(c *container.Container, t *chaos.Target) heldTarget {
	return heldTarget{c: c, done: t.DeferResult()}
}

// revertTimeout is the least time reverts get, so a short --duration does not
// cut off a slow unpause or start.
const revertTimeout = 30 * time.Second

// revertAfter waits for duration or ctx cancellation (dry-runs do not wait),
// then reverts each held target and reports each target's own outcome, so no
// target is reported before its own revert finished. Reverts always run on a
// context detached from ctx, bounded by max(duration, revertTimeout), so a
// SIGTERM or --max-duration deadline during the reverts cannot cut them off.
func revertAfter(ctx context.Context, duration time.Duration, dryRun bool, held []heldTarget,
	revert func(context.Context, *container.Container) error,
) error {
	if len(held) == 0 {
		return nil
	}
	if !dryRun {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			log.Debug("reverting containers by stop event")
		case <-timer.C:
			log.WithField("duration", duration).Debug("reverting containers after duration")
		}
	}
	revertCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), max(duration, revertTimeout))
	defer cancel()
	var err error
	for _, h := range held {
		e := revert(revertCtx, h.c)
		h.done(e)
		err = errors.Join(err, e)
	}
	return err
}
