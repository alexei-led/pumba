#!/usr/bin/env bats

# `pumba status [--check]` (Docker runtime): finds iptables rules tagged
# pumba:<run-id>, qdiscs under Pumba's 504d: root handle and labeled sidecars
# left by other pumba runs. See docs/agents.md.

load test_helper

bats_require_minimum_version 1.5.0

STRESS_IMAGE="ghcr.io/alexei-led/stress-ng:0.20.01"

setup() {
    cleanup_status
}

teardown() {
    pkill -f "pumba.*status_victim" 2>/dev/null || true
    cleanup_status
}

# Remove the victim and every Pumba sidecar labeled with its ID.
cleanup_status() {
    local id
    if id=$(docker inspect -f '{{.Id}}' status_victim 2>/dev/null); then
        docker ps -aq --filter "label=com.gaiaadm.pumba.target=${id}" | xargs -r docker rm -f
    fi
    cleanup_containers "status_victim"
}

# Run jq filter $1 on the state of the only target.result in $output.
assert_state() {
    assert_events "[.[] | select(.type == \"target.result\")] | length == 1 and (.[0].state | $1)"
}

victim_pid() {
    docker inspect -f '{{.State.Pid}}' status_victim
}

status_ndjson() {
    run --separate-stderr pumba -o ndjson status --tc-image "${NETTOOLS_IMAGE}" --pull-image=false "$@" status_victim
}

@test "Should find a tagged iptables rule of a running iptables loss" {
    skip_if_dind
    ensure_nettools_image
    create_test_container status_victim alpine:latest top

    pumba --run-id st-ipt iptables --duration 30s --iptables-image "${NETTOOLS_IMAGE}" --pull-image=false \
        loss --probability 1.0 status_victim >/dev/null 2>&1 3>&- &
    wait_for 10 "nsenter -t $(victim_pid) -n iptables -S INPUT | grep -q 'pumba:st-ipt'" "tagged iptables rule"

    status_ndjson
    assert_success
    assert_state '(.rules | length) == 1 and (.rules[0] | .run_id == "st-ipt" and .chain == "INPUT" and (.spec | contains("--comment \"pumba:st-ipt\"")))'
    assert_state '.qdiscs == [] and .sidecars == []'
    assert_run_end '.status == "ok" and .counts.ok == 1 and .coverage.verdict == "dirty"'

    status_ndjson --run-id other-run
    assert_success
    assert_state '.rules == []'

    run --separate-stderr pumba status --tc-image "${NETTOOLS_IMAGE}" --pull-image=false status_victim
    assert_success
    assert_output --partial 'pumba:st-ipt'
    refute_output --partial 'verdict:'
}

@test "Should report a netem qdisc as pumba-owned and dirty after kill -9, clean after manual cleanup" {
    skip_if_dind
    ensure_nettools_image
    create_test_container status_victim alpine:latest top

    pumba --run-id st-netem netem --duration 60s --tc-image "${NETTOOLS_IMAGE}" --pull-image=false \
        delay --time 100 status_victim >/dev/null 2>&1 3>&- &
    PUMBA_PID=$!
    wait_for 10 "nsenter -t $(victim_pid) -n tc qdisc show dev eth0 | grep -q netem" "netem qdisc"

    status_ndjson
    assert_success
    assert_state '.qdiscs == [{"device":"eth0","kind":"netem","ownership":"pumba-owned","run_id":null}]'

    # qdiscs carry no run: a run filter cannot attribute them
    status_ndjson --check --run-id st-netem
    assert_failure 1
    assert_run_end '.status == "unknown" and .coverage.run_filter == "st-netem"'

    kill -9 "$PUMBA_PID"
    wait "$PUMBA_PID" 2>/dev/null || true
    assert_netem_applied status_victim netem

    run --separate-stderr pumba status --check --tc-image "${NETTOOLS_IMAGE}" --pull-image=false status_victim
    assert_failure 1
    assert_output --partial 'verdict: dirty'

    nsenter -t "$(victim_pid)" -n tc qdisc del dev eth0 root

    run --separate-stderr pumba status --check --tc-image "${NETTOOLS_IMAGE}" --pull-image=false status_victim
    assert_success
    assert_output --partial 'verdict: clean'
}

