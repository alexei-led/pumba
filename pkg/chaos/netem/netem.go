package netem

import (
	"context"
	"net"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// run network emulation command, stop netem on timeout or abort
func runNetem(ctx context.Context, client container.Client, container container.Container, netInterface string, cmd []string, ips []net.IP, duration time.Duration, tcimage string, dryRun bool) error {
	log.WithField("netem", cmd).Debug("running netem command")
	var err error
	err = client.NetemContainer(ctx, container, netInterface, cmd, ips, duration, tcimage, dryRun)
	if err != nil {
		log.WithError(err).Error("failed to start netem for container")
		return err
	}

	// wait for specified duration and then stop netem (where it applied) or stop on ctx.Done()
	select {
	case <-ctx.Done():
		log.Debugf("stopping netem command")
		// use different context to stop netem since parent context is canceled
		err = client.StopNetemContainer(ctx, container, netInterface, ips, tcimage, dryRun)
	}

	return err
}
