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
		// 'replace' (not 'add') so this is idempotent if a previous run's teardown
		// left this same root qdisc in place - see buildStopNetemCommands.
		{"qdisc", "replace", "dev", netInterface, "root", "handle", "1:", "prio"},
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
// When filters were used, removes the classifier and per-band qdiscs that made the
// disruption IP/port-scoped; otherwise just deletes root netem.
//
// The root 'prio' qdisc is deliberately left in place in the filtered case: deleting
// or replacing a ROOT qdisc routes through the kernel's qdisc_graft() parent==NULL
// path (net/sched/sch_api.c), which wraps the swap in dev_deactivate()/dev_activate().
// dev_deactivate() installs noop_qdisc on every tx queue of the device for the
// duration of the swap, and noop_qdisc drops every packet handed to it - not just
// packets that matched our u32 filter - because by that point the whole
// prio+filter+netem tree that implemented the IP scoping has already been unlinked.
// That is the actual cause of "all IPs lose traffic during teardown": deleting the
// 'parent 1:1/1:2/1:3' child qdiscs above does NOT have this effect, because those
// go through prio's classful cops->graft() (sch_prio.c), which atomically substitutes
// a default pfifo qdisc under sch_tree_lock and never touches dev_queue->qdisc.
// Deleting the filter first (before any qdisc teardown) also means matched traffic
// falls back to the loss-free default bands 1:1/1:2 immediately, before anything else
// happens. buildNetemCommands uses 'qdisc replace' for the root so a later netem run
// on the same container remains idempotent against this intentionally-left-behind
// qdisc.
func buildStopNetemCommands(netInterface string, hasFilters bool) [][]string {
	if !hasFilters {
		return [][]string{{"qdisc", "del", "dev", netInterface, "root"}}
	}
	return [][]string{
		// A filter delete without a specific handle removes every filter registered
		// under that parent, which matches every filter buildNetemCommands ever adds.
		{"filter", "del", "dev", netInterface, "parent", "1:0"},
		{"qdisc", "del", "dev", netInterface, "parent", "1:1", "handle", "10:"},
		{"qdisc", "del", "dev", netInterface, "parent", "1:2", "handle", "20:"},
		{"qdisc", "del", "dev", netInterface, "parent", "1:3", "handle", "30:"},
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
