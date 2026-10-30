package chaos

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
)

// runIDBytes yields a 12-character hex run ID: short enough for labels and
// iptables comments, random enough to keep concurrent runs apart.
const runIDBytes = 6

// maxRunIDLen keeps a user-supplied run ID well inside the 256-byte iptables
// comment limit and the 63-character label value limit.
const maxRunIDLen = 48

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// NewRunID returns a short random hex run ID.
func NewRunID() string {
	b := make([]byte, runIDBytes)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b)
}

// ValidateRunID rejects run IDs that are not safe in labels and iptables
// comments.
func ValidateRunID(id string) error {
	if len(id) > maxRunIDLen || !runIDPattern.MatchString(id) {
		return fmt.Errorf("invalid --run-id %q: use 1-%d characters from [A-Za-z0-9_.-]", id, maxRunIDLen)
	}
	return nil
}

// ResolveRunID returns id after validating it, or a new run ID when id is
// empty.
func ResolveRunID(id string) (string, error) {
	if id == "" {
		return NewRunID(), nil
	}
	if err := ValidateRunID(id); err != nil {
		return "", err
	}
	return id, nil
}
