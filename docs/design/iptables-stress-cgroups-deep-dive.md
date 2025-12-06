# IPTables, Stress Testing, and Cgroups: Deep Dive Design

## Overview

This document extends the container runtime abstraction design with detailed research and implementation plans for IPTables, stress testing, and cgroups support across Docker and containerd runtimes.

---

## Part 1: IPTables Implementation for Containerd

### 1.1 How IPTables Works with Network Namespaces

Each container gets a complete, isolated IPTables ruleset:
- Completely isolated from host's IPTables rules
- Independent for each namespace
- Fresh/empty when namespace is created (except built-in chains)

**Key Architecture:**
```
┌─────────────────────────────────────────────────────────────┐
│ Host Network Namespace                                       │
│  ┌─────────────────────┐     ┌─────────────────────────────┐│
│  │ Host IPTables       │     │ veth_host ←──────────────┐  ││
│  │ - PREROUTING        │     │ (bridge side)            │  ││
│  │ - FORWARD           │     └─────────────────────────────┘││
│  │ - POSTROUTING (NAT) │                                 │  ││
│  └─────────────────────┘                                 │  ││
└──────────────────────────────────────────────────────────│──┘
                                                           │
                              virtual pipe                 │
                                                           │
┌──────────────────────────────────────────────────────────│──┐
│ Container Network Namespace                              │  │
│  ┌─────────────────────┐     ┌─────────────────────────────┐│
│  │ Container IPTables  │     │ eth0 ←────────────────────┘ ││
│  │ - INPUT             │     │ (container side)            ││
│  │ - OUTPUT            │     └─────────────────────────────┘│
│  │ - FORWARD           │                                    │
│  └─────────────────────┘                                    │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 Implementation Approaches

#### Approach 1: nsenter with iptables binary (Recommended)

```go
// pkg/container/containerd_iptables.go

package container

import (
    "context"
    "fmt"
    "net"
    "os/exec"
    "runtime"
    "strings"
    "time"

    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
)

// IPTablesContainer injects iptables rules for containerd containers
func (c *containerdClient) IPTablesContainer(
    ctx context.Context,
    container *Container,
    cmdPrefix, cmdSuffix []string,
    srcIPs, dstIPs []*net.IPNet,
    sports, dports []string,
    duration time.Duration,
    image string,
    pull, dryrun bool,
) error {
    log.WithFields(log.Fields{
        "name":          container.Name,
        "id":            container.ID,
        "commandPrefix": cmdPrefix,
        "commandSuffix": cmdSuffix,
        "srcIPs":        srcIPs,
        "dstIPs":        dstIPs,
        "dryrun":        dryrun,
    }).Info("running iptables on container")

    if dryrun {
        return nil
    }

    // Verify container has valid PID
    if container.PID == 0 {
        return errors.New("container PID not available - container may not be running")
    }

    // Build iptables commands
    commands := c.buildIPTablesCommands(cmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports)

    // Execute commands in container's network namespace
    for _, cmd := range commands {
        if err := c.execIPTablesInNamespace(ctx, container.PID, cmd); err != nil {
            return errors.Wrapf(err, "failed to execute iptables command: %v", strings.Join(cmd, " "))
        }
    }

    return nil
}

// execIPTablesInNamespace executes iptables command in container's network namespace
func (c *containerdClient) execIPTablesInNamespace(ctx context.Context, pid int, args []string) error {
    // Build nsenter command
    nsenterArgs := []string{
        "-t", fmt.Sprintf("%d", pid),
        "-n", // Enter network namespace
        "--",
        "iptables",
    }
    nsenterArgs = append(nsenterArgs, args...)

    log.WithFields(log.Fields{
        "pid":  pid,
        "args": strings.Join(args, " "),
    }).Debug("executing iptables via nsenter")

    cmd := exec.CommandContext(ctx, "nsenter", nsenterArgs...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return errors.Wrapf(err, "iptables command failed: %s", string(output))
    }

    return nil
}

