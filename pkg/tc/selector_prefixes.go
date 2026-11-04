package tc

import "net/netip"

// Covered CIDRs must not install extra probabilistic rules for the same packet.
func withoutCoveredPrefixes(values []string) []string {
	prefixes := make(map[netip.Prefix]bool, len(values))
	for _, value := range values {
		prefixes[netip.MustParsePrefix(value)] = true
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		prefix := netip.MustParsePrefix(value)
		covered := false
		for bits := prefix.Bits() - 1; bits >= 0; bits-- {
			if prefixes[netip.PrefixFrom(prefix.Addr(), bits).Masked()] {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, value)
		}
	}
	return result
}
