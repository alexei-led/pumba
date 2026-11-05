#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
# shellcheck source=tests/revert_helper.bash
source tests/revert_helper.bash

# These runtime-shaped functions never access a daemon. The regression checks
# that failed readiness cannot leak a fixture and failed creation cannot grant
# teardown ownership of an unrelated name.
docker() {
	if [ "$CREATE_FAIL" = true ]; then return 1; fi
	printf '%s\n' created-id
}
wait_for_running() { return 1; }

export RV_RUNTIME=docker RV_IMAGE=fixture RV_RUN=old
export CREATE_FAIL=false
RV_IDS=()
if rv_create fixture target; then
	echo 'readiness failure was hidden' >&2
	exit 1
fi
[ "${RV_IDS[*]}" = created-id ]
[ "$RV_CREATED" = created-id ]

CREATE_FAIL=true
RV_IDS=()
RV_CREATED=""
if rv_create fixture target; then
	echo 'creation failure was hidden' >&2
	exit 1
fi
[ "${#RV_IDS[@]}" = 0 ]
[ -z "$RV_CREATED" ]
echo 'Recovery fixtures retain exact ownership before readiness and fail closed on creation errors'