// buildIPTablesCommands builds the list of iptables commands to execute
func (c *containerdClient) buildIPTablesCommands(
    cmdPrefix, cmdSuffix []string,
    srcIPs, dstIPs []*net.IPNet,
    sports, dports []string,
) [][]string {
    var commands [][]string

    // No IP/port filters - single command
    if len(srcIPs) == 0 && len(dstIPs) == 0 && len(sports) == 0 && len(dports) == 0 {
        cmd := append([]string{}, cmdPrefix...)
        cmd = append(cmd, cmdSuffix...)
        return [][]string{cmd}
    }

    // Source IP filters
    for _, ip := range srcIPs {
        cmd := append([]string{}, cmdPrefix...)
        cmd = append(cmd, "-s", ip.String())
        cmd = append(cmd, cmdSuffix...)
        commands = append(commands, cmd)
    }

    // Destination IP filters
    for _, ip := range dstIPs {
        cmd := append([]string{}, cmdPrefix...)
        cmd = append(cmd, "-d", ip.String())
        cmd = append(cmd, cmdSuffix...)
        commands = append(commands, cmd)
    }

    // Source port filters
    for _, sport := range sports {
        cmd := append([]string{}, cmdPrefix...)
        cmd = append(cmd, "--sport", sport)
        cmd = append(cmd, cmdSuffix...)
        commands = append(commands, cmd)
    }

    // Destination port filters
    for _, dport := range dports {
        cmd := append([]string{}, cmdPrefix...)
        cmd = append(cmd, "--dport", dport)
        cmd = append(cmd, cmdSuffix...)
        commands = append(commands, cmd)
    }

    return commands
}

// StopIPTablesContainer removes iptables rules from container
func (c *containerdClient) StopIPTablesContainer(
    ctx context.Context,
    container *Container,
    cmdPrefix, cmdSuffix []string,
    srcIPs, dstIPs []*net.IPNet,
    sports, dports []string,
    image string,
    pull, dryrun bool,
) error {
    log.WithFields(log.Fields{
        "name":   container.Name,
        "id":     container.ID,
        "dryrun": dryrun,
    }).Info("stopping iptables on container")

    if dryrun {
        return nil
    }

    if container.PID == 0 {
        return errors.New("container PID not available")
    }

    // Convert -A (append) or -I (insert) to -D (delete)
    deleteCmdPrefix := make([]string, len(cmdPrefix))
    copy(deleteCmdPrefix, cmdPrefix)
    for i, arg := range deleteCmdPrefix {
        if arg == "-A" || arg == "-I" {
            deleteCmdPrefix[i] = "-D"
            break
        }
    }

    commands := c.buildIPTablesCommands(deleteCmdPrefix, cmdSuffix, srcIPs, dstIPs, sports, dports)

    for _, cmd := range commands {
        // Ignore errors on cleanup - rules may not exist
        c.execIPTablesInNamespace(ctx, container.PID, cmd)
    }

    return nil
}
```

#### Approach 2: Pure Go with setns() and go-iptables (Advanced)

```go
// pkg/container/iptables_namespace.go

package container

import (
    "context"
    "fmt"
    "net"
    "runtime"

    "github.com/coreos/go-iptables/iptables"
    "github.com/vishvananda/netns"
    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
)

// IPTablesManager handles iptables operations in network namespaces
type IPTablesManager struct {
    ipt *iptables.IPTables
}

// ExecuteInNamespace executes iptables operations in a container's network namespace
func (m *IPTablesManager) ExecuteInNamespace(pid int, fn func(*iptables.IPTables) error) error {
    // CRITICAL: Lock this goroutine to its OS thread
    // Go's runtime multiplexes goroutines onto threads; without locking,
    // the goroutine could switch threads and end up in the wrong namespace
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()

    // Get target namespace from PID
    targetNs, err := netns.GetFromPid(pid)
    if err != nil {
        return errors.Wrapf(err, "failed to get namespace for PID %d", pid)
    }
    defer targetNs.Close()

    // Save current namespace for restoration
    origNs, err := netns.Get()
    if err != nil {
        return errors.Wrap(err, "failed to get current namespace")
    }
    defer origNs.Close()

    // Switch to target namespace
    if err := netns.Set(targetNs); err != nil {
        return errors.Wrap(err, "failed to switch to target namespace")
    }

    // Ensure we return to original namespace
    defer func() {
        if err := netns.Set(origNs); err != nil {
            log.WithError(err).Error("failed to restore original namespace")
        }
    }()

    // Create iptables client (now in target namespace)
    ipt, err := iptables.New()
    if err != nil {
        return errors.Wrap(err, "failed to create iptables client in namespace")
    }

    // Execute the provided function
    return fn(ipt)
}

