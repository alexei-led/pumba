package container

import (
	"net"
	"time"
)

// SidecarSpec describes the sidecar container a runtime adapter may launch
// to apply network/iptables chaos. Runtimes that operate without a sidecar
// (or that resolve the image differently) are free to ignore these hints.
type SidecarSpec struct {
	Image string
	Pull  bool
}

// NetemRequest carries every parameter required to apply or stop a netem rule
// on a target container. Stop operations reuse the same struct; Duration is
// ignored on stop. Zero values are safe — slices may be nil and Sidecar may
// be left empty when the runtime does not need it.
type NetemRequest struct {
	Container *Container
	Interface string
	Command   []string
	IPs       []*net.IPNet
	SPorts    []string
	DPorts    []string
	Duration  time.Duration
	Sidecar   SidecarSpec
	DryRun    bool
}

// IPTablesRequest carries every parameter required to apply or stop an
// iptables rule on a target container. Stop operations reuse the same
// struct; Duration is ignored on stop. Zero values are safe.
type IPTablesRequest struct {
	Container *Container
	CmdPrefix []string
	CmdSuffix []string
	SrcIPs    []*net.IPNet
	DstIPs    []*net.IPNet
	SPorts    []string
	DPorts    []string
	Duration  time.Duration
	Sidecar   SidecarSpec
	DryRun    bool
}

// StressRequest carries every parameter required to apply a stress-ng workload
// against a target container. Sidecar describes the stress-ng image hint for
// runtimes that launch one; runtimes free to ignore (e.g. containerd direct
// exec) leave it empty. InjectCgroup selects between the default child-cgroup
// mode and the cg-inject sibling-cgroup mode.
type StressRequest struct {
	Container    *Container
	Stressors    []string
	Duration     time.Duration
	Sidecar      SidecarSpec
	InjectCgroup bool
	DryRun       bool
}

// StressResult bundles the sidecar identifier and the streaming output
// channels emitted by a successful StressContainer call. SidecarID is the
// runtime-specific handle the caller passes to StopContainerWithID for
// premature cancellation; Output and Errors close once the stress workload
// terminates.
type StressResult struct {
	SidecarID string
	Output    <-chan string
	Errors    <-chan error
}

// RemoveOpts bundles the boolean flags that govern Lifecycle.RemoveContainer.
// Force translates to the runtime's force-removal flag (typically SIGKILL +
// teardown). Links and Volumes opt into removing linked containers and
// associated volumes — runtimes that don't support a flag (e.g. containerd)
// log and ignore it. DryRun short-circuits the call without touching the
// runtime.
type RemoveOpts struct {
	Force   bool
	Links   bool
	Volumes bool
	DryRun  bool
}
