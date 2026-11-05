package tc

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Recovery accepts only the fixed grammar emitted for Pumba IPv4 loss rules.
// A listing parser establishes visibility, not permission to execute its text.
var recoveryValue = regexp.MustCompile(`^[A-Za-z0-9_./:+-]+$`)
var recoveryRunID = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,48}$`)

// RecoveryDeleteCommand validates a listed rule and builds an exact, filter-table
// delete argv. No shell is involved. Ambiguous ownership and unknown options
// fail closed; duplicate rule occurrences must be retained by the caller.
func RecoveryDeleteCommand(backend, runID, chain, spec string) ([]string, error) {
	if backend != "iptables-nft" && backend != "iptables-legacy" {
		return nil, fmt.Errorf("unknown recovery backend %q", backend)
	}
	if !recoveryRunID.MatchString(runID) {
		return nil, errors.New("invalid recovery run ID")
	}
	args, err := splitArgs(spec)
	if err != nil {
		return nil, err
	}
	const header = 2
	if len(args) < header || args[0] != "-A" || args[1] != chain || (chain != "INPUT" && chain != "OUTPUT" && chain != "FORWARD") {
		return nil, errors.New("invalid recovery rule header")
	}
	if err := validateRecoveryArgs(args[header:], runID); err != nil {
		return nil, err
	}
	return append([]string{backend, "-t", "filter", "-D", chain}, args[header:]...), nil
}

func validateRecoveryArgs(args []string, runID string) error {
	seen := map[string]bool{}
	state := recoveryGrammar{runID: runID}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return errors.New("incomplete recovery rule")
		}
		key, value := args[i], args[i+1]
		if !recoveryValue.MatchString(value) || strings.HasPrefix(value, "-") {
			return errors.New("invalid recovery value")
		}
		if key != "-m" && seen[key] {
			return fmt.Errorf("duplicate recovery option %s", key)
		}
		seen[key] = true
		if err := state.option(key, value); err != nil {
			return err
		}
	}
	if !state.comment || !state.drop {
		return errors.New("recovery requires one ownership comment and DROP target")
	}
	return nil
}

type recoveryGrammar struct {
	runID                        string
	comment, commentModule, drop bool
}

func (s *recoveryGrammar) option(key, value string) error {
	switch key {
	case "-s", "-d", "-i", "-o", "-p", "--sport", "--dport", "--probability", "--every", "--packet":
	case "--mode":
		if value != "random" && value != "nth" {
			return errors.New("unsupported statistic mode")
		}
	case "-m":
		switch value {
		case "comment":
			if s.commentModule {
				return errors.New("multiple ownership modules")
			}
			s.commentModule = true
		case "tcp", "udp", "icmp", "statistic":
		default:
			return fmt.Errorf("unsupported recovery match %q", value)
		}
	case "--comment":
		if value != CommentPrefix+s.runID || !s.commentModule {
			return errors.New("ambiguous recovery ownership")
		}
		s.comment = true
	case "-j":
		if value != "DROP" {
			return errors.New("unsupported recovery target")
		}
		s.drop = true
	default:
		return fmt.Errorf("unsupported recovery option %q", key)
	}
	return nil
}