// AddDropRule adds a DROP rule for chaos testing
func (m *IPTablesManager) AddDropRule(pid int, chain string, ruleSpec []string) error {
    return m.ExecuteInNamespace(pid, func(ipt *iptables.IPTables) error {
        return ipt.Append("filter", chain, ruleSpec...)
    })
}

// DeleteDropRule removes a DROP rule
func (m *IPTablesManager) DeleteDropRule(pid int, chain string, ruleSpec []string) error {
    return m.ExecuteInNamespace(pid, func(ipt *iptables.IPTables) error {
        return ipt.Delete("filter", chain, ruleSpec...)
    })
}

// SimulatePacketLoss adds statistical packet drop rule
func (m *IPTablesManager) SimulatePacketLoss(pid int, probability float64, chain string) error {
    ruleSpec := []string{
        "-m", "statistic",
        "--mode", "random",
        "--probability", fmt.Sprintf("%.4f", probability),
        "-j", "DROP",
    }

    return m.ExecuteInNamespace(pid, func(ipt *iptables.IPTables) error {
        return ipt.Insert("filter", chain, 1, ruleSpec...)
    })
}
```

### 1.3 Required Capabilities and Permissions

| Capability | Purpose |
|------------|---------|
| `CAP_NET_ADMIN` | Modify iptables rules, routing tables, interface config |
| `CAP_NET_RAW` | Use raw sockets (often needed with NET_ADMIN) |
| `CAP_SYS_ADMIN` | Required for nsenter and namespace operations |

**For Pumba container:**
```yaml
securityContext:
  capabilities:
    add:
      - NET_ADMIN
      - NET_RAW
      - SYS_ADMIN
```

### 1.4 Go Libraries for IPTables

| Library | Purpose | Notes |
|---------|---------|-------|
| `github.com/coreos/go-iptables/iptables` | IPTables manipulation | Recommended, well-maintained |
| `github.com/vishvananda/netns` | Network namespace switching | Simple, widely used |
| `github.com/vishvananda/netlink` | Netlink kernel communication | Lower-level, for routes/interfaces |
| `github.com/google/nftables` | NFTables (iptables successor) | Modern alternative |
| `github.com/containernetworking/plugins/pkg/ns` | CNI namespace utilities | Used in Kubernetes CNI plugins |

---

## Part 2: Cgroups Architecture

### 2.1 Cgroups v1 vs v2 Comparison

| Aspect | Cgroups v1 | Cgroups v2 |
|--------|-----------|-----------|
| **Hierarchy** | Multiple (one per controller) | Single unified |
| **Mount Point** | `/sys/fs/cgroup/<controller>/` | `/sys/fs/cgroup/` |
| **Process Placement** | Can be in multiple cgroups | Single cgroup only |
| **Internal Processes** | Allowed anywhere | Only in leaf nodes |
| **Delegation** | Not safe for non-root | Officially supported |
| **Rootless Containers** | Not supported | Supported |
| **Controller Interface** | Different per controller | Unified interface |

### 2.2 Filesystem Layout

**Cgroups v1:**
```
/sys/fs/cgroup/
├── cpu,cpuacct/
│   └── docker/
│       └── <container-id>/
│           ├── cpu.shares
│           ├── cpu.cfs_quota_us
│           └── cpu.cfs_period_us
├── memory/
│   └── docker/
│       └── <container-id>/
│           ├── memory.limit_in_bytes
│           └── memory.usage_in_bytes
├── pids/
│   └── docker/
│       └── <container-id>/
│           └── pids.max
└── blkio/
    └── docker/
        └── <container-id>/
            └── blkio.throttle.*
```

**Cgroups v2 (Unified):**
```
/sys/fs/cgroup/
├── cgroup.controllers          # Available controllers
├── cgroup.subtree_control      # Enabled for children
├── system.slice/
│   └── docker-<container-id>.scope/
│       ├── cgroup.procs        # PIDs in this cgroup
│       ├── memory.max          # Memory limit
│       ├── memory.current      # Current usage
│       ├── cpu.max             # CPU limit (quota period)
│       ├── cpu.weight          # CPU shares
│       ├── io.max              # I/O limits
│       └── pids.max            # Process limit
└── user.slice/
    └── user-1000.slice/
        └── ...
```

### 2.3 Detecting Cgroup Version

```go
// pkg/container/cgroup_detect.go

package container

import (
    "os"
    "syscall"

    "github.com/containerd/cgroups/v3"
)

// CgroupMode represents the cgroup version
type CgroupMode int

