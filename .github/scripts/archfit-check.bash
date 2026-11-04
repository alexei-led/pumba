#!/usr/bin/env bash
# Match the Linux/amd64 measurement environment of the pinned Archfit Action.
# Pins: archfit-action@2221f4bfece0725a657414fed751cdc090ba1bd4, scripts/lib.sh.
set -euo pipefail

: "${DOCKER_CONTEXT:?Set DOCKER_CONTEXT explicitly; this check starts an analysis container.}"
for tool in git tar docker; do
	command -v "$tool" >/dev/null || {
		printf 'Required command not found: %s\n' "$tool" >&2
		exit 1
	}
done

root=$(git -C "${BASH_SOURCE[0]%/*}/../.." rev-parse --show-toplevel)
common=$(git -C "$root" rev-parse --path-format=absolute --git-common-dir)
image=ghcr.io/alexei-led/archfit@sha256:bdcfe29bf9def9a4f0ae544923bc3e755fdec2c2cd6c46ec353c01e60642896b
inputs=(.archfit.yaml .archfit-baseline.json)
if [[ -e "$root/.archfit-labels.yaml" || -L "$root/.archfit-labels.yaml" ]]; then
	inputs+=(.archfit-labels.yaml)
fi
for input in "${inputs[@]}"; do
	[[ -f "$root/$input" && ! -L "$root/$input" ]] || {
		printf 'Archfit input must be a regular file: %s\n' "$root/$input" >&2
		exit 1
	}
done
case "$root$common" in
*:*)
	printf 'Docker bind paths must not contain a colon\n' >&2
	exit 1
	;;
esac
[[ -d "$common" ]] || {
	printf 'Git common directory is unavailable\n' >&2
	exit 1
}

# Git needs temporary registrations even for its policy comparison. Give the
# container a private copy, never write access to shared worktree registrations.
[[ -d "$common/worktrees" && ! -L "$common/worktrees" ]] || {
	printf 'Git worktrees directory must be an existing non-symlink directory\n' >&2
	exit 1
}

# Stream inputs and registrations; remote contexts need no host temporary mount.
# Absolute paths preserve linked-worktree .git pointers.
tar -C "$root" -cf - "${inputs[@]}" -C "$common" worktrees |
	docker run --rm -i --platform linux/amd64 \
		--user "$(id -u):$(id -g)" \
		--cap-drop ALL --security-opt no-new-privileges \
		--env HOME=/tmp \
		--env GIT_CONFIG_COUNT=1 \
		--env GIT_CONFIG_KEY_0=safe.directory \
		--env "GIT_CONFIG_VALUE_0=$root" \
		--volume "$root:$root" --volume "$common:$common:ro" \
		--tmpfs "$common/worktrees:rw,nosuid,nodev,noexec,mode=1777" \
		--workdir "$root" --entrypoint /bin/sh "$image" -c '
            set -eu
            root=$1
            common=$2
            shift 2
            inputs=$(mktemp -d)
            tar -xf - -C "$inputs"
            cp -R "$inputs/worktrees/." "$common/worktrees/"
            exec /usr/local/bin/archfit check --config "$inputs/.archfit.yaml" --root "$root" --base= "$@"
        ' archfit-check "$root" "$common" "$@"
