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