const (
    CgroupV1 CgroupMode = iota
    CgroupV2
    CgroupHybrid
)

// DetectCgroupMode detects which cgroup version is in use
func DetectCgroupMode() CgroupMode {
    // Using containerd/cgroups library
    if cgroups.Mode() == cgroups.Unified {
        return CgroupV2
    }
    return CgroupV1
}

// DetectCgroupModeFromFS detects cgroup version from filesystem
func DetectCgroupModeFromFS() (CgroupMode, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs("/sys/fs/cgroup", &stat); err != nil {
        return CgroupV1, err
    }

    // Magic numbers for filesystem types
    const (
        CGROUP2_SUPER_MAGIC = 0x63677270 // cgroup2fs
        TMPFS_MAGIC         = 0x01021994 // tmpfs (used by cgroup v1)
    )

    switch stat.Type {
    case CGROUP2_SUPER_MAGIC:
        return CgroupV2, nil
    case TMPFS_MAGIC:
        // Check for hybrid mode
        if _, err := os.Stat("/sys/fs/cgroup/unified"); err == nil {
            return CgroupHybrid, nil
        }
        return CgroupV1, nil
    default:
        return CgroupV1, nil
    }
}
```

### 2.4 Finding Container Cgroup Paths

```go
// pkg/container/cgroup_path.go

package container

import (
    "bufio"
    "fmt"
    "os"
    "strings"

    "github.com/pkg/errors"
)

// CgroupPath holds cgroup path information
type CgroupPath struct {
    Unified    string            // Cgroup v2 unified path
    Controllers map[string]string // Cgroup v1 controller paths
}

// GetCgroupPath gets the cgroup path for a process
func GetCgroupPath(pid int) (*CgroupPath, error) {
    cgroupFile := fmt.Sprintf("/proc/%d/cgroup", pid)

    file, err := os.Open(cgroupFile)
    if err != nil {
        return nil, errors.Wrapf(err, "failed to open %s", cgroupFile)
    }
    defer file.Close()

    result := &CgroupPath{
        Controllers: make(map[string]string),
    }

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        parts := strings.SplitN(line, ":", 3)
        if len(parts) != 3 {
            continue
        }

        hierarchyID := parts[0]
        controllers := parts[1]
        path := parts[2]

        if hierarchyID == "0" {
            // Cgroup v2 unified hierarchy
            result.Unified = path
        } else {
            // Cgroup v1 - map each controller to its path
            for _, controller := range strings.Split(controllers, ",") {
                result.Controllers[controller] = path
            }
        }
    }

    return result, scanner.Err()
}

// GetCgroupFullPath returns the full filesystem path for a cgroup
func GetCgroupFullPath(pid int, controller string) (string, error) {
    cgroupPath, err := GetCgroupPath(pid)
    if err != nil {
        return "", err
    }

    mode := DetectCgroupMode()

    switch mode {
    case CgroupV2:
        return fmt.Sprintf("/sys/fs/cgroup%s", cgroupPath.Unified), nil
    case CgroupV1:
        if path, ok := cgroupPath.Controllers[controller]; ok {
            return fmt.Sprintf("/sys/fs/cgroup/%s%s", controller, path), nil
        }
        return "", errors.Errorf("controller %s not found for PID %d", controller, pid)
    default:
        return "", errors.New("unknown cgroup mode")
    }
}

// Example outputs:
// Docker v1: /sys/fs/cgroup/memory/docker/abc123.../
// Docker v2: /sys/fs/cgroup/system.slice/docker-abc123.scope/
// Containerd v2: /sys/fs/cgroup/system.slice/containerd-abc123.scope/
// K8s v2: /sys/fs/cgroup/kubelet.slice/kubelet-kubepods.slice/.../
```

---

## Part 3: Stress Testing Implementation

### 3.1 Current Docker Approach (dockhack)

The current Pumba implementation uses a "dockhack" approach:

```
┌─────────────────────────────────────────────────────────────┐
│ Pumba Container                                             │
│                                                             │
│  1. Mount Docker socket (/var/run/docker.sock)              │
│  2. Mount cgroup filesystem (/sys/fs/cgroup)                │
│  3. Create stress container with:                           │
│     - CAP_SYS_ADMIN                                         │
│     - apparmor:unconfined                                   │
│     - entrypoint: dockhack cg_exec <target-id> stress-ng    │
│                                                             │
│  dockhack cg_exec:                                          │
│  1. Reads target container's cgroup path                    │
│  2. Uses cgexec to run stress-ng in that cgroup             │
│  3. stress-ng respects cgroup resource limits               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Limitations:**
- Requires Docker socket mount (security risk)
- Requires CAP_SYS_ADMIN + apparmor:unconfined
- Not portable to containerd
- Not suitable for rootless containers