@test "Should report dirty for a run's iptables rule left by kill -9, clean after manual cleanup" {
    skip_if_dind
    ensure_nettools_image
    create_test_container status_victim alpine:latest top

    pumba --run-id st-crash iptables --duration 60s --iptables-image "${NETTOOLS_IMAGE}" --pull-image=false \
        loss --probability 1.0 status_victim >/dev/null 2>&1 3>&- &
    PUMBA_PID=$!
    wait_for 10 "nsenter -t $(victim_pid) -n iptables -S INPUT | grep -q 'pumba:st-crash'" "tagged iptables rule"
    kill -9 "$PUMBA_PID"
    wait "$PUMBA_PID" 2>/dev/null || true

    status_ndjson --check --run-id st-crash
    assert_failure 1
    assert_run_end '.status == "dirty" and .error == null and (.coverage.reasons | any(contains("iptables rule of run st-crash")))'

    nsenter -t "$(victim_pid)" -n iptables -F INPUT

    status_ndjson --check --run-id st-crash
    assert_success
    assert_state '.rules == [] and .qdiscs == [] and .sidecars == []'
    assert_run_end '.status == "clean" and .coverage.verdict == "clean" and .coverage.reasons == []'
}

@test "Should label the stress sidecar with run, target and kind and report it" {
    skip_if_dind
    ensure_nettools_image
    create_test_container status_victim alpine:latest top
    target_id=$(docker inspect -f '{{.Id}}' status_victim)

    # stress-ng and the run outlast the assertions; SIGTERM ends the run below
    pumba --run-id st-stress stress --duration 300s --stress-image "${STRESS_IMAGE}" \
        --stressors="--cpu 1 --timeout 300s" status_victim >/dev/null 2>&1 3>&- &
    PUMBA_PID=$!
    wait_for 60 "[ -n \"\$(docker ps -q --filter label=com.gaiaadm.pumba.run-id=st-stress)\" ]" "labeled stress sidecar"

    sidecar=$(docker ps -q --filter label=com.gaiaadm.pumba.run-id=st-stress)
    run docker inspect -f '{{json .Config.Labels}}' "$sidecar"
    assert_success
    echo "$output" | jq -e --arg id "$target_id" \
        '.["com.gaiaadm.pumba.kind"] == "stress" and .["com.gaiaadm.pumba.target"] == $id and .["com.gaiaadm.pumba.skip"] == "true"'

    status_ndjson --check --run-id st-stress
    assert_failure 1
    assert_state ".sidecars | length == 1 and (.[0] | .kind == \"stress\" and .run_id == \"st-stress\" and .target_id == \"${target_id}\")"
    assert_run_end '.status == "dirty"'

    kill -TERM "$PUMBA_PID"
    wait "$PUMBA_PID" 2>/dev/null || true
    wait_for 30 "[ -z \"\$(docker ps -aq --filter label=com.gaiaadm.pumba.run-id=st-stress)\" ]" "stress sidecar removal"
}

@test "Should report unknown with exit 1 when status --check selects nothing" {
    run --separate-stderr pumba -o ndjson status --check status_missing_12345
    assert_failure 1
    assert_run_end '.status == "unknown" and .error == null and .counts.selected == 0 and .coverage.missing_targets == ["status_missing_12345"]'

    run --separate-stderr pumba status --check status_missing_12345
    assert_failure 1
    assert_output --partial 'verdict: unknown'

    run --separate-stderr pumba status status_missing_12345
    assert_success
}

@test "Should reject status --check with --interval" {
    run --separate-stderr pumba -o ndjson --interval 5s status --check status_victim
    assert_failure 1
    assert_run_end '.status == "usage_error"'
}
