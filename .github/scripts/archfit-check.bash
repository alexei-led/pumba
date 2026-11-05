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

# Git needs metadata for its policy comparison. Keep the shared common
# directory read-only and copy it into private container storage. A fresh clone
# has no worktrees directory; that's a valid empty registration set. If present,
# registrations must remain a real directory rather than redirecting the copy.
if [[ -e "$common/worktrees" || -L "$common/worktrees" ]]; then
	[[ -d "$common/worktrees" && ! -L "$common/worktrees" ]] || {
		printf 'Git worktrees path must be a non-symlink directory when present\n' >&2
		exit 1
	}
fi

# Stream trusted profile inputs; remote contexts need no host temporary mount.
# Absolute paths preserve linked-worktree .git pointers.
tar -C "$root" -cf - "${inputs[@]}" |
	docker run --rm -i --platform linux/amd64 \
		--user "$(id -u):$(id -g)" \
		--cap-drop ALL --security-opt no-new-privileges \
		--env HOME=/tmp \
		--env GIT_CONFIG_COUNT=1 \
		--env GIT_CONFIG_KEY_0=safe.directory \
		--env "GIT_CONFIG_VALUE_0=$root" \
		--volume "$root:$root" --volume "$common:/run/pumba-git-common:ro" \
		--tmpfs "$common:rw,nosuid,nodev,noexec,mode=1777" \
		--workdir "$root" --entrypoint /bin/sh "$image" -c '
            set -eu
            root=$1
            common=$2
            shift 2
            inputs=$(mktemp -d)
            tar -xf - -C "$inputs"
            cp -R /run/pumba-git-common/. "$common/"
            exec /usr/local/bin/archfit check --config "$inputs/.archfit.yaml" --root "$root" --base= "$@"
        ' archfit-check "$root" "$common" "$@"
