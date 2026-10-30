# Pumba User Guide

Pumba is a chaos testing command line tool for Docker, containerd, and Podman containers. This guide covers container chaos commands, targeting, and general usage. For network chaos (netem, iptables), see [Network Chaos](network-chaos.md). For stress testing, see [Stress Testing](stress-testing.md).

## Prerequisites

- **Docker runtime** (default): Docker `v18.06.0` or later
- **containerd runtime**: containerd `v2.0` or later
- **Podman runtime**: Podman `v5.x` or later with a Docker-compatible socket (rootful for network/stress chaos)
- Download the Pumba binary for your OS from the [releases page](https://github.com/alexei-led/pumba/releases), or run it as a [Docker container](deployment.md)

## Runtime Selection

Pumba supports three container runtimes: **Docker** (default), **containerd**, and **Podman**.

### Docker Runtime (default)

No extra flags needed — Pumba uses the Docker API by default:

```bash
pumba kill mycontainer
```

### containerd Runtime

Use `--runtime containerd` to target containerd directly. This is useful for Kubernetes environments where containers are managed by containerd without Docker:

```bash
# Target containers in the Kubernetes namespace
pumba --runtime containerd --containerd-namespace k8s.io kill <container-id>

# Target Docker-managed containers via containerd
pumba --runtime containerd --containerd-namespace moby kill <container-id>
```

**Global flags for containerd:**

| Flag                     | Default                           | Description                                  |
| ------------------------ | --------------------------------- | -------------------------------------------- |
| `--runtime`              | `docker`                          | Container runtime (`docker` or `containerd`) |
| `--containerd-socket`    | `/run/containerd/containerd.sock` | containerd socket path                       |
| `--containerd-namespace` | `k8s.io`                          | containerd namespace                         |

**Container name resolution:**

Pumba resolves container names from well-known labels (checked in priority order):

1. **Kubernetes**: `io.kubernetes.container.name` → `namespace/pod/container`
2. **nerdctl**: `nerdctl/name`
3. **Docker Compose**: `com.docker.compose.service`
4. **Fallback**: container ID

When using `re2:` regex patterns with the containerd runtime, Kubernetes container names use the `namespace/pod/container` format. Example: `re2:^default/` matches all containers in the `default` namespace.

**Sidecar container for network chaos:**

By default, Pumba executes `tc`/`iptables` commands directly inside the target container. Netem requires both a POSIX `sh` and `tc` for ownership-safe qdisc changes. If the target doesn't have these tools, use `--tc-image` to spawn a sidecar container that shares the target's network namespace. Custom netem sidecar images must include `sh`, `grep`, and `tc`; Docker/Podman also require `which` and `tail`, while containerd requires `sleep`:

```bash
# Sidecar mode — works even if the target has no tc tools
pumba --runtime containerd netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --duration 5m delay --time 3000 <container-id>
```

**Known limitations of the containerd runtime:**

- **Stress testing**: executes `stress-ng` directly inside the target container — the container image must include `stress-ng`. The `--stress-image` and `--inject-cgroup` flags are ignored with the containerd runtime
- **Remove** (`rm`): For Docker-managed containers in the `moby` namespace, kills the task but Docker retains its own metadata

### Podman Runtime

Use `--runtime podman` to target containers managed by Podman. Pumba reuses the Docker SDK against Podman's Docker-compat socket — every chaos command (`kill`, `stop`, `pause`, `rm`, `restart`, `exec`, `netem`, `iptables`, `stress`) works the same as with Docker:

```bash
# Kill a container by name (auto-detected socket)
sudo pumba --runtime podman kill mycontainer

# Add network delay (requires rootful Podman)
sudo pumba --runtime podman netem --duration 5m delay --time 3000 mycontainer

# Stress CPU using sidecar (child-cgroup mode)
sudo pumba --runtime podman stress --duration 60s \
  --stressors="--cpu 4 --timeout 60s" mycontainer

# Inject stress-ng into the target's cgroup (shared resource accounting)
sudo pumba --runtime podman stress --duration 60s --inject-cgroup \
  --stress-image ghcr.io/alexei-led/stress-ng:latest \
  --stressors="--cpu 1 --timeout 5s" mycontainer

# Explicit socket override
pumba --runtime podman --podman-socket unix:///run/podman/podman.sock kill mycontainer
```

**Global flags for Podman:**

| Flag              | Default           | Description                                                                                          |
| ----------------- | ----------------- | ---------------------------------------------------------------------------------------------------- |
| `--runtime`       | `docker`          | Set to `podman` to target Podman                                                                     |
| `--podman-socket` | _(auto-detected)_ | Podman socket URI (e.g. `unix:///run/podman/podman.sock`); empty triggers auto-detection (see below) |

**Socket auto-detection order:**

1. `$CONTAINER_HOST` environment variable
2. `$PODMAN_SOCK` environment variable
3. `podman machine inspect` (macOS — picks up the active machine's socket)
4. `/run/podman/podman.sock` (Linux rootful default)
5. `$XDG_RUNTIME_DIR/podman/podman.sock` (Linux rootless default — **not supported** for netem/iptables/stress)

**Rootful requirement:**

`netem`, `iptables`, and `stress` require **rootful Podman**. Rootless is detected at `NewClient` time (via `Info.SecurityOptions`) and each of these commands fails fast with an actionable error pointing to:

- macOS: `podman machine set --rootful` (then `podman machine start`)
- Linux: the rootful `podman.socket` systemd unit (`systemctl --system enable --now podman.socket`)

Lifecycle commands (`kill`, `stop`, `pause`, `rm`, `restart`, `exec`) work in both rootful and rootless mode.

**macOS development with `podman machine`:**

Podman on macOS runs containers inside a Linux VM. Stress cgroup resolution reads `/proc/<pid>/cgroup` from the **host's view** (Podman's default `cgroupns=private` hides ancestry from inside the container), so pumba must run on the same kernel as the target containers. Run the pumba binary **inside** the `podman machine` VM, mirroring the containerd-in-colima pattern:

```bash
# one-time setup
brew install podman
podman machine init --rootful --cpus 4 --memory 4096 --now

# copy a linux/arm64 or linux/amd64 pumba binary into the VM
podman machine cp /path/to/pumba podman-machine-default:/tmp/pumba
podman machine ssh sudo install -m 0755 /tmp/pumba /usr/local/bin/pumba

# run inside the VM
podman machine ssh sudo pumba --runtime podman --log-level debug ps
podman machine ssh sudo pumba --runtime podman netem --duration 10s delay --time 200 <container-id>
```

**Known differences from the Docker runtime:**

- **Stress testing**: always runs `stress-ng` in a sidecar (never execs inside the target). `--stress-image` is required; `--inject-cgroup` routes via `/cg-inject` for shared accounting with the target.
- **Sidecar cgroup placement**: Podman uses `libpod-<id>.scope/container` for the leaf cgroup under systemd, vs Docker's `docker-<id>.scope`. Pumba detects the nested `container/` sub-cgroup automatically for inject-cgroup mode (cgroup v2 "no internal processes" rule).
- **Rootless is unsupported** for network/stress chaos. The slirp4netns/pasta netns setup and user-namespace cgroup math are out of scope; use rootful mode.

## Container Targeting

Pumba supports several ways to select which containers to affect.

### By Name

Specify one or more container names directly:

```bash
# Single container
pumba kill mydb

# Multiple containers (space-separated)
pumba kill container1 container2 container3
```

### By Regex

Use the `re2:` prefix to match container names with [RE2 regular expressions](https://github.com/google/re2/wiki/Syntax):

```bash
# All containers whose names start with "test"
pumba kill "re2:^test"

# All containers with "api" in the name
pumba kill "re2:api"

# Combine multiple regexes and exact names (all selectors use OR logic)
pumba kill "re2:^web-" database "re2:^api-"
```

### By Labels

Use `--label` to filter containers by Docker labels. Multiple labels can be specified and all must match:

```bash
# Containers with a specific label
pumba --label app=web kill

# Multiple labels (AND logic)
pumba --label app=web --label env=staging kill
```

### Random Selection

Use `--random` (or `-r`) to randomly select a single container from all matching targets:

```bash
# Kill one random container matching the regex
pumba --random kill "re2:^test"
```

## Container Chaos Commands

Each command targets containers using the [targeting methods](#container-targeting) described above. Run `pumba <command> --help` for the full list of options. The `kill`, `stop`, and `rm` commands require at least one container argument (name, list of names, or RE2 regex).

### kill

Send a termination signal to the main process inside target containers.

```bash
# Kill with default SIGKILL
pumba kill mydb

# Kill with a specific signal
pumba kill --signal SIGTERM mydb

# Limit to killing only 2 containers
pumba kill --limit 2 "re2:^test"
```

### stop

Stop the main process inside target containers by sending SIGTERM, then SIGKILL after a grace period.

```bash
# Stop with default 5-second grace period
pumba stop myapp

# Stop with a 30-second grace period
pumba stop --time 30 myapp
```

### pause

Pause all running processes within target containers for a specified duration.

```bash
# Pause for 5 seconds
pumba pause --duration 5s myapp
```

### rm

Remove target containers, including stopped ones. By default, force-kills running containers and removes associated volumes.

```bash
# Force remove (default)
pumba rm myapp

# Remove without force, keeping volumes
pumba rm --force=false --volumes=false myapp
```

### restart

Restart target containers.

```bash
pumba restart myapp
```

### exec

Execute a command inside target containers.

```bash
# Execute default command (kill 1)
pumba exec web

# Execute a custom command with arguments
pumba exec --command "echo" --args "hello" web

# Multiple arguments using repeated --args flags
pumba exec --command "ls" --args "-la" --args "/etc" "re2:^api.*"

# Limit execution to 2 containers
pumba exec --command "touch" --args "/tmp/test-file" --limit 2 "re2:.*"
```

## Recurring Chaos

Use `--interval` (or `-i`) to run chaos commands on a recurring schedule:

```bash
# Kill a random matching container every 30 seconds
pumba --interval 30s --random kill "re2:^test"

# Pause a container for 5s every minute
pumba --interval 1m pause --duration 5s myapp
```

The interval supports time unit suffixes: `ms`, `s`, `m`, `h`.

When using `--interval` with commands that have a `--duration` (like `pause` or `netem`), the duration must be shorter than the interval.

## Dry Run Mode

Use `--dry-run` to see what Pumba would do without actually creating chaos:

```bash
pumba --dry-run kill "re2:^test"
```

This logs the planned actions without executing them. Useful for verifying your targeting before running for real.
Dry-run returns right away; it does not wait for `--duration`.

## Logging

### Log Level

Control verbosity with `--log-level` (or `-l`). Available levels: `debug`, `info`, `warning` (default), `error`, `fatal`, `panic`.

```bash
pumba --log-level info kill myapp
```

### JSON Logging

Use `--json` (or `-j`) to produce JSON-formatted logs, compatible with Logstash and Splunk:

```bash
pumba --json --log-level info kill myapp
```

## Structured Output

Use `--output ndjson` (or `-o ndjson`) to get result events on stdout, one JSON object per line.
Logs stay on stderr, so stdout can be piped straight into `jq`:

```bash
pumba -o ndjson --run-id nightly-7 pause --duration 10s myapp 2>/dev/null | jq -c '{type, status}'
```

Each run emits `run.start`, then `target.applied` (fault active) and `target.result` (after revert)
per target, and finally `run.end` with the overall `status` (`ok`, `failed`, `partial`,
`cleanup_failed`, `no_targets`, `refused`, `usage_error`, `aborted`). Every event carries the run ID
from `--run-id`, or a generated one.

Exit codes are unchanged: `0` on success, `1` on error, in both modes. A failed cleanup (netem or iptables rules left in place)
exits `1`.

`--max-duration` bounds a run in time: when it expires, in-flight chaos is cleaned up and the run
exits `1`. It does not undo `kill`, `rm`, or `stop` without `--restart`, and it cannot help if Pumba
itself is killed.

```bash
pumba --max-duration 1h --interval 5m --random kill 're2:^worker-'
```

The full contract (fields, error codes, delivery, recipes) is in [Agents](agents.md).

## Finding Leftovers

A run that dies before its cleanup (`kill -9`, OOM, node loss) leaves its faults in place.
Pumba tags what it creates with the run ID (`--run-id`, or a generated one):

- every sidecar (netem, iptables, stress, and status's own inspect sidecar) carries the labels
  `com.gaiaadm.pumba.run-id`, `com.gaiaadm.pumba.target` (target container ID),
  `com.gaiaadm.pumba.kind` and `com.gaiaadm.pumba.skip=true`;
- every iptables rule carries `-m comment --comment pumba:<run-id>`.

`pumba status` reports, per selected running container (default: all running):

- iptables rules with a `pumba:` comment;
- qdiscs under Pumba's root handle `504d:`, as `pumba-owned` (the topology Pumba installs) or
  `suspicious` (the handle, but not Pumba's topology). qdiscs have no comment field, so they
  are never attributed to a run;
- Pumba sidecars labeled with the container's ID. Sidecars whose target was not inspected
  (gone or stopped) are reported as orphans only when nothing is selected: no names, no `re2:`
  pattern and no `--label`.

```bash
pumba status                         # everything Pumba left on running containers
pumba status --run-id nightly-7 web  # only rules and sidecars of run nightly-7 (qdiscs always shown)
pumba status --check web             # prints a verdict; exit 0 only when clean
```

`status --run-id` filters; the global `--run-id` only names the status run itself.

| Verdict | When | Exit |
|---|---|---|
| `clean` | every selected container was inspected and nothing matched | 0 |
| `dirty` | a tagged rule or labeled sidecar matched; without `--run-id`, also a `pumba-owned` qdisc | 1 |
| `unknown` | an inspection failed, a named container is missing, a container could not be read, a `suspicious` qdisc, nothing selected, or (with `--run-id`) only qdiscs found | 1 |

`dirty` wins over `unknown`. `--check` cannot be combined with `--interval`; `--random` and
`--dry-run` are ignored; `--max-targets` applies.

Status changes no fault. It inspects each container's network namespace in a short-lived
sidecar (`--tc-image`, needs `tc` and `iptables`), or with `--tc-image ""` by exec in the
container, which then needs both tools. On Podman it needs rootful mode, like netem and iptables.

To clean up by hand, delete what status lists, for example:

```bash
pid=$(docker inspect -f '{{.State.Pid}}' web)
sudo nsenter -t "$pid" -n tc qdisc del dev eth0 root
sudo nsenter -t "$pid" -n iptables -D INPUT -i eth0 -m comment --comment pumba:nightly-7 ...  # status spec, -A → -D
docker rm -f $(docker ps -aq --filter label=com.gaiaadm.pumba.run-id=nightly-7)
```

Limits: iptables rules from Pumba 1.4 and earlier have no comment and are not found; containerd
stress with `--stress-image ""` runs stress-ng by exec in the target and leaves no container to
find; containers sharing a network namespace report the same rules and qdiscs.

## Slack Integration

Pumba can send log events to a Slack channel via incoming webhooks:

```bash
pumba --slackhook https://hooks.slack.com/services/T.../B.../xxx \
      --slackchannel "#chaos-alerts" \
      kill myapp
```

- `--slackhook` - Slack incoming webhook URL
- `--slackchannel` - Slack channel (default: `#pumba`)

Delivery is best effort and asynchronous, with a 5-second HTTP timeout and at most
16 concurrent requests. Events beyond that limit are dropped instead of delaying
chaos. Normal shutdown waits up to 5 seconds for pending sends; abrupt exits can
lose them. Delivery failures print one warning to stderr, without exposing the
webhook URL or response body.


## TLS Configuration

When connecting to a remote Docker daemon over TLS:

```bash
pumba --tls \
      --tlscacert /path/to/ca.pem \
      --tlscert /path/to/cert.pem \
      --tlskey /path/to/key.pem \
      --host tcp://remote-docker:2376 \
      kill myapp
```

Use `--tlsverify` to additionally verify the remote server certificate.

## Error Handling

Use `--skip-error` to continue running on the next interval tick even if the chaos command fails:

```bash
pumba --interval 30s --skip-error kill "re2:^test"
```

This is useful in recurring mode when target containers may temporarily not exist.

## Further Reading

- [Network Chaos](network-chaos.md) - netem and iptables commands
- [Stress Testing](stress-testing.md) - CPU, memory, and I/O stress (includes `--inject-cgroup` for same-cgroup injection)
- [Deployment](deployment.md) - Docker, Kubernetes, and OpenShift
- [Agents](agents.md) - Result events, run ID, `status --check`, exit codes and run budget for scripts and agents
- [Contributing](../CONTRIBUTING.md) - Building and contributing to Pumba

## Global safety options

`--max-targets N` refuses to run when the number of selected containers exceeds
N, before injecting any fault. The default is **unset** (`0`): there is no cap.
Negative values are rejected. The count is evaluated after name, regex and label
selection and after `--limit` and `--random` narrowing. With `--random`, only one
container is counted. Dry-run also refuses over-cap selections. With `--interval`,
the cap is checked again on every tick. A safety refusal exits non-zero even with
`--skip-error`.

```bash
pumba --max-targets 3 --dry-run kill 're2:^web-'
```

## Protecting containers

Set the container label `com.gaiaadm.pumba.skip=true` to exclude a container from
Pumba selection, including explicit names, regexes, and label selectors. The value
must be the string `true`; merely adding the key is not enough.

```bash
docker run -d --name protected --label com.gaiaadm.pumba.skip=true alpine sleep 3600
```

For Kubernetes, a Pod template label can express the same protection intent:

```yaml
spec:
  template:
    metadata:
      labels:
        com.gaiaadm.pumba.skip: "true"
```

**Kubernetes limitation:** Pumba reads labels on the runtime container, not Pod
metadata. A Pod label alone is not a guarantee of protection: verify that your
runtime exposes `com.gaiaadm.pumba.skip=true` on each container. Otherwise use
explicit target allow-lists; do not rely on this Pod label to exclude containers.
Pumba does not translate Kubernetes labels into runtime labels.

Use `--max-targets` as an additional opt-in safety cap, and `--dry-run` to inspect
selection before injecting faults. The cap does not replace protection labels.
