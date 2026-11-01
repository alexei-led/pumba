package container

// StressCleanupError identifies failure to reap a stress workload, distinct
// from the workload's own exit error. Err retains the original cleanup cause.
type StressCleanupError struct {
	Err error
}

func (e *StressCleanupError) Error() string { return "stress cleanup failed: " + e.Err.Error() }
func (e *StressCleanupError) Unwrap() error { return e.Err }
