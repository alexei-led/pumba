#!/usr/bin/env bats

load test_helper

setup() {
    # Use Docker to create container (has networking for apk add)
    # Then target it via containerd using its full ID
    docker rm -f stress_victim >/dev/null 2>&1 || true
    docker run -d --name stress_victim alpine sh -c "apk add --no-cache stress-ng >/dev/null 2>&1 && sleep infinity"
    # Wait for stress-ng to be installed
    sleep 5
    # Verify stress-ng is available
    docker exec stress_victim which stress-ng
}

teardown() {
    docker rm -f stress_victim >/dev/null 2>&1 || true
}

@test "Should handle stress on non-existent container via containerd runtime" {
    run pumba --log-level debug stress --duration 5s --stressors="--cpu 1 --timeout 2s" nonexistent_container_12345
    # Pumba should handle gracefully — exit 0 (no matching containers found)
    [ $status -eq 0 ]
}

@test "Should run stress in dry-run mode via containerd runtime" {
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    run pumba --dry-run --log-level debug stress --duration 5s --stressors="--cpu 1 --timeout 2s" $full_id
    [ $status -eq 0 ]

    # Container should still be running (dry-run)
    [ "$(docker inspect -f '{{.State.Status}}' stress_victim)" = "running" ]
}

@test "Should apply stress (CPU) via containerd runtime" {
    # Get full container ID (Docker-created containers live in moby namespace)
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    # Run stress via pumba containerd runtime
    # The containerd StressContainer runs stress-ng directly inside the container via exec
    # Duration is the pumba-side timeout; stressors --timeout is stress-ng side
    run pumba --log-level debug stress --duration 10s --stressors="--cpu 1 --timeout 3s" $full_id

    echo "Pumba stress output: $output"

    # Pumba should exit successfully
    [ $status -eq 0 ]

    # In containerd mode, stress-ng stdout is not forwarded to pumba output
    # But debug log should confirm completion
    [[ "$output" =~ "stress-ng completed" ]]
}
