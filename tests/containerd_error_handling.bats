#!/usr/bin/env bats

# Error handling tests for containerd runtime.
# These verify graceful failure when containers don't exist, have exited, or images are missing.

load test_helper

setup() {
    docker rm -f ctr_err_victim >/dev/null 2>&1 || true
}

teardown() {
    docker rm -f ctr_err_victim >/dev/null 2>&1 || true
}

@test "Should handle non-existent nettools image gracefully via containerd runtime" {
    docker run -d --name ctr_err_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_err_victim)

    # Use a non-existent tc-image — pumba should fail with clear error
    run pumba --log-level debug netem --tc-image nonexistent-image:v999 --duration 2s delay --time 100 $full_id
    echo "Non-existent image output: $output"
    # Should fail (non-zero exit) since the image doesn't exist
    assert_failure
}

@test "Should handle kill on already-exited container via containerd runtime" {
    docker run -d --name ctr_err_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_err_victim)
    docker stop ctr_err_victim
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_err_victim | grep -q exited" "container to exit"

    # Kill an already-exited container — should handle gracefully
    run pumba --log-level debug kill $full_id
    echo "Kill exited container: status=$status, output=$output"
    # May succeed (no-op) or fail — either is acceptable, as long as no crash
    [[ $status -eq 0 ]] || [[ "$output" =~ "not found" ]] || [[ "$output" =~ "not running" ]] || [[ "$output" =~ "error" ]]
}

@test "Should handle invalid duration format gracefully via containerd runtime" {
    docker run -d --name ctr_err_victim alpine top

    run pumba netem --duration invalid delay --time 100 ctr_err_victim
    assert_failure
    assert_output --partial "duration"
}

@test "Should handle invalid delay time gracefully via containerd runtime" {
    docker run -d --name ctr_err_victim alpine top

    run pumba netem --duration 1s delay --time invalid ctr_err_victim
    assert_failure
    assert_output --partial "invalid"
}

@test "Should handle invalid rate limit format gracefully via containerd runtime" {
    docker run -d --name ctr_err_victim alpine top

    run pumba netem --duration 1s rate --rate invalid ctr_err_victim
    assert_failure
    assert_output --partial "invalid"
}

@test "Should handle invalid probability value gracefully via containerd runtime" {
    docker run -d --name ctr_err_victim alpine top

    run pumba iptables --duration 1s loss --probability 2.5 ctr_err_victim
    assert_failure
    assert_output --partial "probability"
}

@test "Should handle subcommand typos gracefully via containerd runtime" {
    run pumba netem dealy --duration 1s --time 100 nonexistent_container
    assert_failure
}

@test "Should handle inconsistent iptables mode gracefully via containerd runtime" {
    docker run -d --name ctr_err_victim alpine top

    run pumba iptables --duration 1s loss --mode nth ctr_err_victim
    assert_failure
}

@test "Should fail when kill command has no container arguments via containerd runtime" {
    run pumba kill
    assert_failure
    assert_output --partial "container name"
}

@test "Should fail when stop command has no container arguments via containerd runtime" {
    run pumba stop
    assert_failure
    assert_output --partial "container name"
}

@test "Should fail when rm command has no container arguments via containerd runtime" {
    run pumba rm
    assert_failure
    assert_output --partial "container name"
}

@test "Should fail when pause command has no duration via containerd runtime" {
    docker run -d --name ctr_err_victim alpine top

    run pumba pause ctr_err_victim
    assert_failure
    assert_output --partial "duration"
}

@test "Should fail when netem delay has no duration via containerd runtime" {
    docker run -d --name ctr_err_victim alpine top

    run pumba netem delay --time 100 ctr_err_victim
    assert_failure
    assert_output --partial "duration"
}

@test "Should handle exec on non-existent container gracefully via containerd runtime" {
    run pumba exec nonexistent_container_xyz
    assert_success
    assert_output --partial "no containers to exec"
}
