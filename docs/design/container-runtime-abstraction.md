# Container Runtime Abstraction Layer Design

## Overview

This document outlines the design for adding support for multiple container runtimes to Pumba, starting with containerd alongside the existing Docker support. The goal is to create a clean abstraction layer that allows Pumba to work with different container technologies while maintaining backward compatibility.

## Current Architecture Analysis

### Existing Structure

```
pkg/container/
├── client.go           # Client interface definition
├── container.go        # Container struct (Docker-coupled)
├── docker_client.go    # Docker implementation
├── http_client.go      # HTTP client for Docker daemon
├── util.go             # Filtering and listing utilities
└── mock_*.go           # Mock implementations for testing
```

### Current Client Interface

The existing `Client` interface in `pkg/container/client.go` defines 15 methods:

```go
type Client interface {
    ListContainers(context.Context, FilterFunc, ListOpts) ([]*Container, error)
    StopContainer(context.Context, *Container, int, bool) error
    KillContainer(context.Context, *Container, string, bool) error
    ExecContainer(context.Context, *Container, string, []string, bool) error
    RestartContainer(context.Context, *Container, time.Duration, bool) error
    RemoveContainer(context.Context, *Container, bool, bool, bool, bool) error
    PauseContainer(context.Context, *Container, bool) error
    UnpauseContainer(context.Context, *Container, bool) error
    StartContainer(context.Context, *Container, bool) error
    StopContainerWithID(context.Context, string, time.Duration, bool) error

    // Network chaos operations
    NetemContainer(...)
    StopNetemContainer(...)
    IPTablesContainer(...)
    StopIPTablesContainer(...)

    // Stress testing
    StressContainer(...)
}
```

### Current Container Model

The `Container` struct is tightly coupled to Docker types:

```go
type Container struct {
    ContainerInfo types.ContainerJSON  // Docker API response
    ImageInfo     types.ImageInspect   // Docker image details
}
```

### Key Coupling Points

1. **Container struct**: Directly embeds `types.ContainerJSON` and `types.ImageInspect`
2. **Global client**: `chaos.DockerClient` is a global variable
3. **CLI flags**: Docker-specific (`--host`, `--tls*` flags)
4. **NewClient function**: Only creates Docker clients

---

## Proposed Design

### Design Principles

1. **Runtime Agnostic**: Core operations should work identically across runtimes
2. **Backward Compatible**: Existing Docker workflows must continue unchanged
3. **Extensible**: Easy to add new runtimes (Podman, CRI-O, etc.)
4. **Minimal API Changes**: Preserve the existing `Client` interface where possible
5. **Feature Parity Where Possible**: Implement equivalent functionality for each runtime

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                           CLI Layer                             │
│   --runtime docker|containerd  --host  --namespace             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Runtime Factory                            │
│              NewClient(runtime, config) Client                  │
└─────────────────────────────────────────────────────────────────┘
                                │
                ┌───────────────┴───────────────┐
                ▼                               ▼
┌───────────────────────────┐   ┌───────────────────────────────┐
│      Docker Client        │   │      Containerd Client        │
│  (existing dockerClient)  │   │   (new containerdClient)      │
└───────────────────────────┘   └───────────────────────────────┘
                │                               │
                ▼                               ▼
┌───────────────────────────┐   ┌───────────────────────────────┐
│   Docker Engine API       │   │    Containerd gRPC API        │
│  /var/run/docker.sock     │   │ /run/containerd/containerd.sock│
└───────────────────────────┘   └───────────────────────────────┘
```

---

## Detailed Design

### 1. Runtime-Agnostic Container Model

Create a new runtime-agnostic container representation:

```go
// pkg/container/model.go

package container

// RuntimeType identifies the container runtime
type RuntimeType string

const (
    RuntimeDocker     RuntimeType = "docker"
    RuntimeContainerd RuntimeType = "containerd"
)

// ContainerState represents the state of a container
type ContainerState string

const (
    StateRunning    ContainerState = "running"
    StatePaused     ContainerState = "paused"
    StateStopped    ContainerState = "stopped"
    StateCreated    ContainerState = "created"
    StateRestarting ContainerState = "restarting"
    StateExited     ContainerState = "exited"
    StateDead       ContainerState = "dead"
    StateUnknown    ContainerState = "unknown"
)

// Container represents a runtime-agnostic container
type Container struct {
    // Core identification
    ID        string            // Container ID
    Name      string            // Container name (without leading slash)
    Namespace string            // Namespace (containerd) or empty (Docker)

    // Image information
    ImageID   string            // Image ID
    ImageName string            // Image name with tag

    // State
    State     ContainerState    // Current state
    PID       int               // Main process PID (0 if not running)

    // Metadata
    Labels    map[string]string // Container labels
    Created   time.Time         // Creation timestamp

    // Network (for network chaos operations)
    NetworkMode    string       // Network mode/namespace identifier
    NetworkNSPath  string       // Path to network namespace (for sidecar injection)

    // Runtime-specific data (opaque to chaos commands)
    Runtime       RuntimeType   // Which runtime this container belongs to
    RuntimeData   interface{}   // Runtime-specific data for advanced operations
}

