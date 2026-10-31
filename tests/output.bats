#!/usr/bin/env bats

# Result events: --output ndjson (pumba.event/v1), --run-id, --max-duration,
# and text-mode exit codes (Docker runtime). See docs/agents.md.

load test_helper

bats_require_minimum_version 1.5.0

setup() {
    cleanup_containers "output_"
    sudo pkill -f "pumba.*output_" 2>/dev/null || true
}

teardown() {
    sudo pkill -f "pumba.*output_" 2>/dev/null || true
    cleanup_containers "output_"
    docker ps -q --filter "ancestor=${NETTOOLS_IMAGE:-ghcr.io/alexei-led/pumba-alpine-nettools:latest}" | xargs -r docker rm -f
}

# Print event types of the NDJSON in $output as one compact JSON array.
event_types() {
    echo "$output" | jq -sc '[.[].type]'
}

@test "Should emit run.start, planned target.result and run.end for dry-run kill" {
    create_test_container output_victim alpine:latest top

    run --separate-stderr pumba -o ndjson --dry-run kill output_victim
    assert_success

    assert_equal "$(event_types)" '["run.start","target.result","run.end"]'
    assert_events 'all(.schema == "pumba.event/v1" and .action == "kill")'
    assert_events '.[0].dry_run == true and .[0].runtime == "docker" and .[0].params.signal == "SIGKILL"'
    assert_events '.[1] | .status == "planned" and .tick == 1 and (.target.name | ltrimstr("/")) == "output_victim" and .cleanup == null'
    assert_run_end '.status == "ok" and .counts.planned == 1 and .error == null and .events_dropped == 0'
    assert_container_running output_victim
}

@test "Should report pause applied before its terminal result after unpause" {
    create_test_container output_victim alpine:latest top

    run --separate-stderr pumba -o ndjson pause --duration 2s output_victim
    assert_success

    assert_equal "$(event_types)" '["run.start","target.applied","target.result","run.end"]'
    assert_events '.[2] | .status == "ok" and .cleanup.ok == true and .error == null'
    assert_events '.[1].tick == .[2].tick and .[1].target == .[2].target'
    assert_run_end '.status == "ok" and .counts.selected == 1 and .counts.ok == 1'
    assert_container_running output_victim
}

@test "Should report netem delay applied and cleaned up" {
    skip_if_dind
    ensure_nettools_image
    create_test_container output_victim alpine:latest top

    run --separate-stderr pumba -o ndjson netem --duration 2s --tc-image "${NETTOOLS_IMAGE}" --pull-image=false \
        delay --time 100 output_victim
    assert_success

    assert_equal "$(event_types)" '["run.start","target.applied","target.result","run.end"]'
    assert_events '.[0].action == "netem delay" and .[0].params.time == 100 and .[0].params.duration == "2s"'
    assert_events '.[2] | .status == "ok" and .cleanup.ok == true'
    assert_run_end '.status == "ok" and .counts.ok == 1'
    assert_netem_cleaned output_victim
}

@test "Should keep stdout pure NDJSON with --json debug logs" {
    create_test_container output_victim alpine:latest top

    run --separate-stderr pumba --json --log-level debug -o ndjson --dry-run kill output_victim
    assert_success

    assert_events 'length == 3 and all(.schema == "pumba.event/v1")'
    # logs stay on stderr, as JSON
    echo "$stderr" | head -1 | jq -e '.level == "debug"'
}

@test "Should end with no_targets and exit 0 when nothing matches" {
    run --separate-stderr pumba -o ndjson kill output_missing_12345
    assert_success

    assert_equal "$(event_types)" '["run.start","run.end"]'
    assert_run_end '.status == "no_targets" and .error.code == "no_targets" and .counts.selected == 0'
}

@test "Should end refused with exit 1 when --max-targets is exceeded" {
    create_test_container output_victim_1 alpine:latest top
    create_test_container output_victim_2 alpine:latest top

    run --separate-stderr pumba -o ndjson --max-targets 1 kill "re2:^output_victim_"
    assert_failure 1

    assert_equal "$(event_types)" '["run.start","run.end"]'
    assert_run_end '.status == "refused" and .error.code == "max_targets_exceeded" and .error.phase == "select"'
    assert_container_running output_victim_1
    assert_container_running output_victim_2
}

@test "Should echo --run-id on every event" {
    create_test_container output_victim alpine:latest top

    run --separate-stderr pumba -o ndjson --run-id ci-run.42 --dry-run kill output_victim
    assert_success

    assert_events 'length == 3 and all(.run_id == "ci-run.42")'
}

@test "Should generate one run ID shared by all events" {
    create_test_container output_victim alpine:latest top

    run --separate-stderr pumba -o ndjson --dry-run kill output_victim
    assert_success

    assert_events '([.[].run_id] | unique | length) == 1 and (.[0].run_id | test("^[0-9a-f]{12}$"))'
}

@test "Should report usage_error with exit 1 for an invalid --run-id" {
    run --separate-stderr pumba -o ndjson --run-id "bad id" kill output_victim
    assert_failure 1

    assert_equal "$(event_types)" '["run.end"]'
    assert_run_end '.status == "usage_error" and .error.code == "invalid_argument" and .error.phase == "usage"'
}

@test "Should keep usage text off stdout in ndjson mode" {
    run --separate-stderr pumba -o ndjson netem delay --time 100 output_victim
    assert_failure 1

    assert_equal "$(event_types)" '["run.end"]'
    assert_run_end '.status == "usage_error"'
    [[ "$stderr" == *"unset or invalid duration value"* ]]
}

@test "Should stop an --interval run at --max-duration with timeout and clean up" {
    create_test_container output_victim alpine:latest top

    run --separate-stderr timeout 30 pumba -o ndjson --max-duration 3s --interval 1s \
        pause --duration 500ms output_victim
    assert_failure 1

    # The deadline may cancel the last tick mid-pause, so only require one ok tick and no leftovers.
    assert_events '[.[] | select(.type == "target.result")] | any(.status == "ok") and all(.status != "cleanup_failed")'
    assert_run_end '.status == "aborted" and .error.code == "timeout"'
    assert_container_running output_victim
}

@test "Should keep text-mode stdout empty and exit codes unchanged" {
    create_test_container output_victim_1 alpine:latest top
    create_test_container output_victim_2 alpine:latest top

    run --separate-stderr pumba --dry-run kill output_victim_1
    assert_success
    assert_output ""

    run --separate-stderr pumba kill output_missing_12345
    assert_success
    assert_output ""

    run --separate-stderr pumba --max-targets 1 kill "re2:^output_victim_"
    assert_failure 1
    assert_output ""

    run --separate-stderr pumba netem delay --time 100 output_victim_1
    assert_failure 1

    run pumba no_such_command
    assert_failure 3
}
