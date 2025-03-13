package iptables

import (
	"context"
	"net"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// `iptable` base command
type ipTablesCommand struct {
	client   container.Client
	names    []string
	pattern  string
	labels   []string
	iface    string
	protocol string
	srcIPs   []*net.IPNet
	dstIPs   []*net.IPNet
	sports   []string
	dports   []string
	duration time.Duration
	image    string
	pull     bool
	limit    int
	dryRun   bool
}

// Params common params for iptables loss command
type Params struct {
	// network interface
	Iface string
	// protocol
	Protocol string
	// source IP addresses
	SrcIPs []*net.IPNet
	// target IP addresses
	DstIPs []*net.IPNet
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

func newIPTablesCommand(client container.Client, gparams *chaos.GlobalParams, params *Params) ipTablesCommand {
	return ipTablesCommand{
		client:   client,
		names:    gparams.Names,
		pattern:  gparams.Pattern,
		labels:   gparams.Labels,
		dryRun:   gparams.DryRun,
		iface:    params.Iface,
		protocol: params.Protocol,
		srcIPs:   params.SrcIPs,
		dstIPs:   params.DstIPs,
		sports:   params.Sports,
		dports:   params.Dports,
		duration: params.Duration,
		image:    params.Image,
		pull:     params.Pull,
		limit:    params.Limit,
	}
}

// run iptables command, stop iptables on timeout or abort
func runIPTables(ctx context.Context, client container.Client, c *container.Container, addCmdPrefix []string, delCmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string, duration time.Duration, image string, pull, dryRun bool) error {
	logger := log.WithFields(log.Fields{
		"id":           c.ID(),
		"name":         c.Name(),
		"addCmdPrefix": addCmdPrefix,
		"delCmdPrefix": delCmdPrefix,
		"cmdSuffix":    cmdSuffix,
		"srcIPs":       srcIPs,
		"dstIPs":       dstIPs,
		"sports":       sports,
		"dports":       dports,
		"duration":     duration,
		"image":        image,
		"pull":         pull,
	})
	logger.Debug("running iptables command")
	err := client.IPTablesContainer(ctx, c, addCmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports, duration, image, pull, dryRun)
	if err != nil {
		return errors.Wrap(err, "iptables failed")
	}
	logger.Debug("iptables command started")

	// create new context with timeout for canceling
	stopCtx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	// wait for specified duration and then stop iptables(where it applied) or stop on ctx.Done()
	select {
	case <-ctx.Done():
		logger.Debug("stopping iptables command on abort")
		// use different context to stop iptables since parent context is canceled
		err = client.StopIPTablesContainer(context.Background(), c, delCmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports, image, pull, dryRun)
		if err != nil {
			return errors.Wrap(err, "failed to stop iptables container")
		}
	case <-stopCtx.Done():
		logger.Debug("stopping iptables command on timout")
		// use parent context to stop iptables in container
		err = client.StopIPTablesContainer(context.Background(), c, delCmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports, image, pull, dryRun)
		if err != nil {
			return errors.Wrap(err, "failed to stop tables container")
		}
	}
	return nil
}
