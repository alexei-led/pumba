package containerd

import "net"

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
