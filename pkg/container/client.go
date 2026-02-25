package container

import (
	"context"
	"net"
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

// Netem manages network emulation on containers.
type Netem interface {
	NetemContainer(context.Context, *Container, string, []string, []*net.IPNet, []string, []string, time.Duration, string, bool, bool) error
	StopNetemContainer(context.Context, *Container, string, []*net.IPNet, []string, []string, string, bool, bool) error
}

// IPTables manages iptables rules on containers.
type IPTables interface {
	IPTablesContainer(context.Context, *Container, []string, []string, []*net.IPNet, []*net.IPNet, []string, []string, time.Duration, string, bool, bool) error
	StopIPTablesContainer(context.Context, *Container, []string, []string, []*net.IPNet, []*net.IPNet, []string, []string, string, bool, bool) error
}

// Stressor manages stress testing on containers.
type Stressor interface {
	StressContainer(context.Context, *Container, []string, string, bool, time.Duration, bool, bool) (string, <-chan string, <-chan error, error)
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
