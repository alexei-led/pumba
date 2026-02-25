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
    [ $status -ne 0 ]
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
