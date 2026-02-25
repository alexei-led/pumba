#!/usr/bin/env bash
# Test helpers for Pumba bats integration tests

# Load bats-support and bats-assert libraries
load "/usr/local/lib/bats-support/load.bash"
load "/usr/local/lib/bats-assert/load.bash"

# Create a test container with the given name
create_test_container() {
  local name=$1
  local image=${2:-alpine:latest}
  local command=${3:-"tail -f /dev/null"}
  echo "Creating test container: $name (using $image)"
  # shellcheck disable=SC2086
  docker run -d --name "$name" "$image" $command
  sleep 1 # Give container time to start
}

# Create containers with labels
create_labeled_containers() {
  local name_prefix=$1
  local label_key=$2
  local label_value=$3
  local count=${4:-2}

  for i in $(seq 1 "$count"); do
    echo "Creating labeled container: ${name_prefix}_${i} with ${label_key}=${label_value}"
    docker run -d --name "${name_prefix}_${i}" --label "${label_key}=${label_value}" alpine tail -f /dev/null
  done
  sleep 1 # Give containers time to start
}

# Remove test containers matching a pattern
cleanup_containers() {
  local pattern=$1
  echo "Cleaning up containers matching: $pattern"
  docker ps -a --filter "name=$pattern" -q | xargs -r docker rm -f || true
}

# Assert container state
assert_container_state() {
  local container=$1
  local expected_state=$2

  local actual_state
  if ! actual_state=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null); then
    echo "Container '$container' does not exist"
    return 1
  fi

  if [[ "$expected_state" != "$actual_state" ]]; then
    echo "Expected container '$container' to be in state '$expected_state', but was '$actual_state'"
    return 1
  fi

  return 0
}

# Wait for a condition to be true
wait_for() {
  local timeout=${1:-10}
  local check_command=$2
  local description=${3:-"condition"}

  echo "Waiting up to ${timeout}s for ${description}..."

  local counter=0
  until eval "$check_command"; do
    counter=$((counter + 1))
    if [[ "$counter" -gt "$timeout" ]]; then
      echo "Timed out waiting for ${description}"
      return 1
    fi
    sleep 1
  done

  return 0
}

# ── Container state assertion wrappers ──────────────────────────────────────

assert_container_running() {
  local container=$1
  local actual
  actual=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null) || {
    fail "Container '$container' does not exist"
  }
  if [[ "$actual" != "running" ]]; then
    fail "Expected container '$container' to be running, but was '$actual'"
  fi
}

assert_container_exited() {
  local container=$1
  local actual
  actual=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null) || {
    fail "Container '$container' does not exist"
  }
  if [[ "$actual" != "exited" ]]; then
    fail "Expected container '$container' to be exited, but was '$actual'"
  fi
}

assert_container_paused() {
  local container=$1
  local actual
  actual=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null) || {
    fail "Container '$container' does not exist"
  }
  if [[ "$actual" != "paused" ]]; then
    fail "Expected container '$container' to be paused, but was '$actual'"
  fi
}

assert_container_removed() {
  local container=$1
  if docker inspect "$container" &>/dev/null; then
    local actual
    actual=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null)
    fail "Expected container '$container' to be removed, but it exists with status '$actual'"
  fi
}

# ── Network emulation (netem) assertion helpers ─────────────────────────────

# Get the PID of a Docker container for nsenter
_docker_container_pid() {
  docker inspect -f '{{.State.Pid}}' "$1"
}

# Assert netem rules are applied to a container's network interface
# Usage: assert_netem_applied <container> <expected_pattern> [interface]
assert_netem_applied() {
  local container=$1
  local expected_pattern=$2
  local interface=${3:-eth0}

  local pid
  pid=$(_docker_container_pid "$container") || {
    fail "Cannot get PID for container '$container'"
  }

  local tc_output
  tc_output=$(nsenter -t "$pid" -n tc qdisc show dev "$interface" 2>&1) || {
    fail "Failed to run tc qdisc show in container '$container' netns (pid=$pid, dev=$interface)"
  }

  if ! echo "$tc_output" | grep -qi "$expected_pattern"; then
    fail "Expected netem pattern '$expected_pattern' in tc output for '$container' ($interface), got:
$tc_output"
  fi
}

# Assert netem rules have been cleaned up from a container
# Usage: assert_netem_cleaned <container> [interface]
assert_netem_cleaned() {
  local container=$1
  local interface=${2:-eth0}

  local pid
  pid=$(_docker_container_pid "$container") || {
    fail "Cannot get PID for container '$container'"
  }

  local tc_output
  tc_output=$(nsenter -t "$pid" -n tc qdisc show dev "$interface" 2>&1) || true

  if echo "$tc_output" | grep -qi "netem"; then
    fail "Expected netem rules to be cleaned from '$container' ($interface), but found:
$tc_output"
  fi
}

