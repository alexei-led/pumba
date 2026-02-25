package containerd

import "net"

// buildNetemCommands constructs tc commands for applying network emulation.
// When IP/port filters are specified, creates a priority-based queueing hierarchy:
//
//	       1:   root qdisc (prio)
//	      / | \
//	    1:1 1:2 1:3    classes
//	     |   |   |
//	   10:  20:  30:   qdiscs
//	   sfq  sfq  netem
//	band 0   1    2
//
// Matching traffic is routed to band 2 (netem), all other traffic flows through sfq.
func buildNetemCommands(netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string) [][]string {
	if len(ips) == 0 && len(sports) == 0 && len(dports) == 0 {
		// Simple case: apply netem directly on root qdisc
		args := make([]string, 0, len(netemCmd)+6) //nolint:mnd
		args = append(args, "qdisc", "add", "dev", netInterface, "root", "netem")
		args = append(args, netemCmd...)
		return [][]string{args}
	}

	// IP/port filter case: prio qdisc + sfq + netem + u32 filters
	netemArgs := make([]string, 0, len(netemCmd)+9) //nolint:mnd
	netemArgs = append(netemArgs, "qdisc", "add", "dev", netInterface, "parent", "1:3", "handle", "30:", "netem")
	netemArgs = append(netemArgs, netemCmd...)

	commands := [][]string{
		{"qdisc", "add", "dev", netInterface, "root", "handle", "1:", "prio"},
		{"qdisc", "add", "dev", netInterface, "parent", "1:1", "handle", "10:", "sfq"},
		{"qdisc", "add", "dev", netInterface, "parent", "1:2", "handle", "20:", "sfq"},
		netemArgs,
	}

	for _, ip := range ips {
		commands = append(commands, []string{
			"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
			"u32", "match", "ip", "dst", ip.String(), "flowid", "1:3",
		})
	}
	for _, sport := range sports {
		commands = append(commands, []string{
			"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
			"u32", "match", "ip", "sport", sport, "0xffff", "flowid", "1:3",
		})
	}
	for _, dport := range dports {
		commands = append(commands, []string{
			"filter", "add", "dev", netInterface, "protocol", "ip", "parent", "1:0", "prio", "1",
			"u32", "match", "ip", "dport", dport, "0xffff", "flowid", "1:3",
		})
	}

	return commands
}

// buildStopNetemCommands constructs tc commands to remove network emulation.
// When filters were used, removes the priority qdisc hierarchy; otherwise just deletes root netem.
func buildStopNetemCommands(netInterface string, hasFilters bool) [][]string {
	if !hasFilters {
		return [][]string{{"qdisc", "del", "dev", netInterface, "root"}}
	}
	return [][]string{
		{"qdisc", "del", "dev", netInterface, "parent", "1:1", "handle", "10:"},
		{"qdisc", "del", "dev", netInterface, "parent", "1:2", "handle", "20:"},
		{"qdisc", "del", "dev", netInterface, "parent", "1:3", "handle", "30:"},
		{"qdisc", "del", "dev", netInterface, "root", "handle", "1:", "prio"},
	}
}

// buildIPTablesCommands constructs one iptables command per IP/port filter,
// matching Docker's behavior of issuing separate rules per filter element.
func buildIPTablesCommands(cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string) [][]string {
	var commands [][]string

	for _, ip := range srcIPs {
		cmd := make([]string, 0, len(cmdPrefix)+len(cmdSuffix)+2) //nolint:mnd
		cmd = append(cmd, cmdPrefix...)
		cmd = append(cmd, "-s", ip.String())
		cmd = append(cmd, cmdSuffix...)
		commands = append(commands, cmd)
	}
	for _, ip := range dstIPs {
		cmd := make([]string, 0, len(cmdPrefix)+len(cmdSuffix)+2) //nolint:mnd
		cmd = append(cmd, cmdPrefix...)
		cmd = append(cmd, "-d", ip.String())
		cmd = append(cmd, cmdSuffix...)
		commands = append(commands, cmd)
	}
	for _, sport := range sports {
		cmd := make([]string, 0, len(cmdPrefix)+len(cmdSuffix)+2) //nolint:mnd
		cmd = append(cmd, cmdPrefix...)
		cmd = append(cmd, "--sport", sport)
		cmd = append(cmd, cmdSuffix...)
		commands = append(commands, cmd)
	}
	for _, dport := range dports {
		cmd := make([]string, 0, len(cmdPrefix)+len(cmdSuffix)+2) //nolint:mnd
		cmd = append(cmd, cmdPrefix...)
		cmd = append(cmd, "--dport", dport)
		cmd = append(cmd, cmdSuffix...)
		commands = append(commands, cmd)
	}

	// No filters: single command with just prefix + suffix
	if len(commands) == 0 {
		cmd := make([]string, 0, len(cmdPrefix)+len(cmdSuffix))
		cmd = append(cmd, cmdPrefix...)
		cmd = append(cmd, cmdSuffix...)
		commands = append(commands, cmd)
	}

	return commands
}
