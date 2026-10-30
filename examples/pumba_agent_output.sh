#!/bin/sh
# Read Pumba result events (--output ndjson) the way an agent or CI job would.
# Needs: docker, jq, pumba. See docs/agents.md for the event contract.

set -eu

target=${1:-agent-demo}
run_id="demo-$(date +%s)"

docker rm -f "$target" >/dev/null 2>&1 || true
docker run -d --name "$target" alpine top >/dev/null

# Pause the target for 5s; logs go to stderr, events to stdout.
pumba -o ndjson --run-id "$run_id" --max-duration 1m pause --duration 5s "$target" 2>/dev/null |
	while read -r ev; do
		case $(printf '%s' "$ev" | jq -r .type) in
		target.applied) printf '%s' "$ev" | jq -r '"fault active on \(.target.name) - start probes now"' ;;
		target.result) printf '%s' "$ev" | jq -r '"\(.target.name): \(.status), cleanup ok: \(.cleanup.ok)"' ;;
		run.end) printf '%s' "$ev" | jq -r '"run \(.run_id): \(.status) (\(.counts.ok)/\(.counts.selected) ok)"' ;;
		esac
	done

docker rm -f "$target" >/dev/null
