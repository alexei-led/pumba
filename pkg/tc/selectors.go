package tc

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
)

// MaxFilterCombinations bounds the Cartesian expansion of --match-all.
const MaxFilterCombinations = 256

const maxNetemScriptBytes = 64 * 1024

// ErrFilterLimit marks a match-all plan that cannot be installed and cleaned up safely.
var ErrFilterLimit = errors.New("match-all plan exceeds its safety limit")

type selectorDimension struct {
	flag    string
	values  []string
	address bool
}

type selector struct {
	flag, value string
}

func compileSelectors(dimensions ...selectorDimension) ([][]selector, error) {
	normalized := make([][]selector, 0, len(dimensions))
	count := 1
	for _, dimension := range dimensions {
		values, err := normalizeSelectors(dimension)
		if err != nil {
			return nil, err
		}
		if len(values) == 0 {
			continue
		}
		if count > MaxFilterCombinations/len(values) {
			return nil, fmt.Errorf("%w: at most %d combinations", ErrFilterLimit, MaxFilterCombinations)
		}
		count *= len(values)
		normalized = append(normalized, values)
	}
	groups := [][]selector{{}}
	for _, values := range normalized {
		expanded := make([][]selector, 0, len(groups)*len(values))
		for _, group := range groups {
			for _, value := range values {
				next := make([]selector, len(group), len(group)+1)
				copy(next, group)
				expanded = append(expanded, append(next, value))
			}
		}
		groups = expanded
	}
	return groups, nil
}

func normalizeSelectors(d selectorDimension) ([]selector, error) {
	values := make([]string, 0, len(d.values))
	seen := make(map[string]bool, len(d.values))
	for _, raw := range d.values {
		value, err := normalizeSelector(raw, d.address)
		if err != nil {
			return nil, fmt.Errorf("invalid %s selector %q: %w", d.flag, raw, err)
		}
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	if d.address {
		values = withoutCoveredPrefixes(values)
	}
	slices.Sort(values)
	result := make([]selector, len(values))
	for i, value := range values {
		result[i] = selector{flag: d.flag, value: value}
	}
	return result, nil
}

func normalizeSelector(raw string, address bool) (string, error) {
	if address {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return "", err
		}
		if !prefix.Addr().Is4() {
			return "", errors.New("only IPv4 selectors are supported")
		}
		return prefix.Masked().String(), nil
	}
	port, err := strconv.ParseUint(raw, 10, portBits)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(port, 10), nil
}
