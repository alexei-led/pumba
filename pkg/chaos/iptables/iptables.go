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

// targetReport is the part of *chaos.Target runIPTables reports through.
type targetReport interface {
	Applied()
	Reverted(err error)
}

// cleanupTimeout caps how long the iptables-cleanup sidecar cycle is allowed
// to run after abort or scheduled stop. Independent of --duration so a
// 1h chaos run does not give cleanup an hour to complete.
const cleanupTimeout = 30 * time.Second

// run iptables command, stop iptables on timeout, abort or failed install. The add/del prefix
// pair distinguishes the rule installation command (-I/-A/-N) from its mirror
// removal command (-D); both share the rest of the request fields. Reports to
// t when the rules are installed and how their removal went; a failed removal
// after a successful install is returned as an error. Dry-runs do not wait for
// the duration.
func runIPTables(ctx context.Context, client iptablesClient, addReq, delReq *container.IPTablesRequest, t targetReport) error {
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
		// rules before the failing one may already be installed; remove them. Cleanup
		// errors are logged, not returned or reported: deleting the rule that never
		// installed fails on every partial install, and returning it would bury the add error.
		if cleanupErr := stopIPTables(ctx, client, delReq); cleanupErr != nil {
			logger.WithError(cleanupErr).Warn("failed to stop iptables after failed install; rules may remain in the target")
		}
		return fmt.Errorf("iptables failed: %w", err)
	}
	logger.Debug("iptables command started")
	t.Applied()
	if !addReq.DryRun {
		waitIPTables(ctx, addReq.Duration, logger)
	}
	// stopIPTables uses context.WithoutCancel so cleanup succeeds even if the parent
	// ctx is canceled or inherited a deadline that has elapsed alongside the wait.
	err := stopIPTables(ctx, client, delReq)
	t.Reverted(err)
	if err != nil {
		logger.WithError(err).Warn("failed to stop iptables container (container may have been removed)")
		return fmt.Errorf("stopping iptables: %w", err)
	}
	return nil
}

// waitIPTables waits for the requested duration or parent cancellation.
func waitIPTables(ctx context.Context, duration time.Duration, logger *log.Entry) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		logger.Debug("stopping iptables command on abort")
	case <-timer.C:
		logger.Debug("stopping iptables command on timeout")
	}
}

// stopIPTables removes the rules via delReq. It runs on a context detached from
// ctx so cleanup still runs after abort, bounded by cleanupTimeout.
func stopIPTables(ctx context.Context, client iptablesClient, delReq *container.IPTablesRequest) error {
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cleanupCancel()
	return client.StopIPTablesContainer(cleanupCtx, delReq)
}
