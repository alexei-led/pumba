package container

import (
	"context"
	"net"
	"time"
)

const (
	defaultStopSignal = "SIGTERM"
	defaultKillSignal = "SIGKILL"
)

// A FilterFunc is a prototype for a function that can be used to filter the
// results from a call to the ListContainers() method on the Client.
type FilterFunc func(*Container) bool

// Client interface
//
//nolint:interfacebloat
type Client interface {
	ListContainers(context.Context, FilterFunc, ListOpts) ([]*Container, error)
	StopContainer(context.Context, *Container, int, bool) error
	KillContainer(context.Context, *Container, string, bool) error
	ExecContainer(context.Context, *Container, string, bool) error
	RestartContainer(context.Context, *Container, time.Duration, bool) error
	RemoveContainer(context.Context, *Container, bool, bool, bool, bool) error
	NetemContainer(context.Context, *Container, string, []string, []*net.IPNet, []string, []string, time.Duration, string, bool, bool) error
	StopNetemContainer(context.Context, *Container, string, []*net.IPNet, []string, []string, string, bool, bool) error
	IPTablesContainer(context.Context, *Container, []string, []string, []*net.IPNet, []*net.IPNet, []string, []string, time.Duration, string, bool, bool) error
	StopIPTablesContainer(context.Context, *Container, []string, []string, []*net.IPNet, []*net.IPNet, []string, []string, string, bool, bool) error
	PauseContainer(context.Context, *Container, bool) error
	UnpauseContainer(context.Context, *Container, bool) error
	StartContainer(context.Context, *Container, bool) error
	StressContainer(context.Context, *Container, []string, string, bool, time.Duration, bool) (string, <-chan string, <-chan error, error)
	StopContainerWithID(context.Context, string, time.Duration, bool) error
}

type imagePullResponse struct {
	Status         string `json:"status"`
	Error          string `json:"error"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"progressDetail"`
}
