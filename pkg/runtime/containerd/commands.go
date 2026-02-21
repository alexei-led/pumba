package containerd

import (
	"fmt"
	"net"
	"strings"
)

// buildNetemArgs constructs tc netem arguments for applying network emulation.
func buildNetemArgs(netInterface string, netemCmd []string, ips []*net.IPNet, sports, dports []string) []string {
	args := []string{"qdisc", "add", "dev", netInterface, "root", "netem"}
	args = append(args, netemCmd...)
	if len(ips) > 0 || len(sports) > 0 || len(dports) > 0 {
		// When IP/port filtering is needed, use a more complex tc setup
		// For the skeleton, we apply netem directly
		_ = ips
		_ = sports
		_ = dports
	}
	return args
}

// buildStopNetemArgs constructs tc arguments to remove network emulation.
func buildStopNetemArgs(netInterface string) []string {
	return []string{"qdisc", "del", "dev", netInterface, "root"}
}

// buildIPTablesArgs constructs iptables arguments from prefix/suffix and IP/port filters.
func buildIPTablesArgs(cmdPrefix, cmdSuffix []string, srcIPs, dstIPs []*net.IPNet, sports, dports []string) []string {
	var args []string
	args = append(args, cmdPrefix...)

	for _, ip := range srcIPs {
		args = append(args, "-s", ip.String())
	}
	for _, ip := range dstIPs {
		args = append(args, "-d", ip.String())
	}
	if len(sports) > 0 {
		args = append(args, "--sport", strings.Join(sports, ","))
	}
	if len(dports) > 0 {
		args = append(args, fmt.Sprintf("--dport=%s", strings.Join(dports, ",")))
	}

	args = append(args, cmdSuffix...)
	return args
}