// Helper methods
func (c *Container) IsPumba() bool {
    val, ok := c.Labels[pumbaLabel]
    return ok && val == trueValue
}

func (c *Container) IsPumbaSkip() bool {
    val, ok := c.Labels[pumbaSkipLabel]
    return ok && val == trueValue
}

func (c *Container) StopSignal() string {
    if val, ok := c.Labels[signalLabel]; ok {
        return val
    }
    return ""
}

func (c *Container) IsRunning() bool {
    return c.State == StateRunning
}

func (c *Container) IsPaused() bool {
    return c.State == StatePaused
}
```

### 2. Enhanced Client Interface

Extend the interface to support runtime-specific configuration:

```go
// pkg/container/client.go

package container

// ClientConfig holds runtime-agnostic client configuration
type ClientConfig struct {
    // Connection
    Host      string          // Socket path or host address
    TLSConfig *tls.Config     // TLS configuration (optional)

    // Containerd-specific
    Namespace string          // Containerd namespace (default: "default")

    // Timeouts
    Timeout   time.Duration   // Connection/operation timeout
}

// DefaultDockerConfig returns default Docker configuration
func DefaultDockerConfig() *ClientConfig {
    return &ClientConfig{
        Host:    "unix:///var/run/docker.sock",
        Timeout: 30 * time.Second,
    }
}

// DefaultContainerdConfig returns default containerd configuration
func DefaultContainerdConfig() *ClientConfig {
    return &ClientConfig{
        Host:      "/run/containerd/containerd.sock",
        Namespace: "default",
        Timeout:   30 * time.Second,
    }
}

// Client interface remains largely unchanged
// but now operates on the new Container model
type Client interface {
    // Core container operations
    ListContainers(context.Context, FilterFunc, ListOpts) ([]*Container, error)
    StopContainer(context.Context, *Container, int, bool) error
    KillContainer(context.Context, *Container, string, bool) error
    ExecContainer(context.Context, *Container, string, []string, bool) error
    RestartContainer(context.Context, *Container, time.Duration, bool) error
    RemoveContainer(context.Context, *Container, bool, bool, bool, bool) error
    PauseContainer(context.Context, *Container, bool) error
    UnpauseContainer(context.Context, *Container, bool) error
    StartContainer(context.Context, *Container, bool) error
    StopContainerWithID(context.Context, string, time.Duration, bool) error

    // Network chaos operations
    NetemContainer(context.Context, *Container, string, []string, []*net.IPNet,
                   []string, []string, time.Duration, string, bool, bool) error
    StopNetemContainer(context.Context, *Container, string, []*net.IPNet,
                       []string, []string, string, bool, bool) error
    IPTablesContainer(context.Context, *Container, []string, []string, []*net.IPNet,
                      []*net.IPNet, []string, []string, time.Duration, string, bool, bool) error
    StopIPTablesContainer(context.Context, *Container, []string, []string, []*net.IPNet,
                          []*net.IPNet, []string, []string, string, bool, bool) error

    // Stress testing
    StressContainer(context.Context, *Container, []string, string, bool,
                   time.Duration, bool) (string, <-chan string, <-chan error, error)

    // Runtime metadata
    RuntimeType() RuntimeType
    Close() error
}
```

### 3. Runtime Factory

Create a factory for instantiating the appropriate client:

```go
// pkg/container/factory.go

package container

import (
    "github.com/pkg/errors"
)

// NewClient creates a new container client for the specified runtime
func NewClient(runtime RuntimeType, config *ClientConfig) (Client, error) {
    if config == nil {
        switch runtime {
        case RuntimeDocker:
            config = DefaultDockerConfig()
        case RuntimeContainerd:
            config = DefaultContainerdConfig()
        default:
            return nil, errors.Errorf("unknown runtime: %s", runtime)
        }
    }

    switch runtime {
    case RuntimeDocker:
        return NewDockerClient(config)
    case RuntimeContainerd:
        return NewContainerdClient(config)
    default:
        return nil, errors.Errorf("unsupported runtime: %s", runtime)
    }
}

// ParseRuntimeType parses a runtime type string
func ParseRuntimeType(s string) (RuntimeType, error) {
    switch s {
    case "docker", "":
        return RuntimeDocker, nil
    case "containerd":
        return RuntimeContainerd, nil
    default:
        return "", errors.Errorf("unknown runtime type: %s", s)
    }
}
```

### 4. Docker Client Refactoring

Update the existing Docker client to work with the new model:

```go
// pkg/container/docker_client.go

package container

// dockerClient implements Client for Docker
type dockerClient struct {
    containerAPI dockerapi.ContainerAPIClient
    imageAPI     dockerapi.ImageAPIClient
}

// NewDockerClient creates a new Docker client
func NewDockerClient(config *ClientConfig) (Client, error) {
    httpClient, err := HTTPClient(config.Host, config.TLSConfig)
    if err != nil {
        return nil, err
    }

    apiClient, err := dockerapi.NewClientWithOpts(
        dockerapi.WithHost(config.Host),
        dockerapi.WithHTTPClient(httpClient),
        dockerapi.WithAPIVersionNegotiation(),
    )
    if err != nil {
        return nil, errors.Wrap(err, "failed to create docker client")
    }

    return &dockerClient{
        containerAPI: apiClient,
        imageAPI:     apiClient,
    }, nil
}

