package containerd

import (
	"fmt"
	"net"
)

// buildNetemArgs constructs tc netem arguments for applying network emulation.
// Returns an error if IP/port filtering is requested, as this requires a complex
// tc setup with priority qdiscs and u32 filters that is not yet implemented.
func buildNetemArgs(netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string) ([]string, error) {
	if len(ips) > 0 || len(sports) > 0 || len(dports) > 0 {
		return nil, fmt.Errorf("containerd runtime: IP/port filtering for netem is not yet implemented")
	}
	args := []string{"qdisc", "add", "dev", netInterface, "root", "netem"}
	args = append(args, netemCmd...)
	return args, nil
}

// buildStopNetemArgs constructs tc arguments to remove network emulation.
func buildStopNetemArgs(netInterface string) []string {
	return []string{"qdisc", "del", "dev", netInterface, "root"}
}

// buildIPTablesCommands constructs one iptables command per IP/port filter,
// matching Docker's behavior of issuing separate rules per filter element.
func buildIPTablesCommands(cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string) [][]string {
	var commands [][]string

	for _, ip := range srcIPs {
		cmd := append(append([]string{}, cmdPrefix...), "-s", ip.String())
		cmd = append(cmd, cmdSuffix...)
		commands = append(commands, cmd)
	}
	for _, ip := range dstIPs {
		cmd := append(append([]string{}, cmdPrefix...), "-d", ip.String())
		cmd = append(cmd, cmdSuffix...)
		commands = append(commands, cmd)
	}
	for _, sport := range sports {
		cmd := append(append([]string{}, cmdPrefix...), "--sport", sport)
		cmd = append(cmd, cmdSuffix...)
		commands = append(commands, cmd)
	}
	for _, dport := range dports {
		cmd := append(append([]string{}, cmdPrefix...), "--dport", dport)
		cmd = append(cmd, cmdSuffix...)
		commands = append(commands, cmd)
	}

	// No filters: single command with just prefix + suffix
	if len(commands) == 0 {
		cmd := append(append([]string{}, cmdPrefix...), cmdSuffix...)
		commands = append(commands, cmd)
	}

	return commands
}
