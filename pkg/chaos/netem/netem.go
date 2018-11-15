package netem

import (
	"context"
	"net"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// run network emulation command, stop netem on timeout or abort
func runNetem(ctx context.Context, client container.Client, container container.Container, netInterface string, cmd []string, ips []net.IP, duration time.Duration, tcimage string, pull bool, dryRun bool) error {
	log.WithFields(log.Fields{
		"id":       container.ID(),
		"name":     container.Name(),
		"iface":    netInterface,
		"netem":    cmd,
		"ips":      ips,
		"duration": duration,
		"tc-image": tcimage,
		"pull":     pull,
	}).Debug("running netem command")
	var err error
	err = client.NetemContainer(ctx, container, netInterface, cmd, ips, duration, tcimage, pull, dryRun)
	if err != nil {
		log.WithError(err).Error("failed to start netem for container")
		return err
	}

	// create new context with timeout for canceling
	stopCtx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	// wait for specified duration and then stop netem (where it applied) or stop on ctx.Done()
	select {
	case <-ctx.Done():
		log.WithFields(log.Fields{
			"id":       container.ID(),
			"name":     container.Name(),
			"iface":    netInterface,
			"ips":      ips,
			"tc-image": tcimage,
		}).Debug("stopping netem command on abort")
		// use different context to stop netem since parent context is canceled
		err = client.StopNetemContainer(context.Background(), container, netInterface, ips, tcimage, pull, dryRun)
	case <-stopCtx.Done():
		log.WithFields(log.Fields{
			"id":       container.ID(),
			"name":     container.Name(),
			"iface":    netInterface,
			"ips":      ips,
			"tc-image": tcimage,
		}).Debug("stopping netem command on timout")
		// use parent context to stop netem in container
		err = client.StopNetemContainer(context.Background(), container, netInterface, ips, tcimage, pull, dryRun)
	}
	return err
}
