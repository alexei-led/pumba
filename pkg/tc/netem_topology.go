package tc

import (
	"fmt"
	"strings"
)

func writeScopedTopology(script *strings.Builder, r *NetemRequest) {
	iface := shellQuote(r.Interface)
	writeRootInspection(script, iface, "prio")
	fmt.Fprintf(script, "rollback() { tc qdisc del dev %s root handle %s >/dev/null 2>&1 || true; }\n", iface, PumbaRootHandle)
	writeRollbackCommand(script, fmt.Sprintf("tc qdisc add dev %s parent %s1 handle %s sfq", iface, PumbaRootHandle, firstBandHandle))
	writeRollbackCommand(script, fmt.Sprintf("tc qdisc add dev %s parent %s2 handle %s sfq", iface, PumbaRootHandle, secondBandHandle))
	var netem strings.Builder
	fmt.Fprintf(&netem, "tc qdisc add dev %s parent %s3 handle %s netem", iface, PumbaRootHandle, netemBandHandle)
	writeQuotedArgs(&netem, r.Command)
	writeRollbackCommand(script, netem.String())
}

func writeScopedInspection(script *strings.Builder, iface string) {
	fmt.Fprintf(script, "state=$(tc qdisc show dev %s)\n", iface)
	script.WriteString("if [ -z \"$state\" ] || printf '%s\\n' \"$state\" | grep -Eq '^qdisc (noqueue|fq|fq_codel|pfifo_fast) 0: root'; then\n  exit 0\nfi\n")
	for _, topology := range []string{
		"^qdisc prio 504d: root",
		"^qdisc sfq 504e: parent 504d:1",
		"^qdisc sfq 504f: parent 504d:2",
		"^qdisc netem 5050: parent 504d:3",
	} {
		fmt.Fprintf(script, "printf '%%s\\n' \"$state\" | grep -Eq '%s' || { echo 'refusing to remove unverified Pumba qdisc topology' >&2; exit 1; }\n", topology)
	}
	script.WriteString("children=$(printf '%s\\n' \"$state\" | grep -Ec '^qdisc .* parent 504d:' || true)\n")
	script.WriteString("[ \"$children\" -eq 3 ] || { echo 'refusing to remove unverified Pumba qdisc topology' >&2; exit 1; }\n")
}
