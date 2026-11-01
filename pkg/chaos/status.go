package chaos

// Verdict is the answer of `pumba status --check`.
type Verdict string

// Status verdicts. With --check they are also the run.end status.
const (
	// VerdictClean: every selected target was inspected, coverage has no
	// gaps, and no Pumba artifact matched.
	VerdictClean Verdict = "clean"
	// VerdictDirty: an attributable Pumba artifact matched.
	VerdictDirty Verdict = "dirty"
	// VerdictUnknown: the state could not be fully established.
	VerdictUnknown Verdict = "unknown"
)

// TargetState is the Pumba fault state `pumba status` found on one target,
// carried by its target.result. Rules and sidecars are filtered by the status
// run filter; qdiscs carry no run and are never filtered.
type TargetState struct {
	Rules    []StateRule    `json:"rules"`
	Qdiscs   []StateQdisc   `json:"qdiscs"`
	Sidecars []StateSidecar `json:"sidecars"`
}

// StateRule is an iptables rule tagged with a pumba:<run-id> comment.
type StateRule struct {
	RunID string `json:"run_id"`
	Chain string `json:"chain"`
	Spec  string `json:"spec"`
}

// StateQdisc is a qdisc tree under Pumba's root handle. RunID is always nil:
// qdiscs are not attributable to a run.
type StateQdisc struct {
	Device    string  `json:"device"`
	Kind      string  `json:"kind"`
	Ownership string  `json:"ownership"`
	RunID     *string `json:"run_id"`
}

// StateSidecar is a container Pumba created for a fault. RunID is nil for an
// untagged sidecar.
type StateSidecar struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	State    string  `json:"state"`
	TargetID string  `json:"target_id"`
	RunID    *string `json:"run_id"`
}

// StatusCoverage is the part of a status run that belongs to no inspected
// target, carried by run.end.
type StatusCoverage struct {
	Verdict Verdict `json:"verdict"`
	// Reasons explain a dirty or unknown verdict.
	Reasons   []string `json:"reasons"`
	RunFilter string   `json:"run_filter,omitempty"`
	// OrphanSidecars are sidecars whose target was not inspected; reported
	// only when no selection was given.
	OrphanSidecars []StateSidecar `json:"orphan_sidecars"`
	// MissingTargets are named targets that do not exist or are not running.
	MissingTargets []string `json:"missing_targets"`
	// ListingGaps are containers the runtime could not read.
	ListingGaps []ListingGap `json:"listing_gaps"`
}

// ListingGap is a container the runtime left out of a listing.
type ListingGap struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// StatusTarget is one inspected target of a status report.
type StatusTarget struct {
	Target TargetRef
	// Error is set when inspection failed; State then holds only sidecars.
	Error string
	State *TargetState
}

// StatusReport is what one status tick found. Check reports whether the
// verdict decides the run outcome (--check).
type StatusReport struct {
	Check    bool
	Targets  []StatusTarget
	Coverage *StatusCoverage
}

// StatusRecorder is implemented by event sinks that take status reports: the
// Recorder (run.end coverage and verdict) and cmd's text renderer.
type StatusRecorder interface {
	RecordStatus(rep *StatusReport)
}

// RecordStatus hands rep to gp.Events when it takes status reports.
func (gp *GlobalParams) RecordStatus(rep *StatusReport) {
	if r, ok := gp.Events.(StatusRecorder); ok {
		r.RecordStatus(rep)
	}
}

// NewInspectError marks err as a failed inspection: the target.result error
// reports phase inspect, code command_failed. The message is unchanged.
func NewInspectError(err error) error {
	if err == nil {
		return nil
	}
	return &codedError{code: CodeCommandFailed, phase: PhaseInspect, err: err}
}
