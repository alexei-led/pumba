#!/usr/bin/env bash
# Exercise the real status fixture setup and teardown without a container runtime.
set -euo pipefail
cd "$(dirname "$0")/../.."
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

load() { :; }
bats_require_minimum_version() { :; }
require_containerd() { [ "$SCENARIO" != skip ] || exit 77; }
ctr_pull_image() { :; }
wait_for_running() { [ "$SCENARIO" != wait-fail ] || return 24; }
wait_for_ctr_running() { [ "$SCENARIO" != wait-fail ] || return 24; }
sudo() { "$@"; }
pkill() { echo "signal $*" >>"$LOG"; }

remove_id() {
	local id=$1
	awk -v id="$id" '$1 != id' "$STATE" >"$STATE.next"
	mv "$STATE.next" "$STATE"
}

docker() {
	echo "docker $*" >>"$LOG"
	case "$1" in
	run)
		if [ "$SCENARIO" = create-fail ] || [ "$SCENARIO" = stress-fail ]; then
			local name
			if [ "$suite" = docker ]; then name=$STATUS_NAME; else name=$STATUS_STRESS; fi
			echo "$name target -" >>"$STATE"
			echo "$name" >"$COLLISION"
			echo 'not-owned'
			return 23
		fi
		echo 'docker-owned target -' >>"$STATE"
		echo docker-owned
		;;
	ps)
		local arg target=''
		for arg in "$@"; do
			case "$arg" in label=com.gaiaadm.pumba.target=*)
				target=${arg#*=}
				target=${target#*=}
				;;
			esac
		done
		[ -n "$target" ] && [[ " $* " == *' label=com.gaiaadm.pumba.skip=true '* ]] || return 1
		awk -v target="$target" '$2 == "sidecar" && $3 == target {print $1}' "$STATE"
		;;
	rm)
		shift
		local id
		for id in "$@"; do [ "$id" = -f ] || remove_id "$id"; done
		;;
	*)
		echo "unexpected docker call: $*" >&2
		return 1
		;;
	esac
}

ctr() {
	echo "ctr $*" >>"$LOG"
	shift 2 # -n moby
	case "$1 $2" in
	'run -d')
		if [ "$SCENARIO" = create-fail ]; then
			echo "$STATUS_CTR target -" >>"$STATE"
			echo "$STATUS_CTR" >"$COLLISION"
			return 23
		fi
		echo "$STATUS_CTR target -" >>"$STATE"
		;;
	'c ls')
		local selector=${4} target
		[[ "$selector" == labels.\"com.gaiaadm.pumba.skip\"==true,labels.\"com.gaiaadm.pumba.target\"==* ]] || return 1
		target=${selector##*==}
		awk -v target="$target" '$2 == "sidecar" && $3 == target {print $1}' "$STATE"
		;;
	't rm' | 't kill' | 'snapshots rm') : ;;
	'c rm') remove_id "$3" ;;
	*)
		echo "unexpected ctr call: $*" >&2
		return 1
		;;
	esac
}

for suite in docker containerd; do
	for SCENARIO in normal skip create-fail wait-fail stress-fail; do
		[ "$suite/$SCENARIO" != docker/stress-fail ] || continue
		STATE="$TMP/$suite-$SCENARIO.state"
		LOG="$TMP/$suite-$SCENARIO.log"
		COLLISION="$TMP/$suite-$SCENARIO.collision"
		printf '%s\n' 'status_victim target -' 'status_victim_extra target -' \
			'status-ctr target -' 'status_ctr_stress target -' \
			'protected-sidecar sidecar status-ctr' 'prefix-sidecar sidecar docker-owned-extra' \
			'not-owned target -' 'wrong-label target docker-owned' >"$STATE"
		: >"$LOG"
		collision=''
		set +e
		(
			set -e
			file=tests/status.bats
			[ "$suite" = docker ] || file=tests/containerd_status.bats
			# Source the actual functions, stopping before the first Bats test.
			# shellcheck source=/dev/null
			source /dev/stdin <<<"$(sed '/^@test /,$d' "$file")"
			trap teardown EXIT
			setup
			if [ "$suite" = docker ]; then
				[ "$SCENARIO" != skip ] || exit 77
				create_status_victim alpine top
				echo 'owned-sidecar sidecar docker-owned' >>"$STATE"
			else
				if [ "$SCENARIO" != wait-fail ]; then
					echo "owned-ctr-sidecar sidecar $STATUS_CTR_ID" >>"$STATE"
					create_status_stress
					echo 'owned-docker-sidecar sidecar docker-owned' >>"$STATE"
				fi
			fi
		) 2>"$TMP/stderr"
		result=$?
		set -e
		case "$SCENARIO" in
		normal) [ "$result" = 0 ] ;;
		skip) [ "$result" = 77 ] ;;
		create-fail | stress-fail) [ "$result" = 23 ] ;;
		wait-fail) [ "$result" = 24 ] ;;
		esac || {
			echo "$suite/$SCENARIO unexpected exit $result" >&2
			cat "$TMP/stderr" >&2
			exit 1
		}
		for id in status_victim status_victim_extra status-ctr status_ctr_stress protected-sidecar prefix-sidecar not-owned wrong-label; do
			grep -q "^$id " "$STATE" || {
				echo "$suite/$SCENARIO deleted $id" >&2
				exit 1
			}
		done
		if [ -f "$COLLISION" ]; then
			collision=$(cat "$COLLISION")
			grep -q "^$collision " "$STATE" || {
				echo "$suite/$SCENARIO deleted colliding $collision" >&2
				exit 1
			}
		fi
		if awk -v collision="$collision" '$1 != collision {print}' "$STATE" |
			grep -Eq '^(docker-owned|owned-sidecar|owned-ctr-sidecar|owned-docker-sidecar|status-ctr-[0-9a-f]+) '; then
			echo "$suite/$SCENARIO leaked fixtures" >&2
			exit 1
		fi
		if [ "$SCENARIO" = skip ] || [ "$SCENARIO" = create-fail ]; then
			if grep -Eq '^(signal |docker (ps|rm)|ctr -n moby (c ls|c rm|t ))' "$LOG"; then
				echo "$suite/$SCENARIO cleaned without ownership" >&2
				exit 1
			fi
		fi
		echo "PASS status $suite/$SCENARIO"
	done
done