### 3.2 Alternative Approaches for Containerd

#### Approach A: Direct Cgroup Manipulation

```go
// pkg/container/containerd_stress.go

package container

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "time"

    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
)

// StressContainer runs stress tests in container's cgroup
func (c *containerdClient) StressContainer(
    ctx context.Context,
    container *Container,
    stressors []string,
    image string,
    pull bool,
    duration time.Duration,
    dryrun bool,
) (string, <-chan string, <-chan error, error) {
    log.WithFields(log.Fields{
        "name":      container.Name,
        "id":        container.ID,
        "stressors": stressors,
        "duration":  duration,
        "dryrun":    dryrun,
    }).Info("stress testing container")

    if dryrun {
        return "", nil, nil, nil
    }

    // Verify container is running and has valid PID
    if container.PID == 0 {
        return "", nil, nil, errors.New("container PID not available - container may not be running")
    }

    // Get container's cgroup path
    cgroupPath, err := GetCgroupFullPath(container.PID, "")
    if err != nil {
        return "", nil, nil, errors.Wrap(err, "failed to get container cgroup path")
    }

    log.WithField("cgroup", cgroupPath).Debug("found container cgroup")

    output := make(chan string, 1)
    outerr := make(chan error, 1)

    go func() {
        defer close(output)
        defer close(outerr)

        // Start stress-ng process
        args := append([]string{"--timeout", fmt.Sprintf("%ds", int(duration.Seconds()))}, stressors...)
        cmd := exec.CommandContext(ctx, "stress-ng", args...)

        if err := cmd.Start(); err != nil {
            outerr <- errors.Wrap(err, "failed to start stress-ng")
            return
        }

        stressPID := cmd.Process.Pid
        log.WithField("stress_pid", stressPID).Debug("stress-ng started")

        // Move stress-ng process to container's cgroup
        if err := moveToCgroup(stressPID, cgroupPath); err != nil {
            cmd.Process.Kill()
            outerr <- errors.Wrap(err, "failed to move stress-ng to container cgroup")
            return
        }

        log.WithFields(log.Fields{
            "stress_pid": stressPID,
            "cgroup":     cgroupPath,
        }).Debug("moved stress-ng to container cgroup")

        // Wait for completion
        if err := cmd.Wait(); err != nil {
            if ctx.Err() == context.Canceled {
                output <- "stress test canceled"
                return
            }
            outerr <- errors.Wrap(err, "stress-ng failed")
            return
        }

        output <- "stress test completed successfully"
    }()

    return fmt.Sprintf("stress-%d", time.Now().UnixNano()), output, outerr, nil
}

// moveToCgroup moves a process to a specific cgroup
func moveToCgroup(pid int, cgroupPath string) error {
    mode := DetectCgroupMode()

    switch mode {
    case CgroupV2:
        return moveToCgroupV2(pid, cgroupPath)
    case CgroupV1:
        return moveToCgroupV1(pid, cgroupPath)
    default:
        return errors.New("unknown cgroup mode")
    }
}

// moveToCgroupV2 moves process to cgroup v2 hierarchy
func moveToCgroupV2(pid int, cgroupPath string) error {
    procsFile := fmt.Sprintf("%s/cgroup.procs", cgroupPath)

    f, err := os.OpenFile(procsFile, os.O_WRONLY|os.O_APPEND, 0)
    if err != nil {
        return errors.Wrapf(err, "failed to open %s", procsFile)
    }
    defer f.Close()

    _, err = f.WriteString(fmt.Sprintf("%d\n", pid))
    if err != nil {
        return errors.Wrapf(err, "failed to write PID %d to %s", pid, procsFile)
    }

    return nil
}

// moveToCgroupV1 moves process to cgroup v1 hierarchies
func moveToCgroupV1(pid int, basePath string) error {
    // For v1, we need to write to each controller's cgroup.procs
    controllers := []string{"cpu,cpuacct", "memory", "pids", "blkio"}

    for _, controller := range controllers {
        procsFile := fmt.Sprintf("/sys/fs/cgroup/%s%s/cgroup.procs", controller, basePath)

        // Skip if controller not available
        if _, err := os.Stat(procsFile); os.IsNotExist(err) {
            continue
        }

        f, err := os.OpenFile(procsFile, os.O_WRONLY|os.O_APPEND, 0)
        if err != nil {
            return errors.Wrapf(err, "failed to open %s", procsFile)
        }

        _, err = f.WriteString(fmt.Sprintf("%d\n", pid))
        f.Close()

        if err != nil {
            return errors.Wrapf(err, "failed to write PID to %s", procsFile)
        }
    }

    return nil
}
```

