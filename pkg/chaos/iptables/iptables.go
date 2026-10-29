package iptables

import (
	"context"
	"fmt"
	"time"

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

// cleanupTimeout caps how long the iptables-cleanup sidecar cycle is allowed
// to run after abort or scheduled stop. Independent of --duration so a
// 1h chaos run does not give cleanup an hour to complete.
const cleanupTimeout = 30 * time.Second

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
		// rules before the failing one may already be installed; remove them. The
		// failing rule was never installed, so its delete always errors: log it, do
		// not return it, or every partial install reports a cleanup failure.
		if cleanupErr := stopIPTables(ctx, client, delReq); cleanupErr != nil {
			logger.WithError(cleanupErr).Warn("failed to stop iptables container after failure (expected for the rule that did not install)")
		}
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
		if err := stopIPTables(ctx, client, delReq); err != nil {
			logger.WithError(err).Warn("failed to stop iptables container (container may have been removed)")
		}
	case <-stopCtx.Done():
		logger.Debug("stopping iptables command on timeout")
		if err := stopIPTables(ctx, client, delReq); err != nil {
			logger.WithError(err).Warn("failed to stop iptables container (container may have been removed)")
		}
	}
	return nil
}

// stopIPTables removes the rules via delReq. It runs on a context detached from
// ctx so cleanup still runs after abort, bounded by cleanupTimeout.
func stopIPTables(ctx context.Context, client iptablesClient, delReq *container.IPTablesRequest) error {
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cleanupCancel()
	return client.StopIPTablesContainer(cleanupCtx, delReq)
}
