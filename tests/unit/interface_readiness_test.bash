#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
load() { :; }
# shellcheck source=tests/test_helper.bash
source tests/test_helper.bash
podman() {
    [ "$*" = 'exec fixture ip link show dummy0' ] || return 44
    checks=$((checks + 1))
    [ "$ready_after" -gt 0 ] && [ "$checks" -ge "$ready_after" ]
}
sleep() { sleeps=$((sleeps + 1)); }
for ready_after in 1 2 0; do
    checks=0 sleeps=0 actual=0
    wait_for_interface podman fixture dummy0 2 >/dev/null || actual=$?
    case "$ready_after" in
    1) [ "$actual" = 0 ] && [ "$sleeps" = 0 ] ;;
    2) [ "$actual" = 0 ] && [ "$sleeps" = 1 ] ;;
    0) [ "$actual" != 0 ] && [ "$checks" -gt 1 ] ;;
    esac
done
echo 'Fixture interfaces are polled without fixed startup sleeps and fail on timeout'