#### Approach B: nsenter with Cgroup Join

```go
// pkg/container/stress_nsenter.go

package container

import (
    "context"
    "fmt"
    "os/exec"
    "time"

    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
)

// StressContainerWithNsenter uses nsenter to join container's cgroup
func (c *containerdClient) StressContainerWithNsenter(
    ctx context.Context,
    container *Container,
    stressors []string,
    duration time.Duration,
) error {
    if container.PID == 0 {
        return errors.New("container PID not available")
    }

    // Build nsenter command with cgroup namespace join
    // -C/--cgroup: Enter cgroup namespace
    args := []string{
        "-t", fmt.Sprintf("%d", container.PID),
        "-C", // Join cgroup namespace
        "--",
        "stress-ng",
        "--timeout", fmt.Sprintf("%ds", int(duration.Seconds())),
    }
    args = append(args, stressors...)

    log.WithFields(log.Fields{
        "pid":       container.PID,
        "stressors": stressors,
    }).Debug("executing stress-ng via nsenter")

    cmd := exec.CommandContext(ctx, "nsenter", args...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return errors.Wrapf(err, "stress-ng failed: %s", string(output))
    }

    return nil
}
```

#### Approach C: Using containerd/cgroups Library

```go
// pkg/container/stress_cgroups.go

package container

import (
    "context"
    "fmt"
    "os/exec"
    "time"

    "github.com/containerd/cgroups/v3"
    "github.com/containerd/cgroups/v3/cgroup2"
    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
)

// StressContainerWithCgroupsLib uses containerd/cgroups library
func (c *containerdClient) StressContainerWithCgroupsLib(
    ctx context.Context,
    container *Container,
    stressors []string,
    duration time.Duration,
) error {
    if container.PID == 0 {
        return errors.New("container PID not available")
    }

    // Load the container's cgroup
    cgroupPath, err := GetCgroupPath(container.PID)
    if err != nil {
        return errors.Wrap(err, "failed to get cgroup path")
    }

    mode := cgroups.Mode()

    switch mode {
    case cgroups.Unified:
        return c.stressWithCgroupV2(ctx, cgroupPath.Unified, stressors, duration)
    default:
        return c.stressWithCgroupV1(ctx, cgroupPath, stressors, duration)
    }
}

func (c *containerdClient) stressWithCgroupV2(
    ctx context.Context,
    cgroupPath string,
    stressors []string,
    duration time.Duration,
) error {
    // Load existing cgroup
    manager, err := cgroup2.Load(cgroupPath)
    if err != nil {
        return errors.Wrap(err, "failed to load cgroup")
    }

    // Start stress-ng
    args := append([]string{"--timeout", fmt.Sprintf("%ds", int(duration.Seconds()))}, stressors...)
    cmd := exec.CommandContext(ctx, "stress-ng", args...)

    if err := cmd.Start(); err != nil {
        return errors.Wrap(err, "failed to start stress-ng")
    }

    // Add stress-ng process to container's cgroup
    if err := manager.AddProc(uint64(cmd.Process.Pid)); err != nil {
        cmd.Process.Kill()
        return errors.Wrap(err, "failed to add stress-ng to cgroup")
    }

    // Wait for completion
    return cmd.Wait()
}
```

### 3.3 Approach Comparison

| Approach | Requires | Pros | Cons |
|----------|----------|------|------|
| **Direct cgroup manipulation** | CAP_SYS_ADMIN, /sys/fs/cgroup access | No external dependencies, full control | Complex v1/v2 handling |
| **nsenter --cgroup** | nsenter binary, CAP_SYS_ADMIN | Simple, built-in tool | Requires stress-ng on host |
| **containerd/cgroups lib** | Go library, CAP_SYS_ADMIN | Clean API, handles v1/v2 | Library dependency |
| **Sidecar container** | Container runtime | Isolated, portable | Complex setup, overhead |

