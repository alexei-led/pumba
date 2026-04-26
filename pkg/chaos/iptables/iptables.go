package iptables

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

const (
	ProtocolAny  = "any"
	ProtocolTCP  = "tcp"
	ProtocolUDP  = "udp"
	ProtocolICMP = "icmp"
)

// iptablesClient is the narrow interface needed by iptables commands.
type iptablesClient interface {
	container.Lister
	container.IPTables
}

// `iptable` base command
type ipTablesCommand struct {
	client   iptablesClient
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

func newIPTablesCommand(client iptablesClient, gparams *chaos.GlobalParams, params *Params) ipTablesCommand {
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

// run iptables command, stop iptables on timeout or abort. The add/del prefix
// pair distinguishes the rule installation command (-I/-A/-N) from its mirror
// removal command (-D); both share the rest of the request fields.
func runIPTables(ctx context.Context, client iptablesClient, addReq, delReq *container.IPTablesRequest) error {
	logger := log.WithFields(log.Fields{
		"id":           addReq.Container.ID(),
		"name":         addReq.Container.Name(),
		"addCmdPrefix": addReq.CmdPrefix,
		"delCmdPrefix": delReq.CmdPrefix,
		"cmdSuffix":    addReq.CmdSuffix,
		"srcIPs":       addReq.SrcIPs,
		"dstIPs":       addReq.DstIPs,
		"sports":       addReq.SPorts,
		"dports":       addReq.DPorts,
		"duration":     addReq.Duration,
		"image":        addReq.Sidecar.Image,
		"pull":         addReq.Sidecar.Pull,
	})
	logger.Debug("running iptables command")
	if err := client.IPTablesContainer(ctx, addReq); err != nil {
		return fmt.Errorf("iptables failed: %w", err)
	}
	logger.Debug("iptables command started")

	// create new context with timeout for canceling
	stopCtx, cancel := context.WithTimeout(context.Background(), addReq.Duration)
	defer cancel()
	// wait for specified duration and then stop iptables (where it applied) or stop on ctx.Done()
	// use context.WithoutCancel so cleanup succeeds even if the parent ctx is canceled
	// or if it inherited a deadline that has elapsed alongside stopCtx.
	select {
	case <-ctx.Done():
		logger.Debug("stopping iptables command on abort")
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), addReq.Duration)
		defer cleanupCancel()
		if err := client.StopIPTablesContainer(cleanupCtx, delReq); err != nil {
			logger.WithError(err).Warn("failed to stop iptables container (container may have been removed)")
		}
	case <-stopCtx.Done():
		logger.Debug("stopping iptables command on timeout")
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), addReq.Duration)
		defer cleanupCancel()
		if err := client.StopIPTablesContainer(cleanupCtx, delReq); err != nil {
			logger.WithError(err).Warn("failed to stop iptables container (container may have been removed)")
		}
	}
	return nil
}
