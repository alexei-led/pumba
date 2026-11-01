#!/usr/bin/env bash
# Thirteen process-boundary regression groups; no network or runtime required.
set -euo pipefail
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
WORKFLOW="$SCRIPT_DIR/../workflows/build.yaml"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/bin" "$TMP/work"
export PATH="$TMP/bin:$PATH"
export MOCK_SCAN_JSON="$TMP/scan.json" MOCK_SCAN_STATUS=0 MOCK_SCAN_STDERR=''
export MOCK_TEE_STATUS=0 MOCK_GO_STATUS=0
export REAL_TEE
REAL_TEE=$(command -v tee)
RUN_COMMAND=(bash "$SCRIPT_DIR/govulncheck.bash")
LOG="$TMP/result.log"

cat >"$TMP/bin/govulncheck" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
[ "$*" = '-json -scan=symbol ./...' ] || exit 42
cat "$MOCK_SCAN_JSON"
printf '%s' "$MOCK_SCAN_STDERR" >&2
exit "$MOCK_SCAN_STATUS"
STUB
chmod +x "$TMP/bin/govulncheck"

fail() {
    echo "FAIL: $*" >&2
    cat "$LOG" >&2
    exit 1
}
contains() { grep -Fq -- "$1" "$LOG" || fail "missing: $1"; }
excludes() { if grep -Fq -- "$1" "$LOG"; then fail "unexpected: $1"; fi; }
run_gate() {
    local expected=$1 status
    if (
        cd "$TMP/work"
        "${RUN_COMMAND[@]}"
    ) >"$LOG" 2>&1; then status=0; else status=$?; fi
    if [ "$expected" = pass ]; then
        [ "$status" = 0 ] || fail "wanted success, got $status"
    else
        [ "$status" != 0 ] || fail 'wanted failure, got success'
    fi
}
stream() {
    jq -nc --args '
      {config: {protocol_version: "v1.0.0", scanner_name: "govulncheck",
        scanner_version: "v1.9.0", scan_level: "symbol", scan_mode: "source"}},
      {SBOM: {modules: [{path: "github.com/docker/docker", version: "v28.5.2+incompatible"}],
        roots: ["github.com/alexei-led/pumba/cmd"]}},
      ($ARGS.positional[] | split(":") | .[0] as $id | .[1] as $level
        | {module: "github.com/docker/docker", version: "v28.5.2+incompatible"}
        + (if $level != "module" then {package: "github.com/docker/docker/client"} else {} end)
        + (if $level == "symbol" then {function: "NewClientWithOpts"} else {} end) as $frame
        | {osv: {id: $id}}, {finding: {osv: $id, trace: [$frame]}})
    ' "$@" >"$MOCK_SCAN_JSON"
}
mutate() { jq -sc "($1) | .[]" "$TMP/valid.json" >"$MOCK_SCAN_JSON"; }

