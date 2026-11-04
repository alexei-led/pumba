package netns

import "errors"

// JoinExecCleanup preserves the execution result separately from bookkeeping
// failure after an observed exit. Callers must not use it for unknown or
// contradictory exit evidence or termination failures. Output-read errors
// belong in execution, never cleanup, so they still prevent refusal proof.
func JoinExecCleanup(execution, cleanup error) error {
	if cleanup == nil {
		return execution
	}
	return &completedExecError{execution: execution, cleanup: cleanup}
}

type completedExecError struct {
	execution error
	cleanup   error
}

func (e *completedExecError) Error() string {
	return errors.Join(e.execution, e.cleanup).Error()
}

func (e *completedExecError) Unwrap() []error {
	return []error{e.execution, e.cleanup}
}

// ExecutionError exposes only the result of execution, including read errors.
func (e *completedExecError) ExecutionError() error { return e.execution }