### 3.4 Cgroup v2 Considerations

**Important Limitation:**
> In cgroup v2, a cgroup that contains child cgroups cannot have processes inside it.

This means stress processes must go into the correct leaf cgroup:

```
/sys/fs/cgroup/
└── system.slice/
    └── docker-abc123.scope/     # Container cgroup
        ├── cgroup.procs         # ← Stress-ng PID goes here
        ├── memory.max
        └── cpu.max
```

**NOT:**
```
/sys/fs/cgroup/
└── system.slice/
    ├── cgroup.procs             # ← WRONG: parent has children
    └── docker-abc123.scope/
```

### 3.5 Required Permissions

| Operation | Capability | Notes |
|-----------|------------|-------|
| Read /proc/<pid>/cgroup | None | Any process can read |
| Write to cgroup.procs | CAP_SYS_ADMIN | Move processes between cgroups |
| Create sub-cgroups | CAP_SYS_ADMIN | mkdir in /sys/fs/cgroup |
| nsenter --cgroup | CAP_SYS_ADMIN | Enter cgroup namespace |

**Pumba Container Requirements:**
```yaml
securityContext:
  capabilities:
    add:
      - SYS_ADMIN    # cgroup manipulation
  volumeMounts:
    - name: cgroup
      mountPath: /sys/fs/cgroup
      readOnly: false  # Must be writable for cgroup.procs
volumes:
  - name: cgroup
    hostPath:
      path: /sys/fs/cgroup
      type: Directory
```

---

## Part 4: Implementation Design

### 4.1 Unified Network Chaos Module

```go
// pkg/container/network_chaos.go

package container

import (
    "context"
    "net"
    "time"
)

// NetworkChaosConfig holds network chaos configuration
type NetworkChaosConfig struct {
    // Target container
    Container *Container

    // Netem configuration
    Interface string
    NetemCmd  []string

    // IP filtering
    TargetIPs []*net.IPNet
    SrcPorts  []string
    DstPorts  []string

    // IPTables configuration
    IPTablesPrefix []string
    IPTablesSuffix []string
    SrcIPs         []*net.IPNet
    DstIPs         []*net.IPNet

    // Common options
    Duration time.Duration
    Image    string
    Pull     bool
    DryRun   bool
}

// NetworkChaosExecutor executes network chaos
type NetworkChaosExecutor interface {
    // Execute netem chaos
    Netem(ctx context.Context, config *NetworkChaosConfig) error
    StopNetem(ctx context.Context, config *NetworkChaosConfig) error

    // Execute iptables chaos
    IPTables(ctx context.Context, config *NetworkChaosConfig) error
    StopIPTables(ctx context.Context, config *NetworkChaosConfig) error
}

// NsenterNetworkChaos implements NetworkChaosExecutor using nsenter
type NsenterNetworkChaos struct{}

// NewNsenterNetworkChaos creates a new nsenter-based executor
func NewNsenterNetworkChaos() *NsenterNetworkChaos {
    return &NsenterNetworkChaos{}
}
```

### 4.2 Unified Stress Testing Module

```go
// pkg/container/stress_chaos.go

package container

import (
    "context"
    "time"
)

// StressChaosConfig holds stress testing configuration
type StressChaosConfig struct {
    // Target container
    Container *Container

    // Stress configuration
    Stressors []string  // e.g., ["--cpu", "2", "--vm", "1", "--vm-bytes", "256M"]
    Duration  time.Duration

    // Image (for sidecar approach)
    Image string
    Pull  bool

    // Options
    DryRun bool
}

// StressChaosExecutor executes stress chaos
type StressChaosExecutor interface {
    // Execute stress test
    Stress(ctx context.Context, config *StressChaosConfig) (id string, output <-chan string, errors <-chan error, err error)

    // Stop stress test
    StopStress(ctx context.Context, id string) error
}

// CgroupStressChaos implements StressChaosExecutor using direct cgroup manipulation
type CgroupStressChaos struct {
    mode CgroupMode
}

// NewCgroupStressChaos creates a new cgroup-based stress executor
func NewCgroupStressChaos() (*CgroupStressChaos, error) {
    mode := DetectCgroupMode()
    return &CgroupStressChaos{mode: mode}, nil
}
```

### 4.3 Feature Support Matrix (Updated)

