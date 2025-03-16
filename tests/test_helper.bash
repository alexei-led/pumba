#!/usr/bin/env bash
# Test helpers for Pumba bats integration tests

# Load bats assertions if available
load_assertions() {
  if [ -f "/usr/local/lib/bats-assert/load.bash" ]; then
    load "/usr/local/lib/bats-assert/load.bash"
  fi
}

# Create a test container with the given name
create_test_container() {
  local name=$1
  local image=${2:-alpine:latest}
  local command=${3:-"tail -f /dev/null"}
  echo "Creating test container: $name (using $image)"
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
  actual_state=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null)
  
  # Check if container exists first
  if [[ $? -ne 0 ]]; then
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