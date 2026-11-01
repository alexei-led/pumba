package tc

import (
	"fmt"
	"regexp"
	"strings"
)

// Ownership classifies a qdisc tree that uses PumbaRootHandle.
type Ownership string

const (
	// OwnershipPumba means the topology matches what Pumba installs: a netem
	// root, or a prio root with exactly the sfq, sfq, netem bands.
	OwnershipPumba Ownership = "pumba-owned"
	// OwnershipSuspicious means the Pumba handle is present but the topology
	// is not one Pumba installs.
	OwnershipSuspicious Ownership = "suspicious"
)

// QdiscShowCommand lists every qdisc in the netns, one line per qdisc.
func QdiscShowCommand() []string {
	return []string{"tc", "qdisc", "show"}
}

// IPTablesShowCommand lists the filter table rules as iptables arguments.
func IPTablesShowCommand() []string {
	return []string{"iptables", "-S"}
}

// IPTablesVersionCommand identifies which backend the default iptables uses.
func IPTablesVersionCommand() []string {
	return []string{"iptables", "--version"}
}

// IPTablesNFTShowCommand lists the filter table rules of the iptables-nft
// backend, which iptables-legacy cannot see or warn about.
func IPTablesNFTShowCommand() []string {
	return []string{"iptables-nft", "-S"}
}

var iptablesVersion = regexp.MustCompile(`^iptables v\d+\.\d+\.\d+ \((nf_tables|legacy)\)$`)

// IPTablesUsesNFT identifies the backend from iptables --version. Unrecognized
// output cannot establish backend coverage and must not imply an empty netns.
func IPTablesUsesNFT(output string) (bool, error) {
	match := iptablesVersion.FindStringSubmatch(strings.TrimSpace(output))
	if match == nil {
		return false, fmt.Errorf("unrecognized iptables version: %q", strings.TrimSpace(output))
	}
	return match[1] == "nf_tables", nil
}

// IPTablesLegacyShowCommand lists the filter table rules of the iptables-legacy
// backend, a rule set separate from the one iptables-nft shows.
func IPTablesLegacyShowCommand() []string {
	return []string{"iptables-legacy", "-S"}
}

// legacyTablesWarning is what iptables-nft prints when iptables-legacy tables
// exist in the netns that it cannot show.
const legacyTablesWarning = "# Warning: iptables-legacy tables present"

// LegacyTablesPresent reports whether iptables-nft output (stdout or stderr)
// says that iptables-legacy tables exist.
func LegacyTablesPresent(output string) bool {
	return strings.Contains(output, legacyTablesWarning)
}

// TaggedRules is the parsed `iptables -S` output. LegacyTablesPresent means
// the listing is incomplete: iptables-legacy rules need a second listing.
type TaggedRules struct {
	Rules               []TaggedRule
	LegacyTablesPresent bool
}

// PumbaQdisc is a device whose qdiscs use PumbaRootHandle. Qdiscs carry no
// run ID, so a PumbaQdisc is never attributed to a run.
type PumbaQdisc struct {
	Device    string
	Kind      string
	Ownership Ownership
}

// TaggedRule is an iptables rule whose comment is "pumba:<run-id>".
type TaggedRule struct {
	RunID string
	Chain string
	Spec  string
}

type qdisc struct {
	kind, handle, device, parent string
	root                         bool
}

// pumbaBands is the scoped topology Pumba installs under a prio root, keyed by
// parent class.
var pumbaBands = map[string]qdisc{
	PumbaRootHandle + "1": {kind: "sfq", handle: firstBandHandle},
	PumbaRootHandle + "2": {kind: "sfq", handle: secondBandHandle},
	PumbaRootHandle + "3": {kind: "netem", handle: netemBandHandle},
}

// ParseQdiscs parses `tc qdisc show` output and returns one entry per device
// that has a qdisc with PumbaRootHandle, in output order. Lines that are not
// qdiscs fail the parse: unreadable output is never reported as clean.
func ParseQdiscs(output string) ([]PumbaQdisc, error) {
	var devices []string
	byDevice := map[string][]qdisc{}
	for line := range strings.Lines(output) {
		if strings.TrimSpace(line) == "" || line[0] == ' ' || line[0] == '\t' {
			continue // indented lines continue the previous qdisc
		}
		q, err := parseQdiscLine(line)
		if err != nil {
			return nil, err
		}
		if _, seen := byDevice[q.device]; !seen {
			devices = append(devices, q.device)
		}
		byDevice[q.device] = append(byDevice[q.device], q)
	}
	var found []PumbaQdisc
	for _, device := range devices {
		if pq, ok := classify(device, byDevice[device]); ok {
			found = append(found, pq)
		}
	}
	return found, nil
}

