// Package tc plans ownership-safe traffic-control netem operations.
package tc

import (
	"fmt"
	"strings"
)

const (
	// PumbaRootHandle identifies a qdisc tree owned by Pumba. It is deliberately
	// unusual so Pumba never mistakes a user-managed qdisc for its own state.
	PumbaRootHandle = "504d:"

	firstBandHandle  = "504e:"
	secondBandHandle = "504f:"
	netemBandHandle  = "5050:"
)

// NetemRequest contains the tc-specific portion of a netem request.
type NetemRequest struct {
	Interface string
	Command   []string
	IPs       []string
	SPorts    []string
	DPorts    []string
}

// HasFilters reports whether this request needs Pumba's scoped prio topology.
func (r *NetemRequest) HasFilters() bool {
	return len(r.IPs) > 0 || len(r.SPorts) > 0 || len(r.DPorts) > 0
}

// Start returns arguments for `sh -ec` that inspect the root qdisc before any
// mutation and install only Pumba-owned qdiscs. A foreign root fails closed.
func Start(r *NetemRequest) []string {
	if r.HasFilters() {
		return []string{"-ec", scopedStartScript(r)}
	}
	return []string{"-ec", unscopedStartScript(r)}
}

// Stop returns arguments for `sh -ec` that verify Pumba ownership before
// cleanup. An absent/default qdisc has no Pumba state and succeeds.
func Stop(r *NetemRequest) []string {
	if r.HasFilters() {
		return []string{"-ec", scopedStopScript(r)}
	}
	return []string{"-ec", unscopedStopScript(r.Interface)}
}

func scopedStartScript(r *NetemRequest) string {
	iface := shellQuote(r.Interface)
	var script strings.Builder
	writeRootInspection(&script, iface, "prio")
	fmt.Fprintf(&script, "rollback() { tc qdisc del dev %s root handle %s >/dev/null 2>&1 || true; }\n", iface, PumbaRootHandle)
	writeRollbackCommand(&script, fmt.Sprintf("tc qdisc add dev %s parent %s1 handle %s sfq", iface, PumbaRootHandle, firstBandHandle))
	writeRollbackCommand(&script, fmt.Sprintf("tc qdisc add dev %s parent %s2 handle %s sfq", iface, PumbaRootHandle, secondBandHandle))
	var netem strings.Builder
	fmt.Fprintf(&netem, "tc qdisc add dev %s parent %s3 handle %s netem", iface, PumbaRootHandle, netemBandHandle)
	writeQuotedArgs(&netem, r.Command)
	writeRollbackCommand(&script, netem.String())
	filterIndex := 0
	for _, ip := range r.IPs {
		writeRollbackCommand(&script, fmt.Sprintf("tc filter add dev %s protocol ip parent %s handle %s prio 1 u32 match ip dst %s flowid %s3", iface, PumbaRootHandle, filterHandle(filterIndex), shellQuote(ip), PumbaRootHandle))
		filterIndex++
	}
	for _, sport := range r.SPorts {
		writeRollbackCommand(&script, fmt.Sprintf("tc filter add dev %s protocol ip parent %s handle %s prio 1 u32 match ip sport %s 0xffff flowid %s3", iface, PumbaRootHandle, filterHandle(filterIndex), shellQuote(sport), PumbaRootHandle))
		filterIndex++
	}
	for _, dport := range r.DPorts {
		writeRollbackCommand(&script, fmt.Sprintf("tc filter add dev %s protocol ip parent %s handle %s prio 1 u32 match ip dport %s 0xffff flowid %s3", iface, PumbaRootHandle, filterHandle(filterIndex), shellQuote(dport), PumbaRootHandle))
		filterIndex++
	}
	return script.String()
}

func unscopedStartScript(r *NetemRequest) string {
	iface := shellQuote(r.Interface)
	var script strings.Builder
	writeRootInspection(&script, iface, "netem", r.Command)
	return script.String()
}

