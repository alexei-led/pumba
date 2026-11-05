package container

import "errors"

// ErrNetemNotApplied means netem startup positively refused before changing
// the namespace. Callers must not roll back another action's existing fault.
// Unknown execution outcomes must not carry this sentinel. An independent
// helper-removal failure may be joined after refusal is proven; callers must
// still return that failure, but must not roll back the unchanged namespace.
var ErrNetemNotApplied = errors.New("netem refused before changing the namespace")
