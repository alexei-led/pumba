#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
ROOT=$(pwd)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/bin"
cat >"$TMP/bin/gh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
[ "$1" = api ] || exit 90
[ "$SCENARIO" != api-failure ] || exit 23
cat "$FIXTURE"
STUB
chmod +x "$TMP/bin/gh"
export PATH="$TMP/bin:$PATH" FIXTURE="$TMP/run.json"
export GITHUB_REPOSITORY=alexei-led/pumba GITHUB_SHA=0123456789012345678901234567890123456789
for SCENARIO in success wrong-sha wrong-branch wrong-repo pull-request failed empty malformed api-failure; do
    export SCENARIO
    sha=$GITHUB_SHA branch=master repo=$GITHUB_REPOSITORY event=push conclusion=success
    case "$SCENARIO" in
    wrong-sha) sha=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa ;;
    wrong-branch) branch=feature ;;
    wrong-repo) repo=other/fork ;;
    pull-request) event=pull_request ;;
    failed) conclusion=failure ;;
    esac
    printf '{"workflow_runs":[{"head_sha":"%s","head_branch":"%s","head_repository":{"full_name":"%s"},"event":"%s","conclusion":"%s"}]}\n' \
        "$sha" "$branch" "$repo" "$event" "$conclusion" >"$FIXTURE"
    if [ "$SCENARIO" = empty ]; then echo '{"workflow_runs":[]}' >"$FIXTURE"; fi
    if [ "$SCENARIO" = malformed ]; then echo 'not JSON' >"$FIXTURE"; fi
    actual=0
    bash "$ROOT/.github/scripts/verify-release-ci.bash" >/dev/null 2>&1 || actual=$?
    if [ "$SCENARIO" = success ]; then [ "$actual" = 0 ]; else [ "$actual" != 0 ]; fi
done
echo 'Release publication requires trusted exact-SHA CI and fails closed'
