# Shared real-runtime recovery assertions. Only disposable, recorded fixtures
# are mutated. Lane workers define these tests; the release parent runs them.

revert_setup() {
	RV_IDS=()
	RV_DOCKER_IDS=()
	RV_TARGET=""
	RV_WRITER=""
	RV_TOKEN=$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')
	RV_RUN="rv-${RV_TOKEN}"
	RV_IMAGE="docker.io/nicolaka/netshoot:latest"
	RV_PUMBA=(pumba --runtime "$RV_RUNTIME")
	case "$RV_RUNTIME" in
	docker) skip_if_dind ;;
	podman)
		require_podman
		if podman info --format '{{.Host.Security.Rootless}}' | grep -qi true; then
			skip "recovery needs rootful Podman"
		fi
		;;
	containerd)
		require_containerd
		RV_PUMBA=(sudo pumba --runtime containerd --containerd-namespace moby)
		ctr_pull_image moby "$RV_IMAGE"
		;;
	esac
	rv_create "rv-target-${RV_TOKEN}" target || return
	RV_TARGET=$RV_CREATED
}

rv_create() {
	local name=$1 kind=$2 parent=${3:-} id
	local labels=(--label com.gaiaadm.pumba.skip=false)
	if [ "$kind" != target ]; then
		labels=(--label com.gaiaadm.pumba.skip=true
			--label "com.gaiaadm.pumba.run-id=${RV_RUN}"
			--label "com.gaiaadm.pumba.target=${parent}"
			--label "com.gaiaadm.pumba.kind=${kind}")
	fi
	if [ "$RV_RUNTIME" = containerd ]; then
		sudo ctr -n moby run -d --privileged "${labels[@]}" "$RV_IMAGE" "$name" sleep infinity </dev/null || return
		id=$name
	else
		id=$("$RV_RUNTIME" run -d --privileged --name "$name" "${labels[@]}" "$RV_IMAGE" sleep infinity) || return
	fi
	[ -n "$id" ] || return 1
	RV_CREATED=$id
	RV_IDS+=("$id")
	# Record ownership before readiness can fail, so teardown still owns it.
	if [ "$RV_RUNTIME" = containerd ]; then
		wait_for_ctr_running moby "$id"
	else
		wait_for_running "$RV_RUNTIME" "$id"
	fi
}

rv_exec() {
	if [ "$RV_RUNTIME" = containerd ]; then
		sudo ctr -n moby t exec --exec-id "rv-${RANDOM}-${RANDOM}" "$RV_TARGET" "$@" </dev/null
	else
		"$RV_RUNTIME" exec "$RV_TARGET" "$@"
	fi
}

rv_remove() {
	if [ "$RV_RUNTIME" = containerd ]; then
		sudo ctr -n moby t rm -f "$1" >/dev/null 2>&1 || true
		sudo ctr -n moby c rm "$1" >/dev/null 2>&1 || true
		sudo ctr -n moby snapshots rm "$1" >/dev/null 2>&1 || true
		sudo ctr -n moby snapshots rm "$1-snapshot" >/dev/null 2>&1 || true
	else
		"$RV_RUNTIME" rm -f "$1" >/dev/null 2>&1 || true
	fi
}

rv_exists() {
	if [ "$RV_RUNTIME" = containerd ]; then
		sudo ctr -n moby c info "$1" >/dev/null 2>&1
	else
		"$RV_RUNTIME" inspect "$1" >/dev/null 2>&1
	fi
}

revert_teardown() {
	local id
	if [ -n "${RV_WRITER:-}" ]; then
		# Match only this unique invocation, not sudo/shell wrappers.
		sudo pkill -9 -f "^pumba .*--run-id ${RV_RUN} " 2>/dev/null || true
		wait "$RV_WRITER" 2>/dev/null || true
	fi
	if [ -n "${RV_TARGET:-}" ]; then
		# The writer may have created a sidecar just as a readiness check failed.
		while IFS= read -r id; do
			[ -z "$id" ] || RV_IDS+=("$id")
		done < <(rv_sidecar_ids 2>/dev/null || true)
	fi
	for id in "${RV_IDS[@]}"; do rv_remove "$id"; done
	for id in "${RV_DOCKER_IDS[@]}"; do docker rm -f "$id" >/dev/null 2>&1 || true; done
}

rv_revert() {
	run --separate-stderr "${RV_PUMBA[@]}" -o ndjson "$@" revert --run-id "$RV_RUN" --tc-image "" --pull-image=false "$RV_TARGET"
}

