#!/usr/bin/env bats

# Error-handling tests for the podman runtime. Verifies graceful failure when
# containers don't exist / have exited / images are missing / flags are
# malformed, plus podman-specific diagnostics (unreachable socket error listing
# candidates, rootless detection error for netem).
#
# Prerequisites:
#   * `bats`, `podman`, `pumba` on PATH
#   * some tests want to observe the rootless-detection error, so they still
#     need a reachable podman socket (any mode — detection happens at runtime)
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_error_handling.bats
#   * Linux: sudo bats tests/podman_error_handling.bats

load test_helper

setup() {
    require_podman
    podman_rm_force pdm_err_victim
}

teardown() {
    podman_rm_force pdm_err_victim
}

@test "Should handle non-existent nettools image gracefully via podman runtime" {
    podman run -d --name pdm_err_victim alpine top
    full_id=$(podman inspect --format="{{.Id}}" pdm_err_victim)

    run pumba --runtime podman --log-level debug netem --tc-image nonexistent-image:v999 --duration 2s delay --time 100 $full_id
    echo "Non-existent image output: $output"
    assert_failure
}

@test "Should handle kill on already-exited container via podman runtime" {
    podman run -d --name pdm_err_victim alpine top
    full_id=$(podman inspect --format="{{.Id}}" pdm_err_victim)
    podman stop pdm_err_victim
    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_err_victim | grep -q exited" "container to exit"

    run pumba --runtime podman --log-level debug kill $full_id
    echo "Kill exited container: status=$status, output=$output"
    [[ $status -eq 0 ]] || [[ "$output" =~ "not found" ]] || [[ "$output" =~ "not running" ]] || [[ "$output" =~ "error" ]]
}

@test "Should handle invalid duration format gracefully via podman runtime" {
    podman run -d --name pdm_err_victim alpine top

    run pumba --runtime podman netem --duration invalid delay --time 100 pdm_err_victim
    assert_failure
    assert_output --partial "duration"
}

@test "Should handle invalid delay time gracefully via podman runtime" {
    podman run -d --name pdm_err_victim alpine top

    run pumba --runtime podman netem --duration 1s delay --time invalid pdm_err_victim
    assert_failure
    assert_output --partial "invalid"
}

@test "Should handle invalid rate limit format gracefully via podman runtime" {
    podman run -d --name pdm_err_victim alpine top

    run pumba --runtime podman netem --duration 1s rate --rate invalid pdm_err_victim
    assert_failure
    assert_output --partial "invalid"
}

@test "Should handle invalid probability value gracefully via podman runtime" {
    podman run -d --name pdm_err_victim alpine top

    run pumba --runtime podman iptables --duration 1s loss --probability 2.5 pdm_err_victim
    assert_failure
    assert_output --partial "probability"
}

@test "Should handle subcommand typos gracefully via podman runtime" {
    run pumba --runtime podman netem dealy --duration 1s --time 100 nonexistent_container
    assert_failure
}

@test "Should handle inconsistent iptables mode gracefully via podman runtime" {
    podman run -d --name pdm_err_victim alpine top

    run pumba --runtime podman iptables --duration 1s loss --mode nth pdm_err_victim
    assert_failure
}

@test "Should fail when kill command has no container arguments via podman runtime" {
    run pumba --runtime podman kill
    assert_failure
    assert_output --partial "container name"
}

@test "Should fail when stop command has no container arguments via podman runtime" {
    run pumba --runtime podman stop
    assert_failure
    assert_output --partial "container name"
}

@test "Should fail when rm command has no container arguments via podman runtime" {
    run pumba --runtime podman rm
    assert_failure
    assert_output --partial "container name"
}

@test "Should fail when pause command has no duration via podman runtime" {
    podman run -d --name pdm_err_victim alpine top

    run pumba --runtime podman pause pdm_err_victim
    assert_failure
    assert_output --partial "duration"
}

@test "Should fail when netem delay has no duration via podman runtime" {
    podman run -d --name pdm_err_victim alpine top

    run pumba --runtime podman netem delay --time 100 pdm_err_victim
    assert_failure
    assert_output --partial "duration"
}

@test "Should handle exec on non-existent container gracefully via podman runtime" {
    run pumba --runtime podman exec nonexistent_container_xyz
    assert_success
    assert_output --partial "no containers to exec"
}

@test "Should report unreachable --podman-socket with diagnostic error" {
    # Explicit unreachable socket must fail with the runtime's diagnostic error
    # listing attempted candidates (per podman socket-discovery contract).
    run pumba --runtime podman --podman-socket unix:///nonexistent/podman.sock --log-level debug ps
    assert_failure
    assert_output --partial "podman"
}

@test "Should return clear rootless error for netem when podman is rootless" {
    # Podman running rootless cannot apply netem — pumba must fail fast with a
    # diagnostic mentioning rootful mode. Tests that are running on a rootful
    # machine (detected via `podman info`) skip — the error path is covered
    # only when the socket is rootless.
    if podman info --format '{{.Host.Security.Rootless}}' 2>/dev/null | grep -qi '^false$'; then
        skip "podman is rootful — rootless error path not reachable"
    fi

    podman run -d --name pdm_err_victim alpine top

    run pumba --runtime podman --log-level debug netem --duration 1s delay --time 50 pdm_err_victim
    assert_failure
    assert_output --partial "rootless"
    assert_output --partial "rootful"
}