func writeRootInspection(script *strings.Builder, iface, kind string, command ...[]string) {
	fmt.Fprintf(script, "state=$(tc qdisc show dev %s)\n", iface)
	script.WriteString("if printf '%s\\n' \"$state\" | grep -Eq '^qdisc (noqueue|fq_codel|pfifo_fast) 0: root'; then\n")
	if kind == "prio" {
		fmt.Fprintf(script, "  tc qdisc add dev %s root handle %s prio bands 3 priomap 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1\n", iface, PumbaRootHandle)
	} else {
		fmt.Fprintf(script, "  tc qdisc add dev %s root handle %s %s", iface, PumbaRootHandle, kind)
		if len(command) > 0 {
			writeQuotedArgs(script, command[0])
		}
		script.WriteString("\n")
	}
	script.WriteString("else\n  echo 'refusing to replace a foreign or stale root qdisc' >&2\n  exit 1\nfi\n")
}

func scopedStopScript(r *NetemRequest) string {
	iface := shellQuote(r.Interface)
	expectedFilters := len(r.IPs) + len(r.SPorts) + len(r.DPorts)
	var script strings.Builder
	fmt.Fprintf(&script, "state=$(tc qdisc show dev %s)\n", iface)
	script.WriteString("if [ -z \"$state\" ] || printf '%s\\n' \"$state\" | grep -Eq '^qdisc (noqueue|fq_codel|pfifo_fast) 0: root'; then\n  exit 0\nfi\n")
	for _, topology := range []string{
		"^qdisc prio 504d: root",
		"^qdisc sfq 504e: parent 504d:1",
		"^qdisc sfq 504f: parent 504d:2",
		"^qdisc netem 5050: parent 504d:3",
	} {
		fmt.Fprintf(&script, "printf '%%s\\n' \"$state\" | grep -Eq '%s' || { echo 'refusing to remove unverified Pumba qdisc topology' >&2; exit 1; }\n", topology)
	}
	script.WriteString("children=$(printf '%s\\n' \"$state\" | grep -Ec '^qdisc .* parent 504d:' || true)\n")
	script.WriteString("[ \"$children\" -eq 3 ] || { echo 'refusing to remove unverified Pumba qdisc topology' >&2; exit 1; }\n")
	fmt.Fprintf(&script, "filters=$(tc filter show dev %s parent %s)\n", iface, PumbaRootHandle)
	script.WriteString("filter_count=$(printf '%s\\n' \"$filters\" | grep -c 'flowid 504d:3' || true)\n")
	fmt.Fprintf(&script, "[ \"$filter_count\" -eq %d ] || { echo 'refusing to remove unverified Pumba filters' >&2; exit 1; }\n", expectedFilters)
	for i := 0; i < expectedFilters; i++ {
		fmt.Fprintf(&script, "printf '%%s\\n' \"$filters\" | grep -Eq '(^| )fh %s( |$)' || { echo 'refusing to remove unverified Pumba filters' >&2; exit 1; }\n", filterHandle(i))
	}
	// Deleting the verified root removes the complete Pumba-owned hierarchy
	// atomically: the filters and child qdiscs are never removed individually.
	fmt.Fprintf(&script, "tc qdisc del dev %s root handle %s\n", iface, PumbaRootHandle)
	return script.String()
}

func filterHandle(index int) string {
	return fmt.Sprintf("504d%x:", index+1)
}

func unscopedStopScript(netInterface string) string {
	iface := shellQuote(netInterface)
	return fmt.Sprintf(`state=$(tc qdisc show dev %s)
if [ -z "$state" ] || printf '%%s\n' "$state" | grep -Eq '^qdisc (noqueue|fq_codel|pfifo_fast) 0: root'; then
  exit 0
fi
printf '%%s\n' "$state" | grep -Eq '^qdisc netem 504d: root' || { echo 'refusing to remove a foreign root qdisc' >&2; exit 1; }
tc qdisc del dev %s root handle 504d:
`, iface, iface)
}

func writeRollbackCommand(script *strings.Builder, command string) {
	script.WriteString(command)
	script.WriteString(" || { rollback; exit 1; }\n")
}

func writeQuotedArgs(script *strings.Builder, args []string) {
	for _, arg := range args {
		script.WriteByte(' ')
		script.WriteString(shellQuote(arg))
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