func (c *dockerClient) RuntimeType() RuntimeType {
    return RuntimeDocker
}

func (c *dockerClient) Close() error {
    return nil // Docker client doesn't need explicit close
}

// ListContainers converts Docker containers to runtime-agnostic model
func (c *dockerClient) ListContainers(ctx context.Context, fn FilterFunc, opts ListOpts) ([]*Container, error) {
    // ... implementation converts types.ContainerJSON to Container
}

// toContainer converts Docker container info to runtime-agnostic Container
func toContainer(info types.ContainerJSON, imageInfo types.ImageInspect) *Container {
    state := stateFromDockerState(info.State)

    name := info.Name
    if strings.HasPrefix(name, "/") {
        name = name[1:] // Remove leading slash
    }

    networkNSPath := ""
    if info.State.Pid > 0 {
        networkNSPath = fmt.Sprintf("/proc/%d/ns/net", info.State.Pid)
    }

    return &Container{
        ID:            info.ID,
        Name:          name,
        Namespace:     "", // Docker doesn't use namespaces
        ImageID:       imageInfo.ID,
        ImageName:     normalizeImageName(info.Image),
        State:         state,
        PID:           info.State.Pid,
        Labels:        info.Config.Labels,
        Created:       info.Created,
        NetworkMode:   string(info.HostConfig.NetworkMode),
        NetworkNSPath: networkNSPath,
        Runtime:       RuntimeDocker,
        RuntimeData:   &DockerRuntimeData{
            ContainerJSON: info,
            ImageInspect:  imageInfo,
        },
    }
}

// DockerRuntimeData holds Docker-specific container data
type DockerRuntimeData struct {
    ContainerJSON types.ContainerJSON
    ImageInspect  types.ImageInspect
}
```

### 5. Containerd Client Implementation

New containerd client implementation:

```go
// pkg/container/containerd_client.go

package container

import (
    "context"
    "fmt"
    "syscall"
    "time"

    "github.com/containerd/containerd"
    "github.com/containerd/containerd/cio"
    "github.com/containerd/containerd/namespaces"
    "github.com/opencontainers/runtime-spec/specs-go"
    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
)

// containerdClient implements Client for containerd
type containerdClient struct {
    client    *containerd.Client
    namespace string
}

// NewContainerdClient creates a new containerd client
func NewContainerdClient(config *ClientConfig) (Client, error) {
    client, err := containerd.New(
        config.Host,
        containerd.WithDefaultNamespace(config.Namespace),
        containerd.WithTimeout(config.Timeout),
    )
    if err != nil {
        return nil, errors.Wrap(err, "failed to create containerd client")
    }

    // Verify connection
    ctx := namespaces.WithNamespace(context.Background(), config.Namespace)
    if _, err := client.Version(ctx); err != nil {
        return nil, errors.Wrap(err, "failed to connect to containerd")
    }

    return &containerdClient{
        client:    client,
        namespace: config.Namespace,
    }, nil
}

func (c *containerdClient) RuntimeType() RuntimeType {
    return RuntimeContainerd
}

func (c *containerdClient) Close() error {
    return c.client.Close()
}

// withNamespace creates a context with the client's namespace
func (c *containerdClient) withNamespace(ctx context.Context) context.Context {
    return namespaces.WithNamespace(ctx, c.namespace)
}

// ListContainers lists all containers matching the filter
func (c *containerdClient) ListContainers(ctx context.Context, fn FilterFunc, opts ListOpts) ([]*Container, error) {
    ctx = c.withNamespace(ctx)

    // Build filter string for containerd
    var filters []string
    for _, label := range opts.Labels {
        filters = append(filters, fmt.Sprintf("labels.%q", label))
    }

    containers, err := c.client.Containers(ctx, filters...)
    if err != nil {
        return nil, errors.Wrap(err, "failed to list containers")
    }

    var result []*Container
    for _, ctr := range containers {
        container, err := c.toContainer(ctx, ctr)
        if err != nil {
            log.WithError(err).WithField("id", ctr.ID()).Warn("failed to inspect container")
            continue
        }

        if fn(container) {
            result = append(result, container)
        }
    }

    return result, nil
}

// toContainer converts containerd container to runtime-agnostic model
func (c *containerdClient) toContainer(ctx context.Context, ctr containerd.Container) (*Container, error) {
    info, err := ctr.Info(ctx)
    if err != nil {
        return nil, errors.Wrap(err, "failed to get container info")
    }

    // Get task status (running process)
    state := StateStopped
    var pid int
    task, err := ctr.Task(ctx, nil)
    if err == nil {
        status, err := task.Status(ctx)
        if err == nil {
            state = stateFromContainerdStatus(status.Status)
            pid = int(task.Pid())
        }
    }

    // Get image info
    image, err := ctr.Image(ctx)
    var imageName, imageID string
    if err == nil {
        imageName = image.Name()
        imageID = image.Target().Digest.String()
    }

    networkNSPath := ""
    if pid > 0 {
        networkNSPath = fmt.Sprintf("/proc/%d/ns/net", pid)
    }

    return &Container{
        ID:            info.ID,
        Name:          info.ID, // containerd uses ID as name by default
        Namespace:     c.namespace,
        ImageID:       imageID,
        ImageName:     imageName,
        State:         state,
        PID:           pid,
        Labels:        info.Labels,
        Created:       info.CreatedAt,
        NetworkMode:   "", // Determined by runtime spec
        NetworkNSPath: networkNSPath,
        Runtime:       RuntimeContainerd,
        RuntimeData:   &ContainerdRuntimeData{
            Container: ctr,
            Info:      info,
        },
    }, nil
}

