#!/usr/bin/env bash
# Shared assertions for Docker, containerd and rootful Podman plan suites.
# PUMBA_BIN selects an isolated build and is resolved before any nested sudo.

plan_setup() {
    PLAN_RUNTIME=$1
    PLAN_TARGET_ID=''
    PLAN_NAMESPACE=${PLAN_NAMESPACE:-moby}
    PLAN_PUMBA=$(command -v "${PUMBA_BIN:-pumba}") || return
    PLAN_IMAGE=${NETTOOLS_IMAGE:-ghcr.io/alexei-led/pumba-alpine-nettools:latest}
    PLAN_UNUSED_IMAGE=example.invalid/pumba-plan-must-not-pull:never
    local name="pumba-plan-${BATS_TEST_NUMBER}-$$-${RANDOM}"
    local command='ip link add dummy0 type dummy && ip link set dummy0 up && exec tail -f /dev/null'
    case "$PLAN_RUNTIME" in
        docker|podman)
            if [ "$PLAN_RUNTIME" = podman ]; then
                require_podman
                [ "$(podman info --format '{{.Host.Security.Rootless}}')" = false ] || skip 'requires rootful Podman'
            fi
            # Record the full ID only after successful creation. Never pre-delete a name.
            PLAN_TARGET_ID=$("$PLAN_RUNTIME" run -d --cap-add NET_ADMIN --entrypoint sh \
                --label com.gaiaadm.pumba.skip=false --name "$name" "$PLAN_IMAGE" -c "$command") || return
            wait_for_running "$PLAN_RUNTIME" "$PLAN_TARGET_ID" || return
            ;;
        containerd)
            require_containerd
            ctr_pull_image "$PLAN_NAMESPACE" "$PLAN_IMAGE" || return
            sudo ctr -n "$PLAN_NAMESPACE" c create --privileged \
                --label com.gaiaadm.pumba.skip=false "$PLAN_IMAGE" "$name" sh -c "$command" || return
            PLAN_TARGET_ID=$name
            sudo ctr -n "$PLAN_NAMESPACE" t start -d "$PLAN_TARGET_ID" || return
            ;;
    esac
    # Also wait for the fixture's interface, not just PID 1.
    local attempt
    for ((attempt = 0; attempt < 30; attempt++)); do
        if plan_target_exec ip link show dummy0 >/dev/null 2>&1; then break; fi
        sleep 0.1
    done
    plan_target_exec ip link show dummy0 >/dev/null || return
    PLAN_QDISCS=$(plan_target_exec tc qdisc show dev dummy0) || return
    PLAN_RULES=$(plan_target_exec iptables -S) || return
}

plan_sidecars() {
    case "$PLAN_RUNTIME" in
        docker|podman)
            "$PLAN_RUNTIME" ps -aq --filter 'label=com.gaiaadm.pumba.skip=true' \
                --filter "label=com.gaiaadm.pumba.target=$PLAN_TARGET_ID"
            ;;
        containerd)
            sudo ctr -n "$PLAN_NAMESPACE" c ls -q \
                "labels.\"com.gaiaadm.pumba.skip\"==true,labels.\"com.gaiaadm.pumba.target\"==$PLAN_TARGET_ID"
            ;;
    esac
}

plan_teardown() {
    [ -n "${PLAN_TARGET_ID:-}" ] || return 0
    local sidecars id
    sidecars=$(plan_sidecars) || return
    # Only sidecars belonging to this recorded target, then the target itself.
    for id in $sidecars "$PLAN_TARGET_ID"; do
        case "$PLAN_RUNTIME" in
            docker|podman) "$PLAN_RUNTIME" rm -f "$id" >/dev/null 2>&1 || true ;;
            containerd)
                sudo ctr -n "$PLAN_NAMESPACE" t rm -f "$id" >/dev/null 2>&1 || true
                sudo ctr -n "$PLAN_NAMESPACE" c rm "$id" >/dev/null 2>&1 || true
                if [ "$id" = "$PLAN_TARGET_ID" ]; then
                    sudo ctr -n "$PLAN_NAMESPACE" snapshots rm "$id" >/dev/null 2>&1 || true
                else
                    sudo ctr -n "$PLAN_NAMESPACE" snapshots rm "$id-snapshot" >/dev/null 2>&1 || true
                fi
                ;;
        esac
    done
}

plan_target_exec() {
    case "$PLAN_RUNTIME" in
        docker|podman) "$PLAN_RUNTIME" exec "$PLAN_TARGET_ID" "$@" ;;
        containerd) sudo ctr -n "$PLAN_NAMESPACE" t exec --exec-id "plan-check-$$-$RANDOM" "$PLAN_TARGET_ID" "$@" ;;
    esac
}

