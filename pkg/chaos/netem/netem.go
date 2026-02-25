package netem

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// netemClient is the narrow interface needed by all netem commands.
type netemClient interface {
	container.Lister
	container.Netem
}

// `netem` base command
type netemCommand struct {
	client   netemClient
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

func newNetemCommand(client netemClient, gparams *chaos.GlobalParams, params *Params) netemCommand {
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
func runNetem(ctx context.Context, client netemClient, c *container.Container, netInterface string, cmd []string, ips []*net.IPNet, sports, dports []string, duration time.Duration, tcimage string, pull, dryRun bool) error {
	logger := log.WithFields(log.Fields{
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
	})
	logger.Debug("running netem command")
	err := client.NetemContainer(ctx, c, netInterface, cmd, ips, sports, dports, duration, tcimage, pull, dryRun)
	if err != nil {
		return fmt.Errorf("netem failed: %w", err)
	}
	logger.Debug("netem command started")

	// create new context with timeout for canceling
	stopCtx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	// wait for specified duration and then stop netem (where it applied) or stop on ctx.Done()
	select {
	case <-ctx.Done():
		logger.Debug("stopping netem command on abort")
		// use context.WithoutCancel so cleanup succeeds even if the parent ctx is canceled
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), duration)
		defer cleanupCancel()
		err = client.StopNetemContainer(cleanupCtx, c, netInterface, ips, sports, dports, tcimage, pull, dryRun)
		if err != nil {
			logger.WithError(err).Warn("failed to stop netem container (container may have been removed)")
		}
	case <-stopCtx.Done():
		logger.Debug("stopping netem command on timeout")
		err = client.StopNetemContainer(ctx, c, netInterface, ips, sports, dports, tcimage, pull, dryRun)
		if err != nil {
			logger.WithError(err).Warn("failed to stop netem container (container may have been removed)")
		}
	}
	return nil
}