func parseQdiscLine(line string) (qdisc, error) {
	fields := strings.Fields(line)
	const minFields = 3 // qdisc <kind> <handle>
	if len(fields) < minFields || fields[0] != "qdisc" {
		return qdisc{}, fmt.Errorf("unexpected tc qdisc output: %q", strings.TrimSpace(line))
	}
	q := qdisc{kind: fields[1], handle: fields[2]}
	for i := minFields; i < len(fields); i++ {
		switch fields[i] {
		case "root":
			q.root = true
		case "dev", "parent":
			if i+1 == len(fields) {
				return qdisc{}, fmt.Errorf("unexpected tc qdisc output: %q", strings.TrimSpace(line))
			}
			if fields[i] == "dev" {
				q.device = fields[i+1]
			} else {
				q.parent = fields[i+1]
			}
			i++
		}
	}
	if q.device == "" {
		return qdisc{}, fmt.Errorf("tc qdisc output without device: %q", strings.TrimSpace(line))
	}
	return q, nil
}

// classify reports the device when any of its qdiscs uses PumbaRootHandle.
// The root qdisc with that handle wins over a non-root one.
func classify(device string, qdiscs []qdisc) (PumbaQdisc, bool) {
	var pumba *qdisc
	var children []qdisc
	for i := range qdiscs {
		q := &qdiscs[i]
		if q.handle == PumbaRootHandle && (pumba == nil || q.root) {
			pumba = q
		}
		if strings.HasPrefix(q.parent, PumbaRootHandle) {
			children = append(children, *q)
		}
	}
	if pumba == nil {
		return PumbaQdisc{}, false
	}
	ownership := OwnershipSuspicious
	if pumba.root && pumbaTopology(pumba.kind, children) {
		ownership = OwnershipPumba
	}
	return PumbaQdisc{Device: device, Kind: pumba.kind, Ownership: ownership}, true
}

func pumbaTopology(rootKind string, children []qdisc) bool {
	switch rootKind {
	case "netem":
		return len(children) == 0
	case "prio":
		if len(children) != len(pumbaBands) {
			return false
		}
		for _, child := range children {
			want, ok := pumbaBands[child.parent]
			if !ok || child.kind != want.kind || child.handle != want.handle {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// ParseTaggedRules parses `iptables -S` output and returns the rules whose
// comment starts with CommentPrefix, in output order. Policy (-P), chain (-N)
// and "#" lines carry no rule, but the iptables-legacy warning is reported in
// LegacyTablesPresent; any other line that is not an iptables spec fails the
// parse.
func ParseTaggedRules(output string) (TaggedRules, error) {
	var res TaggedRules
	for line := range strings.Lines(output) {
		line = strings.TrimSpace(line)
		if LegacyTablesPresent(line) {
			res.LegacyTablesPresent = true
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		args, err := splitArgs(line)
		if err != nil {
			return TaggedRules{}, err
		}
		if !strings.HasPrefix(args[0], "-") {
			return TaggedRules{}, fmt.Errorf("unexpected iptables output: %q", line)
		}
		if args[0] != "-A" || len(args) < 2 {
			continue
		}
		if runID, ok := runComment(args); ok {
			res.Rules = append(res.Rules, TaggedRule{RunID: runID, Chain: args[1], Spec: line})
		}
	}
	return res, nil
}

func runComment(args []string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--comment" {
			if runID, ok := strings.CutPrefix(args[i+1], CommentPrefix); ok {
				return runID, true
			}
		}
	}
	return "", false
}

// splitArgs splits an iptables-save style line into arguments. iptables
// double-quotes a comment that has characters outside [A-Za-z0-9_-] and
// escapes double quotes, backslashes and single quotes inside it with a
// backslash.
func splitArgs(line string) ([]string, error) {
	var args []string
	var arg strings.Builder
	inArg, quoted, escaped := false, false, false
	for _, r := range line {
		switch {
		case escaped:
			arg.WriteRune(r)
			escaped = false
		case quoted && r == '\\':
			escaped = true
		case r == '"':
			quoted = !quoted
			inArg = true
		case !quoted && (r == ' ' || r == '\t'):
			if inArg {
				args = append(args, arg.String())
				arg.Reset()
				inArg = false
			}
		default:
			arg.WriteRune(r)
			inArg = true
		}
	}
	if quoted || escaped {
		return nil, fmt.Errorf("unterminated quote in iptables output: %q", line)
	}
	if inArg {
		args = append(args, arg.String())
	}
	return args, nil
}
