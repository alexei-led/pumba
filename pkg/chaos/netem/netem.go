package netem

import (
	"context"
	"net"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// `netem` base command
type netemCommand struct {
	client   container.Client
	names    []string
	pattern  string
	labels   []string
	iface    string
	ips      []*net.IPNet
	sports   []string
	dports   []string
	duration time.Duration
	image    string
	pull     bool
	limit    int
	dryRun   bool
}

// Params common params for netem traffic shaping command
type Params struct {
	// network interface
	Iface string
	// target IP addresses
	Ips []*net.IPNet
	// egress port list (comma separated)
	Sports []string
	// ingress port list (comma separated)
	Dports []string
	// duration of the traffic shaping
	Duration time.Duration
	// image name
	Image string
	// force pull image
	Pull bool
	// limit the number of target containers
	Limit int
}

func newNetemCommand(client container.Client, gparams *chaos.GlobalParams, params *Params) netemCommand {
	return netemCommand{
		client:   client,
		names:    gparams.Names,
		pattern:  gparams.Pattern,
		labels:   gparams.Labels,
		dryRun:   gparams.DryRun,
		iface:    params.Iface,
		ips:      params.Ips,
		sports:   params.Sports,
		dports:   params.Dports,
		duration: params.Duration,
		image:    params.Image,
		pull:     params.Pull,
		limit:    params.Limit,
	}
}

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
	err := client.NetemContainer(ctx, c, netInterface, cmd, ips, sports, dports, duration, tcimage, pull, dryRun)
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
		if err != nil {
			return errors.Wrap(err, "failed to stop netem container")
		}
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
		if err != nil {
			return errors.Wrap(err, "failed to stop netem container")
		}
	}
	return nil
}
