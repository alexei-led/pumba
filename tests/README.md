# Pumba Integration Tests

This directory contains integration tests for Pumba using the [bats-core](https://github.com/bats-core/bats-core) testing framework.

## Overview

These tests verify Pumba functionality by running against real Docker and containerd runtimes.

## Requirements

- Docker engine running
- [bats-core](https://github.com/bats-core/bats-core) installed
- For containerd tests: containerd socket access and `sudo` (overlayfs mounts for sidecar containers)
- On macOS: [Colima](https://github.com/abrahmsas/colima) VM provides both Docker and containerd sockets

## Test Structure

- `test_helper.bash`: Helper functions (container creation, cleanup, assertions, wait loops)

### Docker Tests

- `kill.bats`: Kill command (default signal, SIGTERM, `--limit`)
- `stop.bats`: Stop command (default, signal handling, custom timeout)
- `pause.bats`: Pause/unpause with duration
- `restart.bats`: Restart (running, stopped, with timeout)
- `remove.bats`: Remove (running, stopped, regex matching)
- `exec.bats`: Exec command (dry-run, custom command, arguments, limit, real exec)
- `stress.bats`: Stress testing (help, non-existent container, CPU stress, dry-run)
- `netem.bats`: Network emulation basics (delay, loss, rate)
- `netem_extended.bats`: Advanced netem (duplicate, corrupt, loss models, distributions)
- `iptables.bats`: iptables packet loss (random, nth, source/destination filters, ports)
- `combined_chaos.bats`: Combined netem + iptables scenarios
- `multi_container.bats`: Regex targeting, label filtering, random selection, interval mode
- `global.bats`: Help output, version flag
- `global_params.bats`: Global flags (--label, --random, --dry-run, --interval, --json, --log-level, --skip-error)
- `error_handling.bats`: Invalid inputs, missing arguments, edge cases

### Containerd Tests

- `containerd_lifecycle.bats`: Kill (by ID, SIGTERM), restart (with timeout), exec
- `containerd_netem.bats`: netem via containerd (delay, loss, duplicate, corrupt, rate)
- `containerd_iptables.bats`: iptables via containerd (random, nth mode, source IP filter)
- `containerd_stop_pause_remove.bats`: Stop, pause/unpause, remove (Docker-managed and pure containerd)
- `containerd_sidecar.bats`: Sidecar container approach (--tc-image) for containers without tc tools
- `containerd_stress.bats`: Stress testing via containerd runtime
- `containerd_global_flags.bats`: Global flags via containerd (dry-run, regex, --random, --label)
- `skip_ci/stress.bats`: Docker stress test (may be skipped in CI)

### Other Files

- `run_tests.sh`: Script to run all tests and generate a report

## Running Tests

### All tests via Colima VM (recommended on macOS)

Colima provides native Docker + containerd sockets. Running with `sudo` gives overlayfs permissions needed for containerd sidecar tests.

```bash
# Run ALL tests (Docker + containerd) inside Colima VM
colima ssh -- sudo bats tests/*.bats tests/containerd_*.bats

# Run only Docker tests
colima ssh -- sudo bats tests/kill.bats tests/stop.bats tests/pause.bats tests/restart.bats \
  tests/remove.bats tests/exec.bats tests/stress.bats tests/netem.bats tests/netem_extended.bats \
  tests/iptables.bats tests/multi_container.bats tests/global.bats tests/global_params.bats \
  tests/combined_chaos.bats tests/error_handling.bats

# Run only containerd tests
colima ssh -- sudo bats tests/containerd_*.bats
```

### Using make (CI)

```bash
# Build Docker test image and run core integration tests
make integration-tests

# Build and run all tests including stress tests
make integration-tests-all
```

### Using bats directly

```bash
# Run a specific test file
bats tests/stop.bats

# Run all Docker test files
bats tests/*.bats

# Run containerd tests (requires sudo for netem/iptables)
sudo bats tests/containerd_*.bats
```

### Using the test runner

```bash
# Run all tests (excluding stress tests)
./tests/run_tests.sh

# Run all tests including stress tests
./tests/run_tests.sh --all
```

## Test Reports

The test runner generates a report at `/tmp/pumba_test_report.txt` that includes:

- Results of each test file
- Pass/fail status for each test
- Summary of overall test results

## Writing New Tests

When writing new tests, please follow these practices:

1. Use the existing structure as a template
2. Include setup and teardown functions to clean up containers
3. Use helper functions from `test_helper.bash` when possible
4. Follow the Given-When-Then pattern in test comments
5. Make sure tests properly clean up after themselves

### Example Test Structure

```bash
#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "test_pattern"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "test_pattern"
}

@test "Should do something specific" {
    # Given (some precondition)
    create_test_container "test_container"

    # When (some action is performed)
    run pumba some_command test_container

    # Then (expected outcome)
    [ $status -eq 0 ]

    # And (more assertions)
    run docker inspect -f {{.State.Status}} test_container
    [ "$output" = "expected_status" ]
}
```

## Troubleshooting

If tests are failing, check:

1. Docker engine is running (`docker ps`)
2. Pumba binary is in your PATH (`pumba --version`)
3. No leftover containers from previous test runs (`docker ps -a`)
4. For containerd tests: containerd socket is accessible (`sudo ctr version`)
5. For containerd netem/iptables: pumba must run as root (`sudo pumba ...`) — overlayfs mounts for sidecar containers require root
6. Container PID 1 and signals: `sleep`/`tail -f /dev/null` as PID 1 ignores SIGTERM — use `top` in tests that send SIGTERM
7. iptables flag ordering: `--source`, `--destination`, `--src-port`, `--dst-port` go on the `iptables` parent command, before `loss`
8. exec argument parsing: use `--command "touch" --args "/tmp/file"`, not `--command "touch /tmp/file"`
