package container

import (
	"context"
	"time"
)

// A FilterFunc is a prototype for a function that can be used to filter the
// results from a call to the ListContainers() method on the Client.
type FilterFunc func(*Container) bool

// --- Focused Interfaces ---

// Lister lists containers matching a filter.
type Lister interface {
	ListContainers(context.Context, FilterFunc, ListOpts) ([]*Container, error)
}

// Lifecycle manages container lifecycle (stop, kill, start, restart, remove, pause).
type Lifecycle interface {
	StopContainer(context.Context, *Container, int, bool) error
	KillContainer(context.Context, *Container, string, bool) error
	StartContainer(context.Context, *Container, bool) error
	RestartContainer(context.Context, *Container, time.Duration, bool) error
	RemoveContainer(context.Context, *Container, bool, bool, bool, bool) error
	PauseContainer(context.Context, *Container, bool) error
	UnpauseContainer(context.Context, *Container, bool) error
	StopContainerWithID(context.Context, string, time.Duration, bool) error
}

// Executor executes commands in containers.
type Executor interface {
	ExecContainer(context.Context, *Container, string, []string, bool) error
}

// Netem manages network emulation on containers. Requests are passed by
// pointer because NetemRequest is large (~160 bytes) — value semantics would
// cost a copy on every call.
type Netem interface {
	NetemContainer(context.Context, *NetemRequest) error
	StopNetemContainer(context.Context, *NetemRequest) error
}

// IPTables manages iptables rules on containers. Requests are passed by
// pointer for the same size reason as Netem.
type IPTables interface {
	IPTablesContainer(context.Context, *IPTablesRequest) error
	StopIPTablesContainer(context.Context, *IPTablesRequest) error
}

// Stressor manages stress testing on containers. Requests are passed by
// pointer to keep the call site small and to leave room for runtime hints
// (image / pull / inject-cgroup) without growing the signature again.
type Stressor interface {
	StressContainer(context.Context, *StressRequest) (*StressResult, error)
}

// Client is the full container runtime interface, combining all focused interfaces.
type Client interface {
	Lister
	Lifecycle
	Executor
	Netem
	IPTables
	Stressor
	Close() error
}