// KillContainer sends a signal to the container's task
func (c *containerdClient) KillContainer(ctx context.Context, container *Container, signal string, dryrun bool) error {
    log.WithFields(log.Fields{
        "name":   container.Name,
        "id":     container.ID,
        "signal": signal,
        "dryrun": dryrun,
    }).Info("killing container")

    if dryrun {
        return nil
    }

    ctx = c.withNamespace(ctx)

    ctr, err := c.client.LoadContainer(ctx, container.ID)
    if err != nil {
        return errors.Wrap(err, "failed to load container")
    }

    task, err := ctr.Task(ctx, nil)
    if err != nil {
        return errors.Wrap(err, "failed to get container task")
    }

    sig := parseSignal(signal)
    if err := task.Kill(ctx, sig); err != nil {
        return errors.Wrap(err, "failed to kill container")
    }

    return nil
}

// StopContainer stops the container gracefully
func (c *containerdClient) StopContainer(ctx context.Context, container *Container, timeout int, dryrun bool) error {
    signal := container.StopSignal()
    if signal == "" {
        signal = defaultStopSignal
    }

    log.WithFields(log.Fields{
        "name":    container.Name,
        "id":      container.ID,
        "timeout": timeout,
        "signal":  signal,
        "dryrun":  dryrun,
    }).Info("stopping container")

    if dryrun {
        return nil
    }

    ctx = c.withNamespace(ctx)

    ctr, err := c.client.LoadContainer(ctx, container.ID)
    if err != nil {
        return errors.Wrap(err, "failed to load container")
    }

    task, err := ctr.Task(ctx, nil)
    if err != nil {
        return errors.Wrap(err, "failed to get container task")
    }

    // Send stop signal
    sig := parseSignal(signal)
    if err := task.Kill(ctx, sig); err != nil {
        return errors.Wrap(err, "failed to send stop signal")
    }

    // Wait for container to stop
    exitCh, err := task.Wait(ctx)
    if err != nil {
        return errors.Wrap(err, "failed to wait for task")
    }

    select {
    case <-exitCh:
        return nil
    case <-time.After(time.Duration(timeout) * time.Second):
        // Force kill
        if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
            return errors.Wrap(err, "failed to force kill container")
        }
        <-exitCh
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// PauseContainer pauses the container's task
func (c *containerdClient) PauseContainer(ctx context.Context, container *Container, dryrun bool) error {
    log.WithFields(log.Fields{
        "name":   container.Name,
        "id":     container.ID,
        "dryrun": dryrun,
    }).Info("pausing container")

    if dryrun {
        return nil
    }

    ctx = c.withNamespace(ctx)

    ctr, err := c.client.LoadContainer(ctx, container.ID)
    if err != nil {
        return errors.Wrap(err, "failed to load container")
    }

    task, err := ctr.Task(ctx, nil)
    if err != nil {
        return errors.Wrap(err, "failed to get container task")
    }

    if err := task.Pause(ctx); err != nil {
        return errors.Wrap(err, "failed to pause container")
    }

    return nil
}

// UnpauseContainer resumes the container's task
func (c *containerdClient) UnpauseContainer(ctx context.Context, container *Container, dryrun bool) error {
    log.WithFields(log.Fields{
        "name":   container.Name,
        "id":     container.ID,
        "dryrun": dryrun,
    }).Info("unpausing container")

    if dryrun {
        return nil
    }

    ctx = c.withNamespace(ctx)

    ctr, err := c.client.LoadContainer(ctx, container.ID)
    if err != nil {
        return errors.Wrap(err, "failed to load container")
    }

    task, err := ctr.Task(ctx, nil)
    if err != nil {
        return errors.Wrap(err, "failed to get container task")
    }

    if err := task.Resume(ctx); err != nil {
        return errors.Wrap(err, "failed to resume container")
    }

    return nil
}

// ExecContainer executes a command inside the container
func (c *containerdClient) ExecContainer(ctx context.Context, container *Container, command string, args []string, dryrun bool) error {
    log.WithFields(log.Fields{
        "name":    container.Name,
        "id":      container.ID,
        "command": command,
        "args":    args,
        "dryrun":  dryrun,
    }).Info("exec container")

    if dryrun {
        return nil
    }

    ctx = c.withNamespace(ctx)

    ctr, err := c.client.LoadContainer(ctx, container.ID)
    if err != nil {
        return errors.Wrap(err, "failed to load container")
    }

    task, err := ctr.Task(ctx, nil)
    if err != nil {
        return errors.Wrap(err, "failed to get container task")
    }

    // Create exec process spec
    pspec := &specs.Process{
        Args: append([]string{command}, args...),
        Cwd:  "/",
        User: specs.User{UID: 0, GID: 0}, // root
    }

    execID := fmt.Sprintf("pumba-exec-%d", time.Now().UnixNano())
    process, err := task.Exec(ctx, execID, pspec, cio.NullIO)
    if err != nil {
        return errors.Wrap(err, "failed to create exec")
    }
    defer process.Delete(ctx)

    if err := process.Start(ctx); err != nil {
        return errors.Wrap(err, "failed to start exec")
    }

    exitCh, err := process.Wait(ctx)
    if err != nil {
        return errors.Wrap(err, "failed to wait for exec")
    }

    exitStatus := <-exitCh
    if exitStatus.ExitCode() != 0 {
        return errors.Errorf("exec failed: %s exit code %d", command, exitStatus.ExitCode())
    }

    return nil
}

// RemoveContainer removes the container
func (c *containerdClient) RemoveContainer(ctx context.Context, container *Container, force, links, volumes, dryrun bool) error {
    log.WithFields(log.Fields{
        "name":   container.Name,
        "id":     container.ID,
        "force":  force,
        "dryrun": dryrun,
    }).Info("removing container")

    if dryrun {
        return nil
    }

    ctx = c.withNamespace(ctx)

    ctr, err := c.client.LoadContainer(ctx, container.ID)
    if err != nil {
        return errors.Wrap(err, "failed to load container")
    }

    // Stop task if running and force is set
    if force {
        task, err := ctr.Task(ctx, nil)
        if err == nil {
            task.Kill(ctx, syscall.SIGKILL)
            task.Delete(ctx, containerd.WithProcessKill)
        }
    }

    // Delete container
    if err := ctr.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
        return errors.Wrap(err, "failed to remove container")
    }

    return nil
}

