package tc

import (
	"errors"
	"fmt"
	"strings"
)

// CommentPrefix starts the iptables comment Pumba puts on every rule it
// installs for a run: "pumba:<run-id>".
const CommentPrefix = "pumba:"

// ErrCommentMatchUnavailable reports that iptables could not load the comment
// match (xt_comment), so a run-tagged rule cannot be installed.
var ErrCommentMatchUnavailable = errors.New("iptables comment match (xt_comment) unavailable")

// IPTablesRequest contains the iptables-specific portion of a chaos request.
// CmdPrefix carries the operation (-I/-A/-D) and chain; CmdSuffix carries the
// match and target. IPs are CIDR strings.
type IPTablesRequest struct {
	CmdPrefix []string
	CmdSuffix []string
	SrcIPs    []string
	DstIPs    []string
	SPorts    []string
	DPorts    []string
	// RunID tags every rule with "-m comment --comment pumba:<RunID>". Empty
	// means untagged.
	RunID string
}

// IPTablesCommands returns one iptables argv (without the binary) per
// source IP, destination IP, source port and destination port, in that
// order, or a single argv when there are no filters. The run comment sits
// between the filter and CmdSuffix, so an add spec and its delete spec built
// from the same request match the same rule.
func IPTablesCommands(r *IPTablesRequest) [][]string {
	var commands [][]string
	for _, filter := range []struct {
		flag   string
		values []string
	}{
		{"-s", r.SrcIPs},
		{"-d", r.DstIPs},
		{"--sport", r.SPorts},
		{"--dport", r.DPorts},
	} {
		for _, value := range filter.values {
			commands = append(commands, r.command(filter.flag, value))
		}
	}
	if len(commands) == 0 {
		commands = append(commands, r.command())
	}
	return commands
}

func (r *IPTablesRequest) command(filter ...string) []string {
	cmd := make([]string, 0, len(r.CmdPrefix)+len(filter)+len(r.CmdSuffix))
	cmd = append(cmd, r.CmdPrefix...)
	cmd = append(cmd, filter...)
	if r.RunID != "" {
		cmd = append(cmd, "-m", "comment", "--comment", CommentPrefix+r.RunID)
	}
	return append(cmd, r.CmdSuffix...)
}

// ExplainIPTablesError names a missing comment match as the cause when a
// tagged rule fails to install. Untagged failures and unrelated errors are
// returned unchanged.
func ExplainIPTablesError(err error, tagged bool) error {
	if err == nil || !tagged || !commentMatchMissing(err.Error()) {
		return err
	}
	return fmt.Errorf("%w: Pumba tags rules with -m comment; load the xt_comment kernel module: %w", ErrCommentMatchUnavailable, err)
}

// commentMatchMissing matches the messages iptables-legacy and iptables-nft
// print when the comment extension or its kernel module is missing.
func commentMatchMissing(msg string) bool {
	msg = strings.ToLower(msg)
	return strings.Contains(msg, "couldn't load match `comment'") ||
		strings.Contains(msg, "couldn't load match 'comment'") ||
		strings.Contains(msg, `couldn't load match "comment"`) ||
		(strings.Contains(msg, "extension comment revision") && strings.Contains(msg, "not supported"))
}
