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
func NetemStart(req *ctr.NetemRequest) []string {
	return append([]string{shellBinary}, tc.Start(netemRequest(req))...)
}

// NetemStop returns the argv that removes req's netem qdisc after verifying
// Pumba owns it: sh -ec <script>.
func NetemStop(req *ctr.NetemRequest) []string {
	return append([]string{shellBinary}, tc.Stop(netemRequest(req))...)
}

// NetemPlan returns the commands NetemStart and NetemStop build for req and
// where they run.
func NetemPlan(req *ctr.NetemRequest) *ctr.CommandPlan {
	return newPlan(req.Sidecar, [][]string{NetemStart(req)}, [][]string{NetemStop(req)})
}

func netemRequest(req *ctr.NetemRequest) *tc.NetemRequest {
	return &tc.NetemRequest{
		Interface: req.Interface,
		Command:   req.Command,
		IPs:       cidrs(req.IPs),
		SPorts:    req.SPorts,
		DPorts:    req.DPorts,
	}
}

// IPTablesCommands returns one iptables argv (binary first) per rule of req,
// built by the single pkg/tc builder.
func IPTablesCommands(req *ctr.IPTablesRequest) [][]string {
	commands := tc.IPTablesCommands(&tc.IPTablesRequest{
		CmdPrefix: req.CmdPrefix,
		CmdSuffix: req.CmdSuffix,
		SrcIPs:    cidrs(req.SrcIPs),
		DstIPs:    cidrs(req.DstIPs),
		SPorts:    req.SPorts,
		DPorts:    req.DPorts,
		RunID:     req.RunID,
	})
	for i, args := range commands {
		commands[i] = append([]string{iptablesBinary}, args...)
	}
	return commands
}

// IPTablesPlan returns the commands IPTablesCommands builds for add (apply)
// and del (revert) and where they run. Both requests share the sidecar hint.
func IPTablesPlan(add, del *ctr.IPTablesRequest) *ctr.CommandPlan {
	return newPlan(add.Sidecar, IPTablesCommands(add), IPTablesCommands(del))
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
