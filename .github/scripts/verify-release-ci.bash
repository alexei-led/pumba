#!/usr/bin/env bash
set -euo pipefail
: "${GITHUB_REPOSITORY:?}"
: "${GITHUB_SHA:?}"
if ! [[ "$GITHUB_SHA" =~ ^[0-9a-f]{40}$ ]]; then
    echo 'Invalid release commit' >&2
    exit 1
fi
url="repos/$GITHUB_REPOSITORY/actions/workflows/build.yaml/runs?head_sha=$GITHUB_SHA&event=push&status=success&per_page=100"
if ! gh api "$url" | jq -e --arg sha "$GITHUB_SHA" --arg repo "$GITHUB_REPOSITORY" '
    .workflow_runs | any(
        .head_sha == $sha and .head_branch == "master" and .event == "push"
        and .conclusion == "success" and .head_repository.full_name == $repo
    )' >/dev/null; then
    echo 'Release requires successful full CI on this exact protected-branch commit' >&2
    exit 1
fi
