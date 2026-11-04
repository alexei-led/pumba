#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
ROOT=$(pwd)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
cat >"$TMP/go" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
case "$1" in
    list)
        if [ "${2:-}" = -m ]; then echo example.invalid/pumba; else echo example.invalid/pumba/cmd; fi
        ;;
    build)
        if [ "${FAIL_TARGET:-}" = "$GOOS/$GOARCH" ]; then touch "$CHECK_TMP/failed"; exit 23; fi
        if [ -n "${FAIL_TARGET:-}" ] && [ "$GOOS" = darwin ]; then
            for _ in {1..100}; do [ -e "$CHECK_TMP/failed" ] && break; sleep 0.01; done
            [ -e "$CHECK_TMP/failed" ]
        fi
        while [ "$1" != -o ]; do shift; done
        mkdir -p "$(dirname "$2")"
        printf '%s/%s\n' "$GOOS" "$GOARCH" >"$2"
        ;;
    *) echo "unexpected Go command: $*" >&2; exit 99 ;;
esac
STUB
chmod +x "$TMP/go"
export CHECK_TMP="$TMP"
for FAIL_TARGET in '' linux/arm64; do
    export FAIL_TARGET
    actual=0
    make --no-print-directory -f "$ROOT/Makefile" release GO="$TMP/go" BIN="$TMP/out" \
        MODULE=example.invalid/pumba PKGS=example.invalid/pumba/cmd TESTPKGS=example.invalid/pumba/cmd \
        >"$TMP/build.log" 2>&1 || actual=$?
    if [ -z "$FAIL_TARGET" ]; then
        [ "$actual" = 0 ]
        for os in darwin linux; do
            for arch in amd64 arm64; do [ "$(cat "$TMP/out/pumba_${os}_${arch}")" = "$os/$arch" ]; done
        done
    else
        [ "$actual" -ne 0 ]
        grep -q 'build failed: linux/arm64' "$TMP/build.log"
        [ -s "$TMP/out/pumba_darwin_arm64" ]
        [ ! -e "$TMP/out/pumba_linux_arm64" ]
    fi
done
echo 'Release builds complete all targets and fail closed on compiler errors'