revert_rules_case() {
	local _
	# No sidecar exists: discovery must inspect the target, and preserve both
	# duplicate AND occurrences rather than deduplicating delete work. The
	# listing normalizes these selectors; recovery must retain all dimensions.
	for _ in 1 2; do
		rv_add_match_all_rule iptables "$RV_RUN"
	done
	rv_add_match_all_rule iptables another-run
	rv_revert --dry-run
	assert_success
	assert_events '[.[] | select(.type == "target.result")][0].recovery.rules.revert | length == 2'
	assert_events 'all(.[] | select(.type == "target.result") | .recovery.rules.revert[]; index("192.0.2.1/32") != null and index("198.51.100.2/32") != null and index("12345") != null and index("443") != null)'
	run rv_exec iptables -S INPUT
	assert_output --partial "pumba:${RV_RUN}"
	rv_revert
	assert_success
	assert_events 'all(.[]; .type != "target.applied")'
	assert_run_end '.status == "ok" and .counts.ok == 1 and .artifact_counts.selected == 0'
	run rv_exec iptables -S INPUT
	refute_output --partial "pumba:${RV_RUN}"
	assert_output --partial pumba:another-run
	rv_revert
	assert_success
}

revert_orphan_case() {
	local orphan missing="$RV_TARGET" bystander
	rv_create "rv-orphan-${RV_TOKEN}" stress "$missing"
	orphan=$RV_CREATED
	rv_remove "$missing"
	rv_create "rv-bystander-${RV_TOKEN}" target
	bystander=$RV_CREATED
	# Exact original ID, not a fabricated missing target object. Namespace
	# coverage is unavailable even after successful orphan removal.
	run --separate-stderr "${RV_PUMBA[@]}" -o ndjson revert --run-id "$RV_RUN" --tc-image "" "$missing"
	assert_failure 1
	assert_events '[.[] | select(.type == "artifact.result")] | length == 1 and .[0].status == "ok"'
	assert_events 'all(.[]; .type != "target.result" and .type != "target.applied")'
	assert_run_end '.counts.selected == 0 and .artifact_counts.ok == 1 and (.error.message | contains("unresolved coverage"))'
	! rv_exists "$orphan"
	rv_exists "$bystander"
}

revert_qdisc_case() {
	rv_exec tc qdisc add dev lo root handle 504d: netem delay 10ms
	rv_exec iptables -A INPUT -m comment --comment "pumba:${RV_RUN}" -j DROP
	rv_revert
	assert_failure 1
	run rv_exec tc qdisc show dev lo
	assert_output --partial '504d:'
	run rv_exec iptables -S INPUT
	refute_output --partial "pumba:${RV_RUN}"
}

revert_shared_namespace_case() {
	local first="$RV_TARGET" second
	if [ "$RV_RUNTIME" = containerd ]; then
		# Docker-backed moby tasks provide two distinct container IDs in one
		# disposable namespace without joining or modifying the host namespace.
		first=$(docker run -d --privileged --label com.gaiaadm.pumba.skip=false --name "rv-shared-a-${RV_TOKEN}" "$RV_IMAGE" sleep infinity)
		RV_DOCKER_IDS+=("$first")
		second=$(docker run -d --privileged --label com.gaiaadm.pumba.skip=false --network "container:${first}" --name "rv-shared-b-${RV_TOKEN}" "$RV_IMAGE" sleep infinity)
		RV_DOCKER_IDS+=("$second")
	else
		second=$("$RV_RUNTIME" run -d --privileged --label com.gaiaadm.pumba.skip=false --network "container:${first}" --name "rv-shared-${RV_TOKEN}" "$RV_IMAGE" sleep infinity)
		RV_IDS+=("$second")
	fi
	RV_TARGET=$first
	rv_add_match_all_rule iptables "$RV_RUN"
	run --separate-stderr "${RV_PUMBA[@]}" -o ndjson revert --run-id "$RV_RUN" --tc-image "" "$first" "$second"
	assert_success
	assert_run_end '.counts.ok == 2 and .status == "ok"'
	run rv_exec iptables -S INPUT
	refute_output --partial "pumba:${RV_RUN}"
}

rv_add_match_all_rule() {
	rv_exec "$1" -A INPUT -s 192.0.2.1/32 -d 198.51.100.2/32 \
		-p tcp --sport 12345 --dport 443 -m comment --comment "pumba:$2" \
		-m statistic --mode random --probability 0.5 -j DROP
}

revert_backend_case() {
	# Missing backend tools fail this test rather than silently claiming clean.
	rv_add_match_all_rule iptables-nft "$RV_RUN"
	rv_add_match_all_rule iptables-legacy "$RV_RUN"
	rv_revert --dry-run
	assert_success
	assert_events '[.[] | select(.type == "target.result")][0].recovery.rules.revert | map(.[0]) | sort == ["iptables-legacy","iptables-nft"]'
	rv_revert
	assert_success
	run rv_exec iptables-nft -S INPUT
	refute_output --partial "pumba:${RV_RUN}"
	run rv_exec iptables-legacy -S INPUT
	refute_output --partial "pumba:${RV_RUN}"
}
