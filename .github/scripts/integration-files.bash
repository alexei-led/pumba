#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
if [ "$#" -ne 2 ]; then
    echo 'usage: integration-files.bash runtime shard' >&2
    exit 2
fi
case "$1/$2" in
docker/1 | docker/2 | containerd/1 | podman/1 | podman/2) ;;
*)
    echo 'Invalid runtime/shard' >&2
    exit 2
    ;;
esac
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
: >"$TMP/listed"
: >"$TMP/selected"
while IFS=$'\t' read -r runtime shard file extra; do
    case "$runtime" in '' | \#*) continue ;; esac
    if [ -n "$extra" ] || ! [[ "$file" =~ ^tests/[a-z0-9_]+\.bats$ ]] || [ ! -f "$file" ]; then
        echo "Invalid integration manifest entry: $file" >&2
        exit 1
    fi
    case "$file" in
    tests/containerd_*) expected=containerd ;;
    tests/podman_*) expected=podman ;;
    *) expected=docker ;;
    esac
    if [ "$runtime" != "$expected" ]; then
        echo "Wrong runtime for $file" >&2
        exit 1
    fi
    case "$runtime/$shard" in
    docker/1 | docker/2 | containerd/1 | podman/1 | podman/2) ;;
    *)
        echo "Invalid shard for $file" >&2
        exit 1
        ;;
    esac
    printf '%s\n' "$file" >>"$TMP/listed"
    if [ "$runtime" = "$1" ] && [ "$shard" = "$2" ]; then printf '%s\n' "$file" >>"$TMP/selected"; fi
done <.github/integration-shards.tsv
LC_ALL=C sort "$TMP/listed" >"$TMP/sorted"
printf '%s\n' tests/*.bats | LC_ALL=C sort >"$TMP/expected"
if ! diff -u "$TMP/expected" "$TMP/sorted" >&2; then
    echo 'Every integration file must occur exactly once in the shard manifest' >&2
    exit 1
fi
if [ ! -s "$TMP/selected" ]; then
    echo 'Empty integration shard' >&2
    exit 1
fi
cat "$TMP/selected"
