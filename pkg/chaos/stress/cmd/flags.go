package cmd

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/urfave/cli"
)

const (
	minWorkers   = 1
	maxPercent   = 100
	sizeUnitBits = 10
)

var memorySizePattern = regexp.MustCompile(`^[1-9]\d*[bBkKmMgG]?$`)

func resourceFlags() []cli.Flag {
	return []cli.Flag{
		cli.IntFlag{Name: "cpu", Usage: "CPU workers (at least 1)"},
		cli.IntFlag{Name: "cpu-load", Usage: "CPU load percent (0-100); requires --cpu"},
		cli.IntFlag{Name: "memory", Usage: "memory workers (at least 1)"},
		cli.StringFlag{Name: "memory-bytes", Usage: "total memory shared by workers: bytes, b/k/m/g suffix, or 1-100%; requires --memory"},
		cli.IntFlag{Name: "io", Usage: "I/O workers calling sync (at least 1)"},
	}
}

type resourceOptions interface {
	cliflags.Flags
	IsSet(string) bool
}

func parseStressors(flags cliflags.Flags, duration time.Duration) (string, error) {
	c, ok := flags.(resourceOptions)
	if !ok {
		return "", errors.New("stress parser requires explicit flag tracking")
	}
	firstClass := false
	for _, name := range []string{"cpu", "cpu-load", "memory", "memory-bytes", "io"} {
		firstClass = firstClass || c.IsSet(name)
	}
	if !firstClass {
		return c.String("stressors"), nil
	}
	if c.IsSet("stressors") {
		return "", errors.New("cannot combine --stressors with first-class stress flags")
	}
	if err := validateResourceFlags(c); err != nil {
		return "", err
	}
	var args []string
	for _, worker := range []struct{ flag, stressor string }{{"cpu", "cpu"}, {"memory", "vm"}, {"io", "io"}} {
		if c.IsSet(worker.flag) {
			args = append(args, "--"+worker.stressor, strconv.Itoa(c.Int(worker.flag)))
			switch worker.flag {
			case "cpu":
				if c.IsSet("cpu-load") {
					args = append(args, "--cpu-load", strconv.Itoa(c.Int("cpu-load")))
				}
			case "memory":
				if c.IsSet("memory-bytes") {
					args = append(args, "--vm-bytes", c.String("memory-bytes"))
				}
			}
		}
	}
	// stress-ng uses whole seconds. Pumba's own duration timer keeps the exact
	// deadline; round up so subsecond durations never become an unlimited timeout.
	args = append(args, "--timeout", strconv.FormatInt(int64(math.Ceil(duration.Seconds())), 10)+"s")
	return strings.Join(args, " "), nil
}

func validateResourceFlags(c resourceOptions) error {
	for _, name := range []string{"cpu", "memory", "io"} {
		if c.IsSet(name) && c.Int(name) < minWorkers {
			return fmt.Errorf("--%s must be at least %d", name, minWorkers)
		}
	}
	if c.IsSet("cpu-load") {
		if !c.IsSet("cpu") {
			return errors.New("--cpu-load requires --cpu")
		}
		if load := c.Int("cpu-load"); load < 0 || load > maxPercent {
			return fmt.Errorf("--cpu-load must be between 0 and %d", maxPercent)
		}
	}
	if c.IsSet("memory-bytes") {
		if !c.IsSet("memory") {
			return errors.New("--memory-bytes requires --memory")
		}
		return validateMemoryBytes(c.String("memory-bytes"))
	}
	return nil
}

func validateMemoryBytes(value string) error {
	if percent, ok := strings.CutSuffix(value, "%"); ok {
		n, err := strconv.Atoi(percent)
		if err != nil || n < 1 || n > maxPercent {
			return fmt.Errorf("--memory-bytes percent must be between 1 and %d", maxPercent)
		}
		return nil
	}
	if !memorySizePattern.MatchString(value) {
		return errors.New("invalid --memory-bytes: use positive bytes, b/k/m/g suffix, or 1-100%")
	}
	n, err := strconv.ParseUint(strings.TrimRight(value, "bBkKmMgG"), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid --memory-bytes: %w", err)
	}
	// stress-ng scales suffixes without checking overflow; don't let an oversized
	// request wrap into a smaller, valid workload.
	multiplier := uint64(1)
	if unit := strings.IndexByte("bkmg", strings.ToLower(value)[len(value)-1]); unit >= 0 {
		multiplier <<= sizeUnitBits * unit
	}
	if n > math.MaxUint64/multiplier {
		return errors.New("invalid --memory-bytes: size overflows uint64")
	}
	return nil
}