| Feature | Docker | Containerd | Implementation |
|---------|--------|------------|----------------|
| **Basic Container Ops** | | | |
| List | ✅ | ✅ | Native API |
| Kill/Stop/Pause | ✅ | ✅ | Native API |
| Exec | ✅ | ✅ | Native API |
| **Network Chaos** | | | |
| Netem (tc) | ✅ | ✅ | nsenter + tc |
| IPTables | ✅ | ✅ | nsenter + iptables |
| IPTables (go-iptables) | ✅ | ✅ | setns() + go-iptables |
| **Stress Testing** | | | |
| CPU stress | ✅ | ✅ | cgroup + stress-ng |
| Memory stress | ✅ | ✅ | cgroup + stress-ng |
| I/O stress | ✅ | ✅ | cgroup + stress-ng |
| **Cgroups** | | | |
| Cgroup v1 | ✅ | ✅ | Direct fs / library |
| Cgroup v2 | ✅ | ✅ | containerd/cgroups lib |
| **Rootless** | ❌ | ⚠️ | Limited support |

---

## Part 5: Implementation Phases (Updated)

### Phase 1: Core Abstraction (Unchanged)
- Runtime-agnostic Container model
- Factory pattern for client creation

### Phase 2: Docker Refactoring (Unchanged)
- Update Docker client to new model

### Phase 3: Containerd Core Operations (Unchanged)
- Basic container operations via containerd API

### Phase 4: Network Chaos for Containerd (Updated)
1. Implement nsenter-based netem executor
2. Implement nsenter-based iptables executor
3. Add pure Go implementation with go-iptables + netns
4. Add capability detection and validation

### Phase 5: Stress Testing for Containerd (New)
1. Implement cgroup detection (v1/v2)
2. Implement cgroup path discovery
3. Implement process-to-cgroup movement
4. Add stress-ng execution with cgroup targeting
5. Test with both cgroup v1 and v2 systems

### Phase 6: CLI Integration (Updated)
1. Add runtime selection flags
2. Add cgroup mode detection
3. Add capability validation warnings

### Phase 7: Testing & Documentation
1. Unit tests for cgroup operations
2. Integration tests with containerd
3. Tests for cgroup v1 and v2
4. Update documentation

---

## Part 6: Security Considerations

### 6.1 Capability Requirements

| Feature | Minimum Capabilities |
|---------|---------------------|
| Network chaos (netem) | CAP_NET_ADMIN, CAP_NET_RAW |
| Network chaos (iptables) | CAP_NET_ADMIN, CAP_NET_RAW, CAP_SYS_ADMIN |
| Stress testing | CAP_SYS_ADMIN |
| Namespace operations | CAP_SYS_ADMIN |

### 6.2 Volume Mounts Required

```yaml
volumes:
  # For cgroup access (stress testing)
  - name: cgroup
    hostPath:
      path: /sys/fs/cgroup
      type: Directory

  # NOT recommended but needed for Docker sidecar approach
  # - name: docker-sock
  #   hostPath:
  #     path: /var/run/docker.sock
  #     type: Socket
```

### 6.3 Security Best Practices

1. **Avoid Docker socket mounting** - Use direct cgroup manipulation instead
2. **Use read-only mounts where possible** - Only /sys/fs/cgroup needs write
3. **Prefer nsenter over setns()** - Better auditing and security
4. **Validate container PID** - Ensure target is valid before operations
5. **Implement timeouts** - Prevent runaway stress tests
6. **Log all chaos operations** - For audit trail

### 6.4 Rootless Container Limitations

| Operation | Rootless v1 | Rootless v2 |
|-----------|-------------|-------------|
| Cgroup creation | ❌ | ⚠️ (delegated only) |
| Cgroup process move | ❌ | ⚠️ (delegated only) |
| Network namespace | ✅ (slirp4netns) | ✅ |
| IPTables | ❌ | ❌ |
| Stress testing | ❌ | ⚠️ (limited) |

---

## Conclusion

This extended design provides comprehensive implementation guidance for:

1. **IPTables support** via nsenter and pure Go approaches
2. **Stress testing** via direct cgroup manipulation
3. **Cgroups v1 and v2** detection and handling
4. **Security considerations** and capability requirements

The recommended approach uses:
- **nsenter** for network namespace operations (proven, auditable)
- **Direct cgroup.procs writes** for stress testing (no Docker socket needed)
- **containerd/cgroups library** for cgroup v2 compatibility

This eliminates the need for Docker socket mounting and provides a clean, portable solution for both Docker and containerd runtimes.
