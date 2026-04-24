#!/usr/bin/env bats

# Exec tests for the podman runtime, mirroring tests/exec.bats. Uses `podman`
# CLI for container setup and `pumba --runtime podman` for exec invocations.
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_exec.bats
#   * Linux: sudo bats tests/podman_exec.bats

load test_helper

# Create a running podman container with the given name for exec tests.
_podman_create_exec_target() {
    local name=$1
    local image=${2:-alpine:latest}
    local command=${3:-"tail -f /dev/null"}
    echo "Creating podman test container: $name (using $image)"
    # shellcheck disable=SC2086
    podman run -d --name "$name" "$image" $command
    wait_for_running podman "$name"
}

setup() {
    require_podman
    podman_rm_force exec_pdm_target exec_pdm_target_1 exec_pdm_target_2
}

teardown() {
    podman_rm_force exec_pdm_target exec_pdm_target_1 exec_pdm_target_2
}

@test "Should display exec help via podman runtime" {
    run pumba --runtime podman exec --help
    assert_success

    assert_output --partial "command"
    assert_output --partial "args"
    assert_output --partial "limit"
}

@test "Should exec default command in container via podman runtime (dry-run)" {
    _podman_create_exec_target exec_pdm_target

    [ "$(podman inspect -f '{{.State.Status}}' exec_pdm_target)" = "running" ]

    run pumba --runtime podman --dry-run exec exec_pdm_target
    echo "Exec status: $status"
    assert_success
}

@test "Should exec custom command in container via podman runtime (dry-run)" {
    _podman_create_exec_target exec_pdm_target

    [ "$(podman inspect -f '{{.State.Status}}' exec_pdm_target)" = "running" ]

    run pumba --runtime podman --dry-run exec --command "echo" exec_pdm_target
    echo "Custom command status: $status"
    assert_success
}

@test "Should exec command with single argument via podman runtime (dry-run)" {
    _podman_create_exec_target exec_pdm_target

    [ "$(podman inspect -f '{{.State.Status}}' exec_pdm_target)" = "running" ]

    run pumba --runtime podman -l debug --dry-run exec --command "echo" --args "hello" exec_pdm_target
    echo "Command with args status: $status"
    assert_success

    assert_output --partial "args"
    assert_output --partial "hello"
}

@test "Should exec command with multiple arguments via podman runtime (dry-run)" {
    _podman_create_exec_target exec_pdm_target

    [ "$(podman inspect -f '{{.State.Status}}' exec_pdm_target)" = "running" ]

    run pumba --runtime podman -l debug --dry-run exec --command "ls" --args "-la" --args "/etc" exec_pdm_target
    echo "Multiple args status: $status"
    assert_success

    assert_output --partial "args"
}

@test "Should respect limit parameter via podman runtime (dry-run)" {
    _podman_create_exec_target exec_pdm_target_1
    _podman_create_exec_target exec_pdm_target_2

    [ "$(podman inspect -f '{{.State.Status}}' exec_pdm_target_1)" = "running" ]
    [ "$(podman inspect -f '{{.State.Status}}' exec_pdm_target_2)" = "running" ]

    run pumba --runtime podman -l debug --dry-run exec --limit 1 "re2:exec_pdm_target_.*"
    echo "Limit parameter status: $status"
    assert_success

    assert_output --partial "limit"
    assert_output --partial "1"
}

@test "Should actually exec command in running container via podman runtime" {
    _podman_create_exec_target exec_pdm_target
    [ "$(podman inspect -f '{{.State.Status}}' exec_pdm_target)" = "running" ]

    run pumba --runtime podman exec --command "touch" --args "/tmp/pumba_was_here" exec_pdm_target
    assert_success

    run podman exec exec_pdm_target ls /tmp/pumba_was_here
    assert_success
}

@test "Should handle gracefully when exec targets non-existent container via podman runtime" {
    run pumba --runtime podman exec --command "echo" nonexistent_container
    assert_success

    echo "Command output: $output"
    assert_output --partial "no containers to exec"
}
