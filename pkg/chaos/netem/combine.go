package netem

import (
	"context"
	"errors"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

const minimumCombinedEffects = 2

// DelayEffect configures delay within a combined netem qdisc.
type DelayEffect struct {
	Time         int
	Jitter       int
	Correlation  float64
	Distribution string
}

// LossEffect configures random packet loss within a combined netem qdisc.
type LossEffect struct {
	Percent     float64
	Correlation float64
}

// CorruptEffect configures packet corruption within a combined netem qdisc.
type CorruptEffect struct {
	Percent     float64
	Correlation float64
}

// DuplicateEffect configures packet duplication within a combined netem qdisc.
type DuplicateEffect struct {
	Percent     float64
	Correlation float64
}

// RateEffect configures rate limiting within a combined netem qdisc.
type RateEffect struct {
	Rate           string
	PacketOverhead int
	CellSize       int
	CellOverhead   int
}

// CombinedSpec selects effects for one netem qdisc. At least two effects must
// be enabled; use the existing single-effect commands otherwise.
type CombinedSpec struct {
	Delay     *DelayEffect
	Loss      *LossEffect
	Corrupt   *CorruptEffect
	Duplicate *DuplicateEffect
	Rate      *RateEffect
}

func (s CombinedSpec) validate() error {
	if s.effectCount() < minimumCombinedEffects {
		return errors.New("at least two netem effects are required; use a single-effect command for one")
	}
	if s.Delay != nil {
		if err := validateDelay(s.Delay.Time, s.Delay.Jitter, s.Delay.Correlation, s.Delay.Distribution); err != nil {
			return err
		}
	}
	if s.Loss != nil {
		if err := validateLoss(s.Loss.Percent, s.Loss.Correlation); err != nil {
			return err
		}
	}
	if s.Duplicate != nil {
		if err := validateDuplicate(s.Duplicate.Percent, s.Duplicate.Correlation); err != nil {
			return err
		}
	}
	if s.Corrupt != nil {
		if err := validateCorrupt(s.Corrupt.Percent, s.Corrupt.Correlation); err != nil {
			return err
		}
	}
	if s.Rate != nil {
		if err := validateRateParams(s.Rate.Rate, s.Rate.CellSize, s.Rate.CellOverhead); err != nil {
			return err
		}
	}
	return nil
}

func (s CombinedSpec) effectCount() int {
	count := 0
	for _, enabled := range []bool{
		s.Delay != nil,
		s.Loss != nil,
		s.Corrupt != nil,
		s.Duplicate != nil,
		s.Rate != nil,
	} {
		if enabled {
			count++
		}
	}
	return count
}

func (s CombinedSpec) args() []string {
	var args []string
	if s.Delay != nil {
		args = append(args, delayArgs(s.Delay.Time, s.Delay.Jitter, s.Delay.Correlation, s.Delay.Distribution)...)
	}
	if s.Loss != nil {
		args = append(args, lossArgs(s.Loss.Percent, s.Loss.Correlation)...)
	}
	if s.Duplicate != nil {
		args = append(args, duplicateArgs(s.Duplicate.Percent, s.Duplicate.Correlation)...)
	}
	if s.Corrupt != nil {
		args = append(args, corruptArgs(s.Corrupt.Percent, s.Corrupt.Correlation)...)
	}
	if s.Rate != nil {
		args = append(args, rateArgs(s.Rate.Rate, s.Rate.PacketOverhead, s.Rate.CellSize, s.Rate.CellOverhead)...)
	}
	return args
}

type combinedCommand struct {
	client netemClient
	gp     *chaos.GlobalParams
	req    *container.NetemRequest
	limit  int
	spec   CombinedSpec
}

// NewCombinedCommand creates one command that applies multiple effects in one
// owned netem qdisc.
func NewCombinedCommand(client netemClient, gp *chaos.GlobalParams, req *container.NetemRequest, limit int, spec CombinedSpec) (chaos.Command, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	return &combinedCommand{client: client, gp: gp, req: req, limit: limit, spec: spec}, nil
}

func (n *combinedCommand) Run(ctx context.Context, random bool) error {
	resolvedReq, err := resolveRequestTargets(ctx, n.client, n.req)
	if err != nil {
		return fmt.Errorf("failed to resolve --target: %w", err)
	}
	command := n.spec.args()
	log.WithFields(log.Fields{
		"names": n.gp.Names, "pattern": n.gp.Pattern, "labels": n.gp.Labels,
		"limit": n.limit, "random": random, "command": command,
	}).Debug("adding combined network effects to matching containers")
	return chaos.RunOnContainers(ctx, n.client, n.gp, n.limit, random, true,
		func(ctx context.Context, target *container.Container) error {
			netemCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			req := *resolvedReq
			req.Container = target
			req.Command = command
			if err := runNetem(netemCtx, n.client, &req); err != nil {
				return fmt.Errorf("failed to apply combined network effects: %w", err)
			}
			return nil
		})
}