plan_pumba() {
    if [ "$PLAN_RUNTIME" = containerd ]; then
        sudo timeout 15 "$PLAN_PUMBA" --runtime containerd --containerd-namespace "$PLAN_NAMESPACE" "$@"
    else
        timeout 15 "$PLAN_PUMBA" --runtime "$PLAN_RUNTIME" "$@"
    fi
}

plan_assert_untouched() {
    local qdiscs rules sidecars
    qdiscs=$(plan_target_exec tc qdisc show dev dummy0) || return
    rules=$(plan_target_exec iptables -S) || return
    sidecars=$(plan_sidecars) || return
    [ "$qdiscs" = "$PLAN_QDISCS" ] || return 1
    [ "$rules" = "$PLAN_RULES" ] || return 1
    [ -z "$sidecars" ]
}

plan_assert_events() {
    assert_events '[.[].type] == ["run.start","target.result","run.end"]'
    assert_events '.[1] | .status == "planned" and .cleanup == null and .error == null'
    assert_events 'all(.run_id == "plan-test")'
    assert_run_end '.status == "ok" and .counts.selected == 1 and .counts.planned == 1'
}

plan_check_netem() {
    local image mode scoped
    for mode in sidecar exec; do
        image=$PLAN_UNUSED_IMAGE
        [ "$mode" = sidecar ] || image=''
        for scoped in false true; do
            local filters=()
            [ "$scoped" = false ] || filters=(--target 10.0.0.1 --ingress-port 80)
            run --separate-stderr plan_pumba --dry-run --run-id plan-test -o ndjson netem \
                --interface dummy0 --duration 1h --tc-image "$image" --pull-image=true \
                "${filters[@]}" delay --time 100 "$PLAN_TARGET_ID"
            assert_success
            plan_assert_events
            assert_events ".[1].plan | .via == \"$mode\" and (.image // \"\") == \"$image\""
            assert_events '.[1].plan | .apply[0][0:2] == ["sh","-ec"] and .revert[0][0:2] == ["sh","-ec"]'
            assert_events '.[1].plan | (.apply[0][2] | contains("504d:")) and (.revert[0][2] | contains("504d:"))'
            if [ "$scoped" = true ]; then
                assert_events '.[1].plan.apply[0][2] | contains("10.0.0.1/32") and contains("dport")'
            fi
            plan_assert_untouched
        done
    done
}

plan_check_iptables() {
    local image mode filtered
    for mode in sidecar exec; do
        image=$PLAN_UNUSED_IMAGE
        [ "$mode" = sidecar ] || image=''
        for filtered in false true; do
            local filters=()
            [ "$filtered" = false ] || filters=(--source 10.0.0.1 --destination 10.0.0.2 --protocol tcp --dst-port 80)
            run --separate-stderr plan_pumba --dry-run --run-id plan-test -o ndjson iptables \
                --interface dummy0 --duration 1h --iptables-image "$image" --pull-image=true \
                "${filters[@]}" loss --probability 0.2 "$PLAN_TARGET_ID"
            assert_success
            plan_assert_events
            assert_events ".[1].plan | .via == \"$mode\" and (.image // \"\") == \"$image\""
            assert_events '.[1].plan | all(.apply[]; .[0:2] == ["iptables","-I"] and (index("pumba:plan-test") != null))'
            assert_events '.[1].plan | all(.revert[]; .[0:2] == ["iptables","-D"]) and ([.apply[][2:]] == [.revert[][2:]])'
            if [ "$filtered" = true ]; then
                assert_events '.[1].plan.apply | length == 3 and (flatten | index("10.0.0.1/32") != null)'
            fi
            plan_assert_untouched
        done
    done
}

plan_check_text() {
    run --separate-stderr plan_pumba --dry-run netem --interface dummy0 --duration 1h \
        --tc-image "$PLAN_UNUSED_IMAGE" delay --time 100 "$PLAN_TARGET_ID"
    assert_success
    assert_output --partial 'would run on '
    assert_output --partial "via sidecar $PLAN_UNUSED_IMAGE"
    assert_output --partial 'apply:  sh -ec '
    assert_output --partial 'revert: sh -ec '
    plan_assert_untouched

    run --separate-stderr plan_pumba --dry-run --run-id plan-test iptables --interface dummy0 --duration 1h \
        --iptables-image '' loss --probability 0.2 "$PLAN_TARGET_ID"
    assert_success
    assert_output --partial 'via exec in the target'
    assert_output --partial 'apply:  iptables -I INPUT'
    assert_output --partial 'revert: iptables -D INPUT'
    assert_output --partial 'pumba:plan-test'
    plan_assert_untouched
}
