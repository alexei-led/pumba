package netns

import (
	"errors"
	"fmt"
	"slices"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/tc"
)

// ExecExitError preserves an observed process exit and its exact argv. Message
// retains the runtime's diagnostic; messages alone are never exit evidence.
type ExecExitError struct {
	Code    int
	Argv    []string
	Message string
}

func (e *ExecExitError) Error() string { return e.Message }

// NetemApplyError marks only a proven pre-mutation refusal from this apply
// script. Call it on the execution result before joining helper-removal errors.
// Failed probes, output reads, and mixed/unknown execution results retain rollback.
func NetemApplyError(argv []string, err error) error {
	if onlyNetemRefusal(argv, err) {
		return fmt.Errorf("%w: %w", ctr.ErrNetemNotApplied, err)
	}
	return err
}

func onlyNetemRefusal(argv []string, err error) bool {
	if completed, ok := err.(interface{ ExecutionError() error }); ok {
		return onlyNetemRefusal(argv, completed.ExecutionError())
	}
	// Inspect every joined failure before looking for a process exit: errors.As
	// alone could find a refusal while hiding a separate cleanup/read failure.
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyNetemRefusal(argv, child) {
				return false
			}
		}
		return true
	}
	if inner := errors.Unwrap(err); inner != nil {
		return onlyNetemRefusal(argv, inner)
	}
	var exited *ExecExitError
	return errors.As(err, &exited) && exited.Code == tc.NetemRefusedExitCode && slices.Equal(exited.Argv, argv)
}