// RestartContainer restarts the container
func (c *containerdClient) RestartContainer(ctx context.Context, container *Container, timeout time.Duration, dryrun bool) error {
    log.WithFields(log.Fields{
        "name":    container.Name,
        "id":      container.ID,
        "timeout": timeout,
        "dryrun":  dryrun,
    }).Info("restarting container")

    if dryrun {
        return nil
    }

    // Stop then start
    if err := c.StopContainer(ctx, container, int(timeout.Seconds()), false); err != nil {
        return errors.Wrap(err, "failed to stop container for restart")
    }

    return c.StartContainer(ctx, container, false)
}

// StartContainer starts the container
func (c *containerdClient) StartContainer(ctx context.Context, container *Container, dryrun bool) error {
    log.WithFields(log.Fields{
        "name":   container.Name,
        "id":     container.ID,
        "dryrun": dryrun,
    }).Info("starting container")

    if dryrun {
        return nil
    }

    ctx = c.withNamespace(ctx)

    ctr, err := c.client.LoadContainer(ctx, container.ID)
    if err != nil {
        return errors.Wrap(err, "failed to load container")
    }

    // Create new task
    task, err := ctr.NewTask(ctx, cio.NullIO)
    if err != nil {
        return errors.Wrap(err, "failed to create task")
    }

    if err := task.Start(ctx); err != nil {
        return errors.Wrap(err, "failed to start task")
    }

    return nil
}

// StopContainerWithID stops a container by ID
func (c *containerdClient) StopContainerWithID(ctx context.Context, containerID string, timeout time.Duration, dryrun bool) error {
    container := &Container{ID: containerID}
    return c.StopContainer(ctx, container, int(timeout.Seconds()), dryrun)
}

// ContainerdRuntimeData holds containerd-specific container data
type ContainerdRuntimeData struct {
    Container containerd.Container
    Info      containerd.ContainerInfo
}

// Helper functions

func stateFromContainerdStatus(status containerd.ProcessStatus) ContainerState {
    switch status {
    case containerd.Running:
        return StateRunning
    case containerd.Paused:
        return StatePaused
    case containerd.Stopped:
        return StateStopped
    case containerd.Created:
        return StateCreated
    default:
        return StateUnknown
    }
}

func parseSignal(signal string) syscall.Signal {
    // Map signal names to syscall.Signal
    signals := map[string]syscall.Signal{
        "SIGTERM": syscall.SIGTERM,
        "SIGKILL": syscall.SIGKILL,
        "SIGHUP":  syscall.SIGHUP,
        "SIGINT":  syscall.SIGINT,
        "SIGUSR1": syscall.SIGUSR1,
        "SIGUSR2": syscall.SIGUSR2,
        // Add more as needed
    }

    if sig, ok := signals[signal]; ok {
        return sig
    }
    return syscall.SIGTERM // default
}
```

### 6. Network Chaos for Containerd

Network chaos operations (netem, iptables) require special handling for containerd:

```go
// pkg/container/containerd_netem.go

package container

import (
    "context"
    "fmt"
    "net"
    "os/exec"
    "time"

    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
)

