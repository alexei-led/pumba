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
	// TargetNames holds --target values that were not valid IP/CIDR
	// literals at parse time (no runtime client is available yet then).
	// They are candidate container names or IDs. Each command invocation
	// resolves them into IPs on a request copy, so interval runs do not cache
	// addresses across container restarts. Empty for the common IP/CIDR-only
	// case, so existing callers see no behavior change. Runtime adapters
	// (pkg/runtime/*) never need to look at this field: by the time a
	// request reaches them it has already been resolved.
	TargetNames []string
	SPorts      []string
	DPorts      []string
	Duration    time.Duration
	Sidecar     SidecarSpec
	DryRun      bool
	// RunID labels the sidecar with the run. Empty means untagged.
	RunID string
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
	// RunID labels the sidecar and tags every rule with the iptables comment
	// "pumba:<RunID>" in both the add and the delete spec. Empty means
	// untagged.
	RunID string
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
	// RunID labels the stress sidecar with the run. Empty means untagged.
	RunID string
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

// InspectRequest asks a runtime for the Pumba fault state of Container's
// network namespace. Sidecar is the image hint for the short-lived inspect
// sidecar; an empty image runs the commands in the target itself. RunID only
// labels the inspect sidecar; it does not filter the result.
type InspectRequest struct {
	Container *Container
	Sidecar   SidecarSpec
	RunID     string
}

// QdiscOwnership classifies a qdisc tree that uses Pumba's root handle 504d:.
type QdiscOwnership string

const (
	// QdiscPumbaOwned: the topology matches what Pumba installs (netem root,
	// or prio + sfq + netem bands).
	QdiscPumbaOwned QdiscOwnership = "pumba-owned"
	// QdiscSuspicious: the handle is present but the topology is not Pumba's.
	QdiscSuspicious QdiscOwnership = "suspicious"
)

// PumbaQdisc is a device whose qdisc tree uses Pumba's root handle. Qdiscs
// carry no run ID, so it is never attributed to a run.
type PumbaQdisc struct {
	Device    string
	Kind      string
	Ownership QdiscOwnership
}

// PumbaRule is an iptables rule carrying the comment "pumba:<RunID>". Spec is
// the rule as `iptables -S` prints it.
type PumbaRule struct {
	RunID string
	Chain string
	Spec  string
}

// FaultState is the Pumba fault state found in one network namespace.
// Containers that share a netns report the same state.
type FaultState struct {
	Qdiscs []PumbaQdisc
	Rules  []PumbaRule
}
