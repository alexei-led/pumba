#!/usr/bin/env bats

# `pumba status [--check]` via the containerd runtime, mirroring a minimal
# tests/status.bats. Needs sudo pumba (sidecar overlayfs mounts).

load test_helper

bats_require_minimum_version 1.5.0

STRESS_IMAGE="ghcr.io/alexei-led/stress-ng:0.20.01"
SIDECAR_IMAGE="docker.io/nicolaka/netshoot:latest"
PUMBA_CTR="pumba --runtime containerd --containerd-namespace moby"

setup() {
    local token
    token=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')
    STATUS_CTR="status-ctr-${token}"
    STATUS_STRESS="status_ctr_stress_${token}"
    STATUS_RUN="ctr-st-${token}"
    STATUS_CTR_ID=""
    STATUS_STRESS_ID=""
    STATUS_STARTED=""
    require_containerd
    ctr_pull_image moby "${SIDECAR_IMAGE}"
    sudo ctr -n moby run -d --privileged "${SIDECAR_IMAGE}" "$STATUS_CTR" sleep infinity >/dev/null 2>&1 </dev/null || return $?
    STATUS_CTR_ID=$STATUS_CTR
    wait_for_ctr_running moby "$STATUS_CTR"
}

teardown() {
    if [ -n "$STATUS_STARTED" ]; then
        sudo pkill -f "^pumba .*--run-id ${STATUS_RUN}-" 2>/dev/null || true
    fi
    local target sc
    for target in "$STATUS_CTR_ID" "$STATUS_STRESS_ID"; do
        [ -n "$target" ] || continue
        for sc in $(sudo ctr -n moby c ls -q "labels.\"com.gaiaadm.pumba.skip\"==true,labels.\"com.gaiaadm.pumba.target\"==${target}" 2>/dev/null); do
            sudo ctr -n moby t rm -f "$sc" >/dev/null 2>&1 || true
            sudo ctr -n moby c rm "$sc" >/dev/null 2>&1 || true
            sudo ctr -n moby snapshots rm "${sc}-snapshot" >/dev/null 2>&1 || true
        done
    done
    if [ -n "$STATUS_CTR_ID" ]; then
        sudo ctr -n moby t kill -s SIGKILL "$STATUS_CTR_ID" >/dev/null 2>&1 || true
        sudo ctr -n moby c rm "$STATUS_CTR_ID" >/dev/null 2>&1 || true
    fi
    if [ -n "$STATUS_STRESS_ID" ]; then
        docker rm -f "$STATUS_STRESS_ID" >/dev/null 2>&1 || true
    fi
}

create_status_stress() {
    local id
    id=$(docker run -d --name "$STATUS_STRESS" alpine top) || return $?
    STATUS_STRESS_ID=$id
    wait_for_running docker "$STATUS_STRESS"
}

# Run jq filter $1 on the state of the only target.result in $output.
assert_state() {
    assert_events "[.[] | select(.type == \"target.result\")] | length == 1 and (.[0].state | $1)"
}

status_ndjson() {
    run --separate-stderr sudo ${PUMBA_CTR} -o ndjson status --tc-image "${SIDECAR_IMAGE}" --pull-image=false "$@"
}

@test "Should find a tagged iptables rule, dirty after kill -9 and clean after manual cleanup via containerd" {
    sudo ${PUMBA_CTR} --run-id "${STATUS_RUN}-ipt" iptables --interface lo --pull-image=false --duration 60s \
        loss --probability 1.0 "${STATUS_CTR}" >/dev/null 2>&1 3>&- &
    STATUS_STARTED=1
    wait_for 10 "sudo ctr -n moby t exec --exec-id st-ipt-wait-\$RANDOM ${STATUS_CTR} iptables -S INPUT </dev/null | grep -q 'pumba:${STATUS_RUN}-ipt'" "tagged iptables rule"

    status_ndjson "${STATUS_CTR}"
    assert_success
    assert_state "(.rules | length) == 1 and (.rules[0] | .run_id == \"${STATUS_RUN}-ipt\" and (.spec | contains(\"--comment \\\"pumba:${STATUS_RUN}-ipt\\\"\")))"

    sudo pkill -9 -f "^pumba .*--run-id ${STATUS_RUN}-ipt"

    status_ndjson --check --run-id "${STATUS_RUN}-ipt" "${STATUS_CTR}"
    assert_failure 1
    assert_run_end '.status == "dirty"'

    sudo ctr -n moby t exec --exec-id st-ipt-flush "${STATUS_CTR}" iptables -F INPUT </dev/null

    status_ndjson --check --run-id "${STATUS_RUN}-ipt" "${STATUS_CTR}"
    assert_success
    assert_run_end '.status == "clean"'
}

@test "Should report a netem qdisc as pumba-owned via containerd" {
    sudo ${PUMBA_CTR} --run-id "${STATUS_RUN}-netem" netem --interface lo --pull-image=false --duration 60s \
        delay --time 100 "${STATUS_CTR}" >/dev/null 2>&1 3>&- &
    STATUS_STARTED=1
    wait_for 10 "sudo ctr -n moby t exec --exec-id st-netem-wait-\$RANDOM ${STATUS_CTR} tc qdisc show dev lo </dev/null | grep -q netem" "netem qdisc"

    # no image: inspect by exec in the target (netshoot has tc and iptables)
    run --separate-stderr sudo ${PUMBA_CTR} -o ndjson status --tc-image "" --check "${STATUS_CTR}"
    assert_failure 1
    assert_state '.qdiscs == [{"device":"lo","kind":"netem","ownership":"pumba-owned","run_id":null}]'
    assert_run_end '.status == "dirty"'
}

@test "Should label the stress sidecar and report it via containerd" {
    create_status_stress
    full_id=$STATUS_STRESS_ID
    ctr_pull_image moby "${STRESS_IMAGE}"

    # stress-ng and the run outlast the assertions; teardown ends both
    sudo ${PUMBA_CTR} --run-id "${STATUS_RUN}-stress" stress --duration 300s --pull-image=false --stress-image "${STRESS_IMAGE}" \
        --stressors="--cpu 1 --timeout 300s" "$full_id" >/dev/null 2>&1 3>&- &
    STATUS_STARTED=1
    selector="labels.\"com.gaiaadm.pumba.run-id\"==${STATUS_RUN}-stress,labels.\"com.gaiaadm.pumba.target\"==${STATUS_STRESS_ID}"
    wait_for 20 "[ -n \"\$(sudo ctr -n moby c ls -q '$selector')\" ]" "labeled stress sidecar"

    sidecar=$(sudo ctr -n moby c ls -q "$selector")
    run sudo ctr -n moby c info "$sidecar"
    assert_success
    echo "$output" | jq -e --arg id "$full_id" \
        '.Labels | .["com.gaiaadm.pumba.kind"] == "stress" and .["com.gaiaadm.pumba.target"] == $id and .["com.gaiaadm.pumba.skip"] == "true"'

    status_ndjson --check --run-id "${STATUS_RUN}-stress" "$full_id"
    assert_failure 1
    assert_state ".sidecars | length == 1 and (.[0] | .kind == \"stress\" and .run_id == \"${STATUS_RUN}-stress\" and .target_id == \"${full_id}\")"
    assert_run_end '.status == "dirty"'
}

@test "Should report unknown when status --check selects nothing via containerd" {
    run --separate-stderr sudo ${PUMBA_CTR} -o ndjson status --check missing-ctr-12345
    assert_failure 1
    assert_run_end '.status == "unknown" and .coverage.missing_targets == ["missing-ctr-12345"]'
}
