package container

import "errors"

// ErrNetemNotApplied means netem startup positively refused before changing
// the namespace. Callers must not roll back another action's existing fault.
// Unknown execution or cleanup failures must not carry this sentinel.
var ErrNetemNotApplied = errors.New("netem refused before changing the namespace")
