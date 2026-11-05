package tc

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

func matchAllNetemScripts(r *NetemRequest) (start, stop string, err error) {
	groups, err := compileSelectors(
		selectorDimension{"dst", r.IPs, true},
		selectorDimension{"sport", r.SPorts, false},
		selectorDimension{"dport", r.DPorts, false},
	)
	if err != nil {
		return "", "", err
	}
	if !r.HasFilters() {
		start, stop = unscopedStartScript(r), unscopedStopScript(r.Interface)
	} else {
		start, stop = matchAllStartScript(r, groups), matchAllStopScript(r, groups)
	}
	if len(start) > maxNetemScriptBytes || len(stop) > maxNetemScriptBytes {
		return "", "", fmt.Errorf("%w: scripts must not exceed %d bytes", ErrFilterLimit, maxNetemScriptBytes)
	}
	return start, stop, nil
}

func matchAllStartScript(r *NetemRequest, groups [][]selector) string {
	var script strings.Builder
	// Verify the cleanup verifier exists before installing anything.
	fmt.Fprintf(&script, "command -v awk >/dev/null 2>&1 || { echo 'match-all requires awk' >&2; exit %d; }\n", NetemRefusedExitCode)
	writeScopedTopology(&script, r)
	iface := shellQuote(r.Interface)
	for _, group := range groups {
		var command strings.Builder
		fmt.Fprintf(&command, "tc filter add dev %s protocol ip parent %s prio 1 u32", iface, PumbaRootHandle)
		for _, match := range group {
			fmt.Fprintf(&command, " match ip %s %s", match.flag, shellQuote(match.value))
			if match.flag != "dst" {
				command.WriteString(" 0xffff")
			}
		}
		fmt.Fprintf(&command, " flowid %s3", PumbaRootHandle)
		writeRollbackCommand(&script, command.String())
	}
	return script.String()
}

func matchAllStopScript(r *NetemRequest, groups [][]selector) string {
	var script strings.Builder
	iface := shellQuote(r.Interface)
	writeScopedInspection(&script, iface)
	fmt.Fprintf(&script, "filters=$(tc filter show dev %s parent %s)\n", iface, PumbaRootHandle)
	expected := make([]string, len(groups))
	for i, group := range groups {
		expected[i] = groupKey(group)
	}
	fmt.Fprintf(&script, "printf '%%s\\n' \"$filters\" | awk -v wanted=%s %s || { echo 'refusing to remove unverified Pumba filter groups' >&2; exit 1; }\n",
		shellQuote(strings.Join(expected, "\n")), shellQuote(groupVerifier))
	fmt.Fprintf(&script, "tc qdisc del dev %s root handle %s\n", iface, PumbaRootHandle)
	return script.String()
}

func groupKey(group []selector) string {
	ip, sport, dport := "-", "-", "-"
	for _, match := range group {
		if match.flag == "dst" {
			prefix := netip.MustParsePrefix(match.value)
			addr := prefix.Masked().Addr().As4()
			value := uint32(addr[0])<<24 | uint32(addr[1])<<16 | uint32(addr[2])<<8 | uint32(addr[3])
			mask := ^uint32(0) << (ipv4Bits - prefix.Bits())
			ip = fmt.Sprintf("%08x/%08x", value, mask)
			continue
		}
		port, _ := strconv.ParseUint(match.value, 10, portBits) // compileSelectors validated it.
		if match.flag == "sport" {
			sport = fmt.Sprintf("%04x", port)
		} else {
			dport = fmt.Sprintf("%04x", port)
		}
	}
	return ip + "|" + sport + "|" + dport
}

// tc combines non-overlapping sport/dport keys at offset 20. Accept either
// representation, but compare complete groups so swapped conjunctions fail.
const groupVerifier = `BEGIN {
    n = split(wanted, rows, "\n")
    for (i=1; i<=n; i++) remaining[rows[i]]++
}
function finish( key) {
    if (!active) return
    key = ip "|" sport "|" dport
    if (!keys || !(key in remaining) || --remaining[key] < 0) bad=1
    seen++
    active=0
}
$1 == "filter" {
    finish()
    protocol=0; pref=0; classifier=0; flow=""
    for (i=2; i<=NF; i++) {
        if ($i=="protocol") { if ($(i+1)!="ip") bad=1; protocol++ }
        if ($i=="pref") { if ($(i+1)!="1") bad=1; pref++ }
        if ($i=="parent" && $(i+1)!="504d:") bad=1
        if ($i=="chain" && $(i+1)!="0") bad=1
        if ($i=="u32") classifier++
        if ($i=="flowid") { if (flow!="") bad=1; flow=$(i+1) }
    }
    if (protocol!=1 || pref!=1 || classifier!=1) bad=1
    if (flow=="") {
        handle=""; divisor=""
        for (i=2; i<=NF; i++) {
            if ($i=="fh") handle=$(i+1)
            if ($i=="divisor") divisor=$(i+1)
        }
        if (handle=="") { if (++headers>1) bad=1 }
        else if (handle!="800:" || divisor!="1" || ++tables>1) bad=1
        next
    }
    if (flow!="504d:3") bad=1
    active=1; keys=0; ip="-"; sport="-"; dport="-"
    next
}
$1 == "match" {
    if (!active || NF!=4 || $3!="at") { bad=1; next }
    if ($2 !~ /^[0-9a-f]+\/[0-9a-f]+$/ || length($2)!=17 || substr($2,9,1)!="/") { bad=1; next }
    value=substr($2,1,8); mask=substr($2,10,8)
    if ($4=="16" && ip=="-") ip=$2
    else if ($4=="20" && mask=="ffff0000" && sport=="-") sport=substr(value,1,4)
    else if ($4=="20" && mask=="0000ffff" && dport=="-") dport=substr(value,5,4)
    else if ($4=="20" && mask=="ffffffff" && sport=="-" && dport=="-") {
        sport=substr(value,1,4); dport=substr(value,5,4)
    } else bad=1
    keys++
    next
}
NF { bad=1 }
END {
    finish()
    for (key in remaining) if (remaining[key]!=0) bad=1
    if (seen!=n || bad) exit 1
}`
