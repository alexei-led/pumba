#!/usr/bin/env bash
# Offline process-boundary tests: Docker and Git are stubs, never real runtimes.
set -euo pipefail
cd "${BASH_SOURCE[0]%/*}/../.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
root="$tmp/repo with spaces"
bin="$tmp/bin"
mkdir -p "$root/.github/scripts" "$root/.git" "$bin"
cp .github/scripts/archfit-check.bash "$root/.github/scripts/"
printf 'policy bytes\n' >"$root/.archfit.yaml"
printf 'baseline bytes\n' >"$root/.archfit-baseline.json"
for tool in bash tar cat id; do ln -s "$(command -v "$tool")" "$bin/$tool"; done
cat >"$bin/git" <<'STUB'
#!/usr/bin/env bash
[[ ${FAKE_GIT_FAIL:-0} == 0 ]] || exit "$FAKE_GIT_FAIL"
case "$*" in
    *--show-toplevel) printf '%s\n' "$FIXTURE_ROOT" ;;
    *--git-common-dir) printf '%s\n' "$FIXTURE_COMMON" ;;
    *) exit 99 ;;
esac
STUB
cat >"$bin/docker" <<'STUB'
#!/usr/bin/env bash
printf '%s\0' "$@" > "$CAPTURE_ARGS"
cat > "$CAPTURE_TAR"
printf 'fixture report\n'
exit "${FAKE_DOCKER_EXIT:-0}"
STUB
chmod +x "$bin/git" "$bin/docker"
export FIXTURE_ROOT="$root" FIXTURE_COMMON="$root/.git"
export CAPTURE_ARGS="$tmp/args" CAPTURE_TAR="$tmp/inputs.tar"
script="$root/.github/scripts/archfit-check.bash"

run_check() {
    PATH="$bin" DOCKER_CONTEXT=isolated-fixture bash "$script" --format json >"$tmp/stdout" 2>"$tmp/stderr"
}
has_arg() {
    local arg
    while IFS= read -r -d '' arg; do
        [[ "$arg" != "$1" ]] || return 0
    done <"$CAPTURE_ARGS"
    printf 'Missing Docker argument: %s\n' "$1" >&2
    return 1
}

for common in "$root/.git" "$tmp/shared git"; do
    mkdir -p "$common"
    FIXTURE_COMMON="$common"
    run_check
    has_arg linux/amd64
    has_arg ghcr.io/alexei-led/archfit@sha256:bdcfe29bf9def9a4f0ae544923bc3e755fdec2c2cd6c46ec353c01e60642896b
    has_arg "$root:$root"
    has_arg "$common:$common:ro"
    has_arg no-new-privileges
    has_arg json
    tar -xOf "$CAPTURE_TAR" .archfit.yaml | cmp - "$root/.archfit.yaml"
    tar -xOf "$CAPTURE_TAR" .archfit-baseline.json | cmp - "$root/.archfit-baseline.json"
done

for rc in 1 2 3; do
    export FAKE_DOCKER_EXIT="$rc"
    got=0
    run_check || got=$?
    [[ "$got" == "$rc" ]] || {
        printf 'Expected exit %s, got %s\n' "$rc" "$got" >&2
        exit 1
    }
done
unset FAKE_DOCKER_EXIT
rm "$CAPTURE_ARGS"
if PATH="$bin" DOCKER_CONTEXT='' bash "$script" >"$tmp/stdout" 2>"$tmp/stderr"; then
    printf 'Missing explicit context was accepted\n' >&2
    exit 1
fi
[[ ! -e "$CAPTURE_ARGS" ]]

rm "$root/.archfit-baseline.json"
ln -s "$root/.archfit.yaml" "$root/.archfit-baseline.json"
if run_check; then
    printf 'Symlink baseline was accepted\n' >&2
    exit 1
fi
[[ ! -e "$CAPTURE_ARGS" ]]
rm "$root/.archfit-baseline.json"
if run_check; then
    printf 'Missing baseline was accepted\n' >&2
    exit 1
fi
[[ ! -e "$CAPTURE_ARGS" ]]
printf 'baseline bytes\n' >"$root/.archfit-baseline.json"

mv "$bin/docker" "$tmp/docker"
if run_check; then
    printf 'Missing Docker was accepted\n' >&2
    exit 1
fi
[[ ! -e "$CAPTURE_ARGS" ]]
mv "$tmp/docker" "$bin/docker"
export FAKE_GIT_FAIL=23
got=0
run_check || got=$?
[[ "$got" == 23 && ! -e "$CAPTURE_ARGS" ]]
printf 'Archfit profile wrapper regressions passed\n'