// NetemContainer injects network emulation for containerd containers
func (c *containerdClient) NetemContainer(
    ctx context.Context,
    container *Container,
    netInterface string,
    netemCmd []string,
    ips []*net.IPNet,
    sports, dports []string,
    duration time.Duration,
    tcimage string,
    pull, dryrun bool,
) error {
    log.WithFields(log.Fields{
        "name":     container.Name,
        "id":       container.ID,
        "command":  netemCmd,
        "duration": duration,
        "dryrun":   dryrun,
    }).Info("running netem on container")

    if dryrun {
        return nil
    }

    // For containerd, we need to enter the network namespace directly
    // or use a sidecar container approach similar to Docker

    if container.NetworkNSPath == "" {
        return errors.New("container network namespace not available")
    }

    // Option 1: Use nsenter to execute tc commands directly
    if tcimage == "" {
        return c.netemWithNsenter(ctx, container, netInterface, netemCmd, ips, sports, dports)
    }

    // Option 2: Use sidecar container (requires creating containerd container)
    return c.netemWithSidecar(ctx, container, netInterface, netemCmd, ips, sports, dports, tcimage, pull)
}

// netemWithNsenter executes tc commands using nsenter
func (c *containerdClient) netemWithNsenter(
    ctx context.Context,
    container *Container,
    netInterface string,
    netemCmd []string,
    ips []*net.IPNet,
    sports, dports []string,
) error {
    nsenterPrefix := []string{
        "nsenter",
        "-t", fmt.Sprintf("%d", container.PID),
        "-n", // enter network namespace
        "--",
    }

    if len(ips) == 0 && len(sports) == 0 && len(dports) == 0 {
        // Simple netem without IP filtering
        tcCmd := append([]string{"tc", "qdisc", "add", "dev", netInterface, "root", "netem"}, netemCmd...)
        cmd := exec.CommandContext(ctx, nsenterPrefix[0], append(nsenterPrefix[1:], tcCmd...)...)
        if output, err := cmd.CombinedOutput(); err != nil {
            return errors.Wrapf(err, "tc command failed: %s", output)
        }
        return nil
    }

    // Complex netem with IP filtering (prio qdisc setup)
    commands := buildNetemCommands(netInterface, netemCmd, ips, sports, dports)
    for _, tcArgs := range commands {
        tcCmd := append([]string{"tc"}, tcArgs...)
        cmd := exec.CommandContext(ctx, nsenterPrefix[0], append(nsenterPrefix[1:], tcCmd...)...)
        if output, err := cmd.CombinedOutput(); err != nil {
            return errors.Wrapf(err, "tc command failed: %s", output)
        }
    }

    return nil
}

// netemWithSidecar uses a sidecar container for tc commands
func (c *containerdClient) netemWithSidecar(
    ctx context.Context,
    container *Container,
    netInterface string,
    netemCmd []string,
    ips []*net.IPNet,
    sports, dports []string,
    tcimage string,
    pull bool,
) error {
    // Create a temporary container that joins the target's network namespace
    // This is similar to Docker's approach but using containerd API

    // Note: This is a complex operation that requires:
    // 1. Pulling the tc image if needed
    // 2. Creating a container spec with the target's network namespace
    // 3. Running tc commands inside that container
    // 4. Cleaning up the sidecar container

    // Implementation would follow the same pattern as docker_client.go
    // but using containerd APIs

    return errors.New("sidecar netem for containerd not yet implemented - use nsenter mode")
}

// StopNetemContainer removes network emulation
func (c *containerdClient) StopNetemContainer(
    ctx context.Context,
    container *Container,
    netInterface string,
    ips []*net.IPNet,
    sports, dports []string,
    tcimage string,
    pull, dryrun bool,
) error {
    log.WithFields(log.Fields{
        "name":   container.Name,
        "id":     container.ID,
        "dryrun": dryrun,
    }).Info("stopping netem on container")

    if dryrun {
        return nil
    }

    if container.NetworkNSPath == "" || container.PID == 0 {
        return errors.New("container network namespace not available")
    }

    nsenterPrefix := []string{
        "nsenter",
        "-t", fmt.Sprintf("%d", container.PID),
        "-n",
        "--",
    }

    // Build cleanup commands
    var commands [][]string
    if len(ips) != 0 || len(sports) != 0 || len(dports) != 0 {
        commands = [][]string{
            {"qdisc", "del", "dev", netInterface, "parent", "1:1", "handle", "10:"},
            {"qdisc", "del", "dev", netInterface, "parent", "1:2", "handle", "20:"},
            {"qdisc", "del", "dev", netInterface, "parent", "1:3", "handle", "30:"},
            {"qdisc", "del", "dev", netInterface, "root", "handle", "1:", "prio"},
        }
    } else {
        commands = [][]string{
            {"qdisc", "del", "dev", netInterface, "root", "netem"},
        }
    }

    for _, tcArgs := range commands {
        tcCmd := append([]string{"tc"}, tcArgs...)
        cmd := exec.CommandContext(ctx, nsenterPrefix[0], append(nsenterPrefix[1:], tcCmd...)...)
        // Ignore errors on cleanup - qdisc may not exist
        cmd.Run()
    }

    return nil
}

// IPTables operations follow similar pattern
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
    // Similar implementation using nsenter for iptables commands
    return errors.New("iptables for containerd not yet implemented")
}

