package netem

import (
	"context"
	"github.com/pkg/errors"
	"net"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// run network emulation command, stop netem on timeout or abort
func runNetem(ctx context.Context, client container.Client, c *container.Container, netInterface string, cmd []string, ips []*net.IPNet, sports, dports []string, duration time.Duration, tcimage string, pull, dryRun bool) error {
	log.WithFields(log.Fields{
		"id":       c.ID(),
		"name":     c.Name(),
		"iface":    netInterface,
		"netem":    cmd,
		"ips":      ips,
		"sports":   sports,
		"dports":   dports,
		"duration": duration,
		"tc-image": tcimage,
		"pull":     pull,
	}).Debug("running netem command")
	var err error
	err = client.NetemContainer(ctx, c, netInterface, cmd, ips, sports, dports, duration, tcimage, pull, dryRun)
	if err != nil {
		return errors.Wrap(err, "netem failed")
	}

	// create new context with timeout for canceling
	stopCtx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	// wait for specified duration and then stop netem (where it applied) or stop on ctx.Done()
	select {
	case <-ctx.Done():
		log.WithFields(log.Fields{
			"id":       c.ID(),
			"name":     c.Name(),
			"iface":    netInterface,
			"ips":      ips,
			"sports":   sports,
			"dports":   dports,
			"tc-image": tcimage,
		}).Debug("stopping netem command on abort")
		// use different context to stop netem since parent context is canceled
		err = client.StopNetemContainer(context.Background(), c, netInterface, ips, sports, dports, tcimage, pull, dryRun)
	case <-stopCtx.Done():
		log.WithFields(log.Fields{
			"id":       c.ID(),
			"name":     c.Name(),
			"iface":    netInterface,
			"ips":      ips,
			"sports":   sports,
			"dports":   dports,
			"tc-image": tcimage,
		}).Debug("stopping netem command on timout")
		// use parent context to stop netem in container
		err = client.StopNetemContainer(context.Background(), c, netInterface, ips, sports, dports, tcimage, pull, dryRun)
	}
	return err
}
