# Stress recovery must prove workload death, not merely container deletion.
# Execute on the runtime host: host /proc is part of the assertion.

rv_sidecar_ids() {
	if [ "$RV_RUNTIME" = containerd ]; then
		sudo ctr -n moby c ls -q "labels.\"com.gaiaadm.pumba.run-id\"==${RV_RUN},labels.\"com.gaiaadm.pumba.target\"==${RV_TARGET}"
	else
		"$RV_RUNTIME" ps -aq --filter "label=com.gaiaadm.pumba.run-id=${RV_RUN}" --filter "label=com.gaiaadm.pumba.target=${RV_TARGET}"
	fi
}

rv_sidecar_pid() {
	if [ "$RV_RUNTIME" = containerd ]; then
		sudo ctr -n moby t ls | awk -v id="$1" '$1 == id {print $2}'
	else
		"$RV_RUNTIME" inspect -f '{{.State.Pid}}' "$1"
	fi
}

rv_capture_workers() {
	local sidecar=$1 init namespace file pid token comm
	init=$(rv_sidecar_pid "$sidecar")
	[ -n "$init" ] && [ "$init" != 0 ] || return 1
	namespace=$(readlink "/proc/${init}/ns/pid") || return 1
	RV_WORKERS=()
	for file in /proc/[0-9]*/comm; do
		pid=${file#/proc/}
		pid=${pid%/comm}
		[ "$(readlink "/proc/${pid}/ns/pid" 2>/dev/null)" = "$namespace" ] || continue
		comm=$(cat "$file" 2>/dev/null) || continue
		case "$comm" in stress-ng*) ;; *) continue ;; esac
		token=$(awk '{print $22}' "/proc/${pid}/stat") || continue
		RV_WORKERS+=("${pid}:${token}")
	done
	# Require the stress supervisor AND a real workload process.
	[ "${#RV_WORKERS[@]}" -ge 2 ]
}

rv_workers_dead() {
	local worker pid token current state
	for worker in "${RV_WORKERS[@]}"; do
		pid=${worker%%:*}
		token=${worker#*:}
		[ -r "/proc/${pid}/stat" ] || continue
		current=$(awk '{print $22}' "/proc/${pid}/stat") || continue
		[ "$current" = "$token" ] || continue # PID reused, original is dead
		state=$(awk '{print $3}' "/proc/${pid}/stat") || continue
		[ "$state" = Z ] || return 1
	done
}

revert_stress_case() {
	local inject=${1:-false} image="ghcr.io/alexei-led/stress-ng:0.20.01"
	local events="${BATS_TEST_TMPDIR}/stress.ndjson" sidecar="" _
	local flags=()
	if [ "$inject" = true ]; then flags=(--inject-cgroup); fi
	if [ "$RV_RUNTIME" = containerd ]; then
		ctr_pull_image moby "$image"
	else
		"$RV_RUNTIME" pull "$image" >/dev/null
	fi
	"${RV_PUMBA[@]}" -o ndjson --run-id "$RV_RUN" stress --duration 300s --pull-image=false \
		--stress-image "$image" "${flags[@]}" --stressors="--cpu 1 --cpu-method loop --timeout 300s" "$RV_TARGET" \
		>"$events" 2>"${BATS_TEST_TMPDIR}/stress.stderr" 3>&- &
	RV_WRITER=$!
	for _ in {1..60}; do
		sidecar=$(rv_sidecar_ids)
		if [ -n "$sidecar" ] && jq -e 'select(.type == "target.applied")' "$events" >/dev/null 2>&1 && rv_capture_workers "$sidecar"; then
			break
		fi
		sleep 1
	done
	# Save every observed fixture before asserting, so failure teardown is safe.
	if [ -n "$sidecar" ]; then RV_IDS+=("$sidecar"); fi
	jq -e 'select(.type == "target.applied")' "$events" >/dev/null
	rv_capture_workers "$sidecar"
	sudo pkill -9 -f "^pumba .*--run-id ${RV_RUN} "
	wait "$RV_WRITER" 2>/dev/null || true
	RV_WRITER=""
	rv_revert
	assert_success
	! rv_exists "$sidecar"
	rv_exists "$RV_TARGET"
	for _ in {1..30}; do
		if rv_workers_dead; then return 0; fi
		sleep 1
	done
	echo "recovery removed the sidecar but stress processes are still alive" >&2
	return 1
}