func (c *containerdClient) StopIPTablesContainer(
    ctx context.Context,
    container *Container,
    cmdPrefix, cmdSuffix []string,
    srcIPs, dstIPs []*net.IPNet,
    sports, dports []string,
    image string,
    pull, dryrun bool,
) error {
    return errors.New("iptables for containerd not yet implemented")
}
```

### 7. Stress Testing for Containerd

```go
// pkg/container/containerd_stress.go

package container

import (
    "context"
    "time"

    "github.com/pkg/errors"
    log "github.com/sirupsen/logrus"
)

// StressContainer runs stress tests for containerd containers
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

    // Stress testing with containerd requires:
    // 1. Creating a stress-ng container in the same cgroup
    // 2. This is complex as containerd doesn't expose cgroup manipulation easily

    // Alternative: Use nsenter to run stress-ng in the target's cgroup namespace
    // This requires stress-ng to be available on the host

    return "", nil, nil, errors.New("stress testing for containerd not yet implemented - requires cgroup access")
}
```

---

## CLI Changes

### New Flags

```go
// cmd/main.go

app.Flags = []cli.Flag{
    // Runtime selection (new)
    cli.StringFlag{
        Name:   "runtime, R",
        Usage:  "container runtime to use: docker, containerd",
        Value:  "docker",
        EnvVar: "PUMBA_RUNTIME",
    },

    // Containerd-specific (new)
    cli.StringFlag{
        Name:   "containerd-address",
        Usage:  "containerd daemon socket address",
        Value:  "/run/containerd/containerd.sock",
        EnvVar: "CONTAINERD_ADDRESS",
    },
    cli.StringFlag{
        Name:   "containerd-namespace, namespace",
        Usage:  "containerd namespace",
        Value:  "default",
        EnvVar: "CONTAINERD_NAMESPACE",
    },

    // Existing Docker flags remain unchanged
    cli.StringFlag{
        Name:   "host, H",
        Usage:  "Docker daemon socket to connect to",
        Value:  "unix:///var/run/docker.sock",
        EnvVar: "DOCKER_HOST",
    },
    // ... other existing flags
}
```

### Updated Before Hook

```go
func before(c *cli.Context) error {
    // ... existing logging setup ...

    // Parse runtime type
    runtimeStr := c.GlobalString("runtime")
    runtime, err := container.ParseRuntimeType(runtimeStr)
    if err != nil {
        return err
    }

    // Create appropriate client
    var config *container.ClientConfig

    switch runtime {
    case container.RuntimeDocker:
        tlsCfg, err := tlsConfig(c)
        if err != nil {
            return err
        }
        config = &container.ClientConfig{
            Host:      c.GlobalString("host"),
            TLSConfig: tlsCfg,
        }
    case container.RuntimeContainerd:
        config = &container.ClientConfig{
            Host:      c.GlobalString("containerd-address"),
            Namespace: c.GlobalString("containerd-namespace"),
        }
    }

    // Create client using factory
    chaos.ContainerClient, err = container.NewClient(runtime, config)
    if err != nil {
        return errors.Wrap(err, "could not create container client")
    }

    return nil
}
```

### Global Client Variable

```go
// pkg/chaos/command.go

var (
    // ContainerClient container client instance (renamed from DockerClient)
    ContainerClient container.Client

    // Deprecated: Use ContainerClient instead
    DockerClient container.Client
)
```

---

## File Structure After Refactoring

```
pkg/container/
├── client.go               # Client interface (updated)
├── model.go                # Runtime-agnostic Container model (new)
├── factory.go              # Client factory (new)
├── config.go               # ClientConfig and defaults (new)
├── docker_client.go        # Docker implementation (updated)
├── docker_netem.go         # Docker netem operations (extracted)
├── docker_iptables.go      # Docker iptables operations (extracted)
├── docker_stress.go        # Docker stress operations (extracted)
├── containerd_client.go    # Containerd implementation (new)
├── containerd_netem.go     # Containerd netem operations (new)
├── containerd_iptables.go  # Containerd iptables operations (new)
├── containerd_stress.go    # Containerd stress operations (new)
├── http_client.go          # HTTP client for Docker (unchanged)
├── util.go                 # Filtering utilities (updated)
├── signals.go              # Signal parsing utilities (new)
└── mock_*.go               # Updated mocks
```

---

## Key Differences: Docker vs Containerd

| Aspect | Docker | Containerd |
|--------|--------|------------|
| **Container Model** | Single entity | Container + Task |
| **Process** | Implicit in container | Explicit Task object |
| **Namespace** | None | Required (`default`, `k8s.io`, etc.) |
| **Socket** | `/var/run/docker.sock` | `/run/containerd/containerd.sock` |
| **Container Name** | Human-readable + ID | Usually just ID |
| **Network Namespace** | Via container ID | Via task PID |
| **Kill/Stop** | Direct container ops | Task operations |
| **Exec** | Container exec API | Task exec API |
| **Image Pull** | High-level API | Requires explicit handling |

### Containerd Task Lifecycle

```
Container (metadata) ─── NewTask() ──▶ Task (running process)
         │                                    │
         │                              Start()/Kill()/Pause()
         │                                    │
         │                              ◀── Wait() ───
         │                                    │
         │                              Delete()
         │                                    ▼
         ◀────────────────────────────────────
                Container still exists