test_no_findings() {
    stream
    run_gate pass
    contains 'raw JSON scanner exit: 0'
    contains 'no symbol findings'
    cmp "$MOCK_SCAN_JSON" "$TMP/work/govulncheck.json"
}
test_known_symbols() {
    stream GO-2026-4887:symbol GO-2026-4883:symbol
    run_gate pass
    contains 'NOT a vulnerability-free scan'
    contains 'GO-2026-4887'
    contains 'GO-2026-4883'
    contains 'github.com/docker/docker@v28.5.2+incompatible'
}
test_unknown_symbol() {
    stream GO-2026-4887:symbol GO-2099-9999:symbol
    run_gate fail
    contains 'Unaccepted symbol findings'
    contains 'GO-2099-9999'
}
test_acceptance_is_exact() {
    local mutation
    stream GO-2026-4887:symbol
    cp "$MOCK_SCAN_JSON" "$TMP/valid.json"
    for mutation in \
        '.[1].SBOM.modules[0].path = "example.org/unrelated" | .[3].finding.trace[0].module = "example.org/unrelated"' \
        '.[1].SBOM.modules[0].version = "v99.0.0" | .[3].finding.trace[0].version = "v99.0.0"'; do
        mutate "$mutation"
        run_gate fail
        contains 'Unaccepted symbol findings'
    done
}
test_failure_after_known_finding() {
    stream GO-2026-4887:symbol
    export MOCK_SCAN_STDERR='panic: scanner failed after a finding'
    for MOCK_SCAN_STATUS in 1 2 3 17 137; do
        run_gate fail
        contains "raw JSON scanner exit: $MOCK_SCAN_STATUS"
        contains 'panic: scanner failed'
        excludes 'Symbol gate accepted'
    done
    MOCK_SCAN_STATUS=0 MOCK_SCAN_STDERR=''
}
test_json_exit_three() {
    stream
    MOCK_SCAN_STATUS=3
    run_gate fail
    contains 'JSON mode requires exit 0'
    MOCK_SCAN_STATUS=0
}
test_inventory_is_separate() {
    local id
    stream GO-2026-5746:module GO-2026-5668:module GO-2026-5617:module GO-2099-8888:package
    run_gate pass
    contains 'Module-only inventory (not symbol calls)'
    contains 'Package-only inventory (not symbol calls)'
    contains 'GO-2099-8888'
    contains 'no symbol findings'
    excludes 'Symbol gate accepted'
    for id in GO-2026-5746 GO-2026-5668 GO-2026-5617; do contains "$id"; done
    for id in GO-2026-5746 GO-2026-5668 GO-2026-5617; do
        stream "$id:symbol"
        run_gate fail
        contains 'Unaccepted symbol findings'
    done
}
test_deepest_level() {
    stream GO-2026-4887:module GO-2026-4887:package GO-2026-4887:symbol GO-2026-4887:symbol GO-2026-5746:module
    run_gate pass
    contains 'Module-only inventory (not symbol calls): GO-2026-5746'
    contains 'Package-only inventory (not symbol calls): none'
    [ "$(grep -c GO-2026-4887 "$LOG")" = 1 ] || fail 'duplicate finding not collapsed'
}
test_invalid_serialization() {
    local valid payload
    stream GO-2026-4887:symbol
    valid=$(cat "$MOCK_SCAN_JSON")
    for payload in '' '   ' 'not JSON' '[]' 'null' "${valid%?}" "$valid
{" "$valid
panic: failed" "$valid"$'\u00a0' "$valid"$'\n{"progress": NaN}' \
        '{"config": {}, "config": {}}' \
        "$valid"$'\n{"progress":{"key":1,"key":2}}' \
        "$valid"$'\n{"progress":{"key":{"a":1},"key":{"b":2}}}' \
        "$valid"$'\n{"progress":{"key":{},"key":{"b":2}}}' \
        "$valid"$'\n{"progress":{"number":Infinity}}' \
        "$valid"$'\n{"progress":{"number":01}}' \
        "$valid"$'\n{"progress":{"number":+1}}' \
        "$valid"$'\n{"progress":{"number":1.}}'; do
        printf '%s' "$payload" >"$MOCK_SCAN_JSON"
        run_gate fail
        contains 'gate failed'
    done
}
test_required_protocol() {
    local mutation
    stream GO-2026-4887:symbol
    cp "$MOCK_SCAN_JSON" "$TMP/valid.json"
    for mutation in \
        '.[1:]' '.[0:1]' 'del(.[2])' '. + [.[0]]' '[{"error":{"message":"tool failed"}}]' \
        '.[0].config.scan_level = "module"' '.[0].config.scan_mode = "binary"' \
        '.[0].config.protocol_version = "v2.0.0"' '.[0].config.scanner_version = "v1.10.0"' \
        '.[0].config.scanner_name = "other"' '.[1].SBOM.roots = []' '.[1].SBOM.modules = []' \
        '.[3].finding.trace = []' '.[3].finding.trace = null' '.[3].finding.trace = [{}]' \
        '.[3].finding.trace = [{module:"github.com/docker/docker",function:"init"}]' \
        '.[3].finding.trace = [{module:"github.com/docker/docker",package:42}]' \
        '.[3].finding.trace[0].function = ""' '.[3].finding.trace[0].version = "v99.0.0"' \
        '.[3].finding.trace[0].position = 42' '.[3].finding.trace[0].position = null' \
        '.[3].finding.trace[0].position = {}' \
        '.[3].finding.trace[0].position = {filename:42,offset:0,line:1,column:1}' \
        '.[3].finding.trace[0].position = {offset:"0",line:1,column:1}' \
        '.[3].finding.trace[0].position = {offset:0,line:1.5,column:1}' \
        '.[3].finding.trace[0].position = {offset:0,line:1,column:null}' \
        '.[3].finding.trace = [{module:"github.com/docker/docker"},{module:"github.com/docker/docker",package:"github.com/docker/docker",function:"init"}]' \
        '. + [{progress:null}]' '. + [{progress:{},finding:{}}]' \
        '.[3].finding.trace[0] |= del(.function) | .[3].finding.trace[0].receiver = "Client"'; do
        mutate "$mutation"
        run_gate fail
        contains 'gate failed'
    done
}
test_unknown_fields_and_order() {
    stream GO-2026-4887:symbol
    cp "$MOCK_SCAN_JSON" "$TMP/valid.json"
    mutate '.[0].config.future_info = "harmless" | .[3].finding.trace[0].position = {filename:"source.go",offset:0,line:1,column:1,future:true} | .[3].finding.future_info = "NaN +1 01" | [.[0], .[3], {progress:{message:"working",number:-1.5e2,extra:[{key:1},{key:2}],empty:{},other:{}}}, .[2], .[1]]'
    run_gate pass
}
test_missing_scanner() {
    stream
    mv "$TMP/bin/govulncheck" "$TMP/bin/scanner.saved"
    # Isolate only the scanner; keep the parser and path helper usable.
    local dependency
    mkdir -p "$TMP/missing-scanner"
    for dependency in dirname jq; do
        ln -s "$(command -v "$dependency")" "$TMP/missing-scanner/$dependency"
    done
    RUN_COMMAND=(env PATH="$TMP/missing-scanner" /bin/bash "$SCRIPT_DIR/govulncheck.bash")
    run_gate fail
    RUN_COMMAND=(bash "$SCRIPT_DIR/govulncheck.bash")
    contains 'raw JSON scanner exit: 127'
    contains 'JSON mode requires exit 0; scanner failed or exit contract changed'
    mv "$TMP/bin/scanner.saved" "$TMP/bin/govulncheck"
}
test_actual_workflow_pipeline() {
    local scanner_status tee_status
    awk '
      /name: Run govulncheck$/ { step=1; next }
      step && /run: \|/ { body=1; next }
      body && /^          / { sub(/^          /, ""); print; next }
      body { exit }
    ' "$WORKFLOW" >"$TMP/step.bash"
    [ -s "$TMP/step.bash" ] || fail 'workflow step not found'
    mkdir -p "$TMP/work/.github"
    ln -s "$SCRIPT_DIR" "$TMP/work/.github/scripts"
    cat >"$TMP/bin/go" <<'STUB'
#!/usr/bin/env bash
[ "$*" = 'install golang.org/x/vuln/cmd/govulncheck@v1.9.0' ] || exit 42
exit "$MOCK_GO_STATUS"
STUB
    cat >"$TMP/bin/tee" <<'STUB'
#!/usr/bin/env bash
"$REAL_TEE" "$@" || exit $?
exit "$MOCK_TEE_STATUS"
STUB
    chmod +x "$TMP/bin/go" "$TMP/bin/tee"
    RUN_COMMAND=(bash "$TMP/step.bash")
    stream GO-2026-4887:symbol
    for scanner_status in 0 1 3; do
        for tee_status in 0 1; do
            MOCK_SCAN_STATUS=$scanner_status MOCK_TEE_STATUS=$tee_status
            if [ "$scanner_status" = 0 ] && [ "$tee_status" = 0 ]; then run_gate pass; else run_gate fail; fi
        done
    done
    MOCK_SCAN_STATUS=0 MOCK_TEE_STATUS=0 MOCK_GO_STATUS=1
    run_gate fail
    MOCK_GO_STATUS=0
    RUN_COMMAND=(bash "$SCRIPT_DIR/govulncheck.bash")
}

for test in test_no_findings test_known_symbols test_unknown_symbol test_acceptance_is_exact \
    test_failure_after_known_finding test_json_exit_three test_inventory_is_separate \
    test_deepest_level test_invalid_serialization test_required_protocol \
    test_unknown_fields_and_order test_missing_scanner test_actual_workflow_pipeline; do
    "$test"
    echo "PASS $test"
done
