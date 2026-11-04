#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
ROOT=$(pwd)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/repo/.github/scripts" "$TMP/repo/tests"
cp .github/scripts/integration-files.bash "$TMP/repo/.github/scripts/"
cp .github/integration-shards.tsv "$TMP/repo/.github/"
for file in tests/*.bats; do touch "$TMP/repo/$file"; done
script="$TMP/repo/.github/scripts/integration-files.bash"
: >"$TMP/selected"
for runtime in docker containerd podman; do
    for shard in 1 2; do
        if [ "$runtime/$shard" = containerd/2 ]; then continue; fi
        bash "$script" "$runtime" "$shard" >>"$TMP/selected"
    done
done
LC_ALL=C sort "$TMP/selected" >"$TMP/sorted"
printf '%s\n' tests/*.bats | LC_ALL=C sort >"$TMP/expected"
diff -u "$TMP/expected" "$TMP/sorted"
rejected() {
    if bash "$script" "$@" >/dev/null 2>&1; then
        echo 'Invalid shard manifest accepted' >&2
        exit 1
    fi
}
rejected containerd 2
rejected unknown 1
rejected docker 0
rejected docker
printf 'docker\t1\ttests/does_not_exist.bats\n' >>"$TMP/repo/.github/integration-shards.tsv"
rejected docker 1
cp "$ROOT/.github/integration-shards.tsv" "$TMP/repo/.github/"
grep '^docker' "$ROOT/.github/integration-shards.tsv" | head -1 >>"$TMP/repo/.github/integration-shards.tsv"
rejected docker 1
cp "$ROOT/.github/integration-shards.tsv" "$TMP/repo/.github/"
touch "$TMP/repo/tests/new_unassigned.bats"
rejected docker 1
echo 'Integration shards cover every file once and fail closed on drift'
