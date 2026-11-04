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

// Runner runs argv in the target's network namespace and returns its stdout
// and stderr. A non-zero exit must be an error.
type Runner func(argv []string) (stdout, stderr string, err error)

// Binaries the netem and iptables commands run.
const (
	shellBinary    = "sh"
	iptablesBinary = "iptables"
)

// NetemStart returns the argv that installs req's netem qdisc:
// sh -ec <script>. Runtimes execute exactly this argv; NetemPlan reports it.
func NetemStart(req *ctr.NetemRequest) ([]string, error) {
	args, err := tc.Start(netemRequest(req))
	if err != nil {
		return nil, err
	}
	return append([]string{shellBinary}, args...), nil
}

// NetemStop returns the ownership-verifying cleanup argv.
func NetemStop(req *ctr.NetemRequest) ([]string, error) {
	args, err := tc.Stop(netemRequest(req))
	if err != nil {
		return nil, err
	}
	return append([]string{shellBinary}, args...), nil
}

// NetemPlan validates both scripts before returning exact apply/revert argv.
func NetemPlan(req *ctr.NetemRequest) (*ctr.CommandPlan, error) {
	start, err := NetemStart(req)
	if err != nil {
		return nil, err
	}
	stop, err := NetemStop(req)
	if err != nil {
		return nil, err
	}
	return newPlan(req.Sidecar, [][]string{start}, [][]string{stop}), nil
}

func netemRequest(req *ctr.NetemRequest) *tc.NetemRequest {
	return &tc.NetemRequest{
		Interface: req.Interface, Command: req.Command, IPs: cidrs(req.IPs),
		SPorts: req.SPorts, DPorts: req.DPorts, MatchAll: req.MatchAll,
	}
}

// IPTablesCommands translates the request through the single validated tc builder.
func IPTablesCommands(req *ctr.IPTablesRequest) ([][]string, error) {
	commands, err := tc.IPTablesCommands(&tc.IPTablesRequest{
		CmdPrefix: req.CmdPrefix, CmdSuffix: req.CmdSuffix,
		SrcIPs: cidrs(req.SrcIPs), DstIPs: cidrs(req.DstIPs),
		SPorts: req.SPorts, DPorts: req.DPorts, MatchAll: req.MatchAll, RunID: req.RunID,
	})
	if err != nil {
		return nil, err
	}
	for i, args := range commands {
		commands[i] = append([]string{iptablesBinary}, args...)
	}
	return commands, nil
}

// IPTablesPlan validates both operations before returning their exact argv.
func IPTablesPlan(add, del *ctr.IPTablesRequest) (*ctr.CommandPlan, error) {
	apply, err := IPTablesCommands(add)
	if err != nil {
		return nil, err
	}
	revert, err := IPTablesCommands(del)
	if err != nil {
		return nil, err
	}
	return newPlan(add.Sidecar, apply, revert), nil
}

// newPlan places the commands in a sidecar when the request names an image,
// as every runtime does when it executes them.
func newPlan(sidecar ctr.SidecarSpec, apply, revert [][]string) *ctr.CommandPlan {
	if sidecar.Image == "" {
		return &ctr.CommandPlan{Via: ctr.ViaExec, Apply: apply, Revert: revert}
	}
	return &ctr.CommandPlan{Via: ctr.ViaSidecar, Image: sidecar.Image, Apply: apply, Revert: revert}
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
	qdiscOut, _, err := run(tc.QdiscShowCommand())
	if err != nil {
		return nil, fmt.Errorf("failed to list qdiscs: %w", err)
	}
	qdiscs, err := tc.ParseQdiscs(qdiscOut)
	if err != nil {
		return nil, fmt.Errorf("failed to parse qdiscs: %w", err)
	}
	rules, err := inspectRules(run)
	if err != nil {
		return nil, err
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

// inspectRules establishes coverage of both independent iptables backends.
// nft warns about legacy tables, but legacy never warns about nft rules, so
// a legacy default always requires an explicit nft listing. Unknown backends
// and failed listings are errors, never evidence of a clean namespace.
func inspectRules(run Runner) ([]tc.TaggedRule, error) {
	out, stderr, err := run(tc.IPTablesShowCommand())
	if err != nil {
		return nil, fmt.Errorf("failed to list iptables rules: %w", err)
	}
	parsed, err := tc.ParseTaggedRules(out)
	if err != nil {
		return nil, fmt.Errorf("failed to parse iptables rules: %w", err)
	}
	version, _, err := run(tc.IPTablesVersionCommand())
	if err != nil {
		return nil, fmt.Errorf("failed to identify iptables backend: %w", err)
	}
	nft, err := tc.IPTablesUsesNFT(version)
	if err != nil {
		return nil, err
	}
	if !nft {
		nftOut, _, err := run(tc.IPTablesNFTShowCommand())
		if err != nil {
			return nil, fmt.Errorf("failed to list iptables-nft rules: %w", err)
		}
		nftRules, err := tc.ParseTaggedRules(nftOut)
		if err != nil {
			return nil, fmt.Errorf("failed to parse iptables-nft rules: %w", err)
		}
		return append(parsed.Rules, nftRules.Rules...), nil
	}
	if !parsed.LegacyTablesPresent && !tc.LegacyTablesPresent(stderr) {
		return parsed.Rules, nil
	}
	legacyOut, _, err := run(tc.IPTablesLegacyShowCommand())
	if err != nil {
		return nil, fmt.Errorf("iptables-legacy tables present but listing them failed: %w", err)
	}
	legacy, err := tc.ParseTaggedRules(legacyOut)
	if err != nil {
		return nil, fmt.Errorf("failed to parse iptables-legacy rules: %w", err)
	}
	return append(parsed.Rules, legacy.Rules...), nil
}
