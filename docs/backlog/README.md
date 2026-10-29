# Backlog

Deferred ideas, one file per item. Not commitments.

- Format: title, tags, why, approach, open questions, effort (S/M/L).
- Lifecycle: delete the file when the work ships; open a GitHub issue when an item is decided or ready for a contributor.

| File | Tag | Summary |
|---|---|---|
| [agent-plan-schema-mcp.md](agent-plan-schema-mcp.md) | agent | Machine-readable plan, schema, MCP server |
| [agent-structured-output.md](agent-structured-output.md) | agent | Structured result output |
| [arch-otel-sidecar-images.md](arch-otel-sidecar-images.md) | arch | OpenTelemetry spans and sidecar image consolidation |
| [chaos-experiments-probes.md](chaos-experiments-probes.md) | chaos, agent | Experiment files, steady-state probes, scenario library |
| [chaos-time-and-syscalls.md](chaos-time-and-syscalls.md) | chaos | Clock skew and syscall fault injection |
| [container-chaos-extras.md](container-chaos-extras.md) | container | Container chaos extras |
| [devices-disk-pids-gpu.md](devices-disk-pids-gpu.md) | chaos, container | More devices: disk, PIDs/FDs, GPU |
| [fault-lifecycle-run-id.md](fault-lifecycle-run-id.md) | arch, agent, safety | Fault lifecycle, run ID, status/revert/verify-clean |
| [host-chaos.md](host-chaos.md) | host, safety | Host chaos |
| [k8s-integration.md](k8s-integration.md) | k8s | Kubernetes integration |
| [network-atomic-and-selectors.md](network-atomic-and-selectors.md) | network, safety | Atomic selector install + AND-combined selectors |
| [network-dns-l7.md](network-dns-l7.md) | network, chaos | DNS and L7 (HTTP/gRPC) faults |
| [network-ingress-partition-tcp.md](network-ingress-partition-tcp.md) | network | Ingress shaping, partitions, TCP faults, profiles |
| [safety-rails.md](safety-rails.md) | safety | Interactive confirmation rejected; duration validation complete |
| [stack-docker-sdk-moby.md](stack-docker-sdk-moby.md) | stack | Docker SDK to moby/moby client modules |
| [stack-logrus-slog.md](stack-logrus-slog.md) | stack | logrus to log/slog |
| [stack-slackrus-webhook.md](stack-slackrus-webhook.md) | stack | Replace slackrus with generic --webhook |
| [stack-urfave-cli-v3.md](stack-urfave-cli-v3.md) | stack, arch | urfave/cli v1 to v3 |
| [stress-first-class-cgroup.md](stress-first-class-cgroup.md) | stress | Stress: cgroup limits, PSI, stats, Podman 4.9 race |