```

---

## Implementation Phases

### Phase 1: Core Abstraction (Foundation)
1. Create `model.go` with runtime-agnostic Container
2. Create `factory.go` with runtime factory
3. Create `config.go` with configuration types
4. Update `client.go` interface if needed
5. Add `RuntimeType()` and `Close()` methods

### Phase 2: Docker Refactoring
1. Update `docker_client.go` to use new Container model
2. Create `DockerRuntimeData` for Docker-specific data
3. Add converter functions (`toContainer`)
4. Update tests

### Phase 3: Containerd Core Operations
1. Implement `containerd_client.go`:
   - `ListContainers`
   - `KillContainer`
   - `StopContainer`
   - `PauseContainer` / `UnpauseContainer`
   - `ExecContainer`
   - `RemoveContainer`
   - `RestartContainer`
   - `StartContainer`
2. Add containerd dependency to `go.mod`

### Phase 4: Containerd Network Chaos
1. Implement `containerd_netem.go` using nsenter approach
2. Implement `containerd_iptables.go` using nsenter approach
3. Consider sidecar container approach for image-based tc

### Phase 5: CLI Integration
1. Add runtime selection flags
2. Update `before()` hook
3. Rename `DockerClient` to `ContainerClient`
4. Add backward compatibility alias

### Phase 6: Testing & Documentation
1. Unit tests for containerd client
2. Integration tests with containerd
3. Update README
4. Update examples

---

## Dependencies to Add

```go
// go.mod additions

require (
    github.com/containerd/containerd v1.7.x
    github.com/opencontainers/runtime-spec v1.1.x
)
```

---

## Feature Support Matrix

| Feature | Docker | Containerd | Notes |
|---------|--------|------------|-------|
| List containers | ✅ | ✅ | |
| Kill (signal) | ✅ | ✅ | |
| Stop (graceful) | ✅ | ✅ | |
| Pause/Unpause | ✅ | ✅ | |
| Remove | ✅ | ✅ | |
| Restart | ✅ | ✅ | |
| Exec command | ✅ | ✅ | |
| Netem (basic) | ✅ | ✅ | nsenter approach |
| Netem (tc-image) | ✅ | ⚠️ | Sidecar more complex |
| IPTables | ✅ | ⚠️ | nsenter approach |
| Stress testing | ✅ | ⚠️ | Requires cgroup access |
| Label filtering | ✅ | ✅ | Different filter syntax |
| TLS connection | ✅ | ❌ | containerd uses gRPC |
| Namespace support | ❌ | ✅ | Required for k8s |

Legend: ✅ Full support, ⚠️ Partial/limited, ❌ Not applicable

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking existing Docker users | High | Keep Docker as default, maintain backward compatibility |
| Containerd version differences | Medium | Test with multiple containerd versions, document requirements |
| Network chaos complexity | Medium | Start with nsenter, add sidecar support later |
| Performance overhead | Low | Lazy loading, connection pooling |
| Stress testing limitations | Medium | Document limitations, provide alternatives |

---

## Testing Strategy

### Unit Tests
- Mock containerd client interface
- Test Container model conversion
- Test factory pattern
- Test signal parsing

### Integration Tests
- Test with real containerd daemon
- Test namespace isolation
- Test alongside Docker (both running)

### E2E Tests
- Kill command with containerd
- Pause/unpause with containerd
- Network chaos with containerd

---

## Migration Notes

### For Users
```bash
# Current (Docker - unchanged)
pumba kill nginx

# Explicit Docker
pumba --runtime docker kill nginx

# Containerd
pumba --runtime containerd --namespace default kill nginx

# Containerd with Kubernetes namespace
pumba --runtime containerd --namespace k8s.io kill nginx
```

### Environment Variables
```bash
# Select runtime
export PUMBA_RUNTIME=containerd

# Containerd settings
export CONTAINERD_ADDRESS=/run/containerd/containerd.sock
export CONTAINERD_NAMESPACE=default
```

---

## Open Questions

1. **Should we support CRI (Container Runtime Interface)?**
   - Pros: Works with any CRI-compliant runtime
   - Cons: Limited operations, mainly for Kubernetes

2. **Should network chaos use nsenter by default for containerd?**
   - Pros: Simpler, no image pull needed
   - Cons: Requires tc/iptables on host

3. **How to handle stress testing without Docker socket?**
   - Option A: Require stress-ng on host
   - Option B: Create containerd container in same cgroup (complex)
   - Option C: Mark as unsupported for containerd initially

4. **Should we support Podman?**
   - Podman is Docker-compatible, might work with minimal changes
   - Could be Phase 7

---

## Conclusion

This design provides a clean abstraction layer for supporting multiple container runtimes while maintaining backward compatibility with Docker. The phased implementation approach allows for incremental development and testing, with Docker remaining the default and fully-supported runtime.

The containerd implementation focuses on core chaos operations (kill, stop, pause, exec) with network chaos support via the nsenter approach. More complex features like stress testing may have limitations in the initial release.