# ── iptables assertion helpers ──────────────────────────────────────────────

# Assert iptables DROP rules are applied to a container
# Usage: assert_iptables_applied <container> <expected_pattern>
assert_iptables_applied() {
  local container=$1
  local expected_pattern=$2

  local pid
  pid=$(_docker_container_pid "$container") || {
    fail "Cannot get PID for container '$container'"
  }

  local ipt_output
  ipt_output=$(nsenter -t "$pid" -n iptables -L INPUT -n -v 2>&1) || {
    fail "Failed to run iptables in container '$container' netns (pid=$pid)"
  }

  if ! echo "$ipt_output" | grep -qi "$expected_pattern"; then
    fail "Expected iptables pattern '$expected_pattern' in container '$container', got:
$ipt_output"
  fi
}

# Assert iptables DROP rules have been cleaned from a container
# Usage: assert_iptables_cleaned <container>
assert_iptables_cleaned() {
  local container=$1

  local pid
  pid=$(_docker_container_pid "$container") || {
    fail "Cannot get PID for container '$container'"
  }

  local ipt_output
  ipt_output=$(nsenter -t "$pid" -n iptables -L INPUT -n -v 2>&1) || true

  if echo "$ipt_output" | grep -qi "DROP"; then
    fail "Expected iptables DROP rules to be cleaned from '$container', but found:
$ipt_output"
  fi
}

# ── Sidecar cleanup assertion ──────────────────────────────────────────────

# Assert no sidecar containers matching a pattern remain
# Usage: assert_sidecar_cleaned [pattern]
assert_sidecar_cleaned() {
  local pattern=${1:-"pumba-nettools"}

  local remaining
  remaining=$(docker ps -q --filter "ancestor=ghcr.io/alexei-led/$pattern" 2>/dev/null)

  if [[ -n "$remaining" ]]; then
    fail "Expected no sidecar containers matching '$pattern', but found:
$(docker ps --filter "ancestor=ghcr.io/alexei-led/$pattern" --format '{{.Names}}')"
  fi
}

# ── Docker-in-Docker detection ────────────────────────────────────────────

# Skip test if running inside Docker-in-Docker (sidecar container creation
# with --net=container:X fails in DinD due to PID namespace limitations)
skip_if_dind() {
  if [ -f /.dockerenv ]; then
    skip "sidecar tests not supported in Docker-in-Docker"
  fi
}

# ── Containerd helpers ────────────────────────────────────────────────────

# Check if containerd socket is available and skip test if not
require_containerd() {
  if [ ! -S /run/containerd/containerd.sock ]; then
    skip "containerd socket not available"
  fi
}

# Pull an image into containerd with retry logic
# Usage: ctr_pull_image <namespace> <image>
ctr_pull_image() {
  local ns=$1
  local image=$2
  local retries=3
  local i

  for i in $(seq 1 "$retries"); do
    if sudo ctr -n "$ns" i ls -q | grep -q "^${image}$"; then
      echo "Image $image already present in $ns namespace"
      return 0
    fi
    echo "Pulling $image into $ns namespace (attempt $i/$retries)..."
    if sudo ctr -n "$ns" i pull "$image" >/dev/null 2>&1; then
      return 0
    fi
    [ "$i" -lt "$retries" ] && sleep 2
  done

  echo "Failed to pull $image after $retries attempts"
  return 1
}

# ── Nettools image helper ──────────────────────────────────────────────────

ensure_nettools_image() {
  echo "Ensuring nettools image is available..."

  NETTOOLS_IMAGE="ghcr.io/alexei-led/pumba-alpine-nettools:latest"

  if [ "${CI:-}" = "true" ]; then
    echo "CI environment detected, using pumba-alpine-nettools:local"
    if ! docker image inspect pumba-alpine-nettools:local &>/dev/null; then
      echo "Creating local nettools image for testing..."
      docker run --name temp-nettools-container alpine:latest /bin/sh -c "apk add --no-cache iproute2 iptables && echo 'Nettools container ready'"
      docker commit temp-nettools-container pumba-alpine-nettools:local
      docker rm -f temp-nettools-container
    fi
    NETTOOLS_IMAGE="pumba-alpine-nettools:local"
  elif ! docker image inspect ${NETTOOLS_IMAGE} &>/dev/null; then
    echo "Pulling nettools image..."
    if ! docker pull ${NETTOOLS_IMAGE}; then
      echo "Failed to pull image, creating local nettools image for testing..."
      docker run --name temp-nettools-container alpine:latest /bin/sh -c "apk add --no-cache iproute2 iptables && echo 'Nettools container ready'"
      docker commit temp-nettools-container pumba-alpine-nettools:local
      docker rm -f temp-nettools-container
      NETTOOLS_IMAGE="pumba-alpine-nettools:local"
    fi
  else
    echo "Nettools image already exists locally"
  fi

  export NETTOOLS_IMAGE
}
