// Package netns holds the runtime-agnostic part of the tc and iptables work
// the runtime adapters do in a target's network namespace: request
// translation for pkg/tc and fault-state inspection. Each adapter supplies
// only the way to run a command (sidecar or exec in the target).
package netns

import (
	"fmt"
	"net"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/tc"
)

// Runner runs argv in the target's network namespace and returns its stdout.
// A non-zero exit must be an error.
type Runner func(argv []string) (string, error)

// IPTablesCommands returns the iptables argv list (without the binary) for
// req, built by the single pkg/tc builder.
func IPTablesCommands(req *ctr.IPTablesRequest) [][]string {
	return tc.IPTablesCommands(&tc.IPTablesRequest{
		CmdPrefix: req.CmdPrefix,
		CmdSuffix: req.CmdSuffix,
		SrcIPs:    cidrs(req.SrcIPs),
		DstIPs:    cidrs(req.DstIPs),
		SPorts:    req.SPorts,
		DPorts:    req.DPorts,
		RunID:     req.RunID,
	})
}

func cidrs(nets []*net.IPNet) []string {
	out := make([]string, 0, len(nets))
	for _, n := range nets {
		out = append(out, n.String())
	}
	return out
}

// Inspect lists qdiscs and iptables rules through run and returns the Pumba
// fault state. A failed command or unreadable output is an error, never an
// empty state.
func Inspect(run Runner) (*ctr.FaultState, error) {
	qdiscOut, err := run(tc.QdiscShowCommand())
	if err != nil {
		return nil, fmt.Errorf("failed to list qdiscs: %w", err)
	}
	qdiscs, err := tc.ParseQdiscs(qdiscOut)
	if err != nil {
		return nil, fmt.Errorf("failed to parse qdiscs: %w", err)
	}
	rulesOut, err := run(tc.IPTablesShowCommand())
	if err != nil {
		return nil, fmt.Errorf("failed to list iptables rules: %w", err)
	}
	rules, err := tc.ParseTaggedRules(rulesOut)
	if err != nil {
		return nil, fmt.Errorf("failed to parse iptables rules: %w", err)
	}
	state := &ctr.FaultState{}
	for _, q := range qdiscs {
		state.Qdiscs = append(state.Qdiscs, ctr.PumbaQdisc{Device: q.Device, Kind: q.Kind, Ownership: ctr.QdiscOwnership(q.Ownership)})
	}
	for _, r := range rules {
		state.Rules = append(state.Rules, ctr.PumbaRule{RunID: r.RunID, Chain: r.Chain, Spec: r.Spec})
	}
	return state, nil
}
