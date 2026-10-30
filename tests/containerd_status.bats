#!/usr/bin/env bats

# `pumba status [--check]` via the containerd runtime, mirroring a minimal
# tests/status.bats. Needs sudo pumba (sidecar overlayfs mounts).

load test_helper

bats_require_minimum_version 1.5.0

STRESS_IMAGE="ghcr.io/alexei-led/stress-ng:0.20.01"
SIDECAR_IMAGE="docker.io/nicolaka/netshoot:latest"
PUMBA_CTR="pumba --runtime containerd --containerd-namespace moby"

setup() {
    require_containerd
    remove_status_ctr
    ctr_pull_image moby "${SIDECAR_IMAGE}"
    sudo ctr -n moby run -d --privileged "${SIDECAR_IMAGE}" status-ctr sleep infinity >/dev/null 2>&1 </dev/null
    wait_for_ctr_running moby status-ctr
}

teardown() {
    sudo pkill -f '^pumba .*--run-id ctr-st-' 2>/dev/null || true
    remove_status_ctr
    docker rm -f status_ctr_stress >/dev/null 2>&1 || true
    for sc in $(sudo ctr -n moby c ls -q 'labels."com.gaiaadm.pumba.run-id"~=^ctr-st-' 2>/dev/null); do
        sudo ctr -n moby t rm -f "$sc" >/dev/null 2>&1 || true
        sudo ctr -n moby c rm "$sc" >/dev/null 2>&1 || true
        sudo ctr -n moby snapshots rm "${sc}-snapshot" >/dev/null 2>&1 || true
    done
}

remove_status_ctr() {
    sudo ctr -n moby t kill -s SIGKILL status-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm status-ctr >/dev/null 2>&1 || true
}

# Run jq filter $1 on the state of the only target.result in $output.
assert_state() {
    assert_events "[.[] | select(.type == \"target.result\")] | length == 1 and (.[0].state | $1)"
}

status_ndjson() {
    run --separate-stderr sudo ${PUMBA_CTR} -o ndjson status --tc-image "${SIDECAR_IMAGE}" --pull-image=false "$@"
}

@test "Should find a tagged iptables rule, dirty after kill -9 and clean after manual cleanup via containerd" {
    sudo ${PUMBA_CTR} --run-id ctr-st-ipt iptables --interface lo --pull-image=false --duration 60s \
        loss --probability 1.0 status-ctr >/dev/null 2>&1 3>&- &
    wait_for 10 "sudo ctr -n moby t exec --exec-id st-ipt-wait-\$RANDOM status-ctr iptables -S INPUT </dev/null | grep -q 'pumba:ctr-st-ipt'" "tagged iptables rule"

    status_ndjson status-ctr
    assert_success
    assert_state '(.rules | length) == 1 and (.rules[0] | .run_id == "ctr-st-ipt" and (.spec | contains("--comment \"pumba:ctr-st-ipt\"")))'

    sudo pkill -9 -f '^pumba .*--run-id ctr-st-ipt'

    status_ndjson --check --run-id ctr-st-ipt status-ctr
    assert_failure 1
    assert_run_end '.status == "dirty"'

    sudo ctr -n moby t exec --exec-id st-ipt-flush status-ctr iptables -F INPUT </dev/null

    status_ndjson --check --run-id ctr-st-ipt status-ctr
    assert_success
    assert_run_end '.status == "clean"'
}

@test "Should report a netem qdisc as pumba-owned via containerd" {
    sudo ${PUMBA_CTR} --run-id ctr-st-netem netem --interface lo --pull-image=false --duration 60s \
        delay --time 100 status-ctr >/dev/null 2>&1 3>&- &
    wait_for 10 "sudo ctr -n moby t exec --exec-id st-netem-wait-\$RANDOM status-ctr tc qdisc show dev lo </dev/null | grep -q netem" "netem qdisc"

    # no image: inspect by exec in the target (netshoot has tc and iptables)
    run --separate-stderr sudo ${PUMBA_CTR} -o ndjson status --tc-image "" --check status-ctr
    assert_failure 1
    assert_state '.qdiscs == [{"device":"lo","kind":"netem","ownership":"pumba-owned","run_id":null}]'
    assert_run_end '.status == "dirty"'
}

@test "Should label the stress sidecar and report it via containerd" {
    docker run -d --name status_ctr_stress alpine top >/dev/null
    wait_for_running docker status_ctr_stress
    full_id=$(docker inspect --format='{{.Id}}' status_ctr_stress)
    ctr_pull_image moby "${STRESS_IMAGE}"

    # stress-ng and the run outlast the assertions; teardown ends both
    sudo ${PUMBA_CTR} --run-id ctr-st-stress stress --duration 300s --pull-image=false --stress-image "${STRESS_IMAGE}" \
        --stressors="--cpu 1 --timeout 300s" "$full_id" >/dev/null 2>&1 3>&- &
    selector='labels."com.gaiaadm.pumba.run-id"==ctr-st-stress'
    wait_for 20 "[ -n \"\$(sudo ctr -n moby c ls -q '$selector')\" ]" "labeled stress sidecar"

    sidecar=$(sudo ctr -n moby c ls -q "$selector")
    run sudo ctr -n moby c info "$sidecar"
    assert_success
    echo "$output" | jq -e --arg id "$full_id" \
        '.Labels | .["com.gaiaadm.pumba.kind"] == "stress" and .["com.gaiaadm.pumba.target"] == $id and .["com.gaiaadm.pumba.skip"] == "true"'

    status_ndjson --check --run-id ctr-st-stress "$full_id"
    assert_failure 1
    assert_state ".sidecars | length == 1 and (.[0] | .kind == \"stress\" and .run_id == \"ctr-st-stress\" and .target_id == \"${full_id}\")"
    assert_run_end '.status == "dirty"'
}

@test "Should report unknown when status --check selects nothing via containerd" {
    run --separate-stderr sudo ${PUMBA_CTR} -o ndjson status --check missing-ctr-12345
    assert_failure 1
    assert_run_end '.status == "unknown" and .coverage.missing_targets == ["missing-ctr-12345"]'
}
