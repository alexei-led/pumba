# Pumba User Guide

Pumba is a chaos testing command line tool for Docker containers. This guide covers container chaos commands, targeting, and general usage. For network chaos (netem, iptables), see [Network Chaos](network-chaos.md). For stress testing, see [Stress Testing](stress-testing.md).

## Prerequisites

- Docker `v18.06.0` or later
- Download the Pumba binary for your OS from the [releases page](https://github.com/alexei-led/pumba/releases), or run it as a [Docker container](deployment.md)

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

## Slack Integration

Pumba can send log events to a Slack channel via incoming webhooks:

```bash
pumba --slackhook https://hooks.slack.com/services/T.../B.../xxx \
      --slackchannel "#chaos-alerts" \
      kill myapp
```

- `--slackhook` - Slack incoming webhook URL
- `--slackchannel` - Slack channel (default: `#pumba`)

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
- [Contributing](../CONTRIBUTING.md) - Building and contributing to Pumba
