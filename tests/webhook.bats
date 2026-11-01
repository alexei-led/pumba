#!/usr/bin/env bats

# --webhook: result events POSTed to an HTTP receiver (Docker runtime).
# See docs/agents.md#webhook.

load test_helper

bats_require_minimum_version 1.5.0

setup() {
    WEBHOOK_TARGET_ID=""
    RECEIVER_PID=""
}

teardown() {
    if [ -n "${RECEIVER_PID:-}" ]; then
        kill "$RECEIVER_PID" 2>/dev/null || true
        wait "$RECEIVER_PID" 2>/dev/null || true
    fi
    [ -z "${WEBHOOK_TARGET_ID:-}" ] || docker rm -f "$WEBHOOK_TARGET_ID" >/dev/null 2>&1 || true
}

create_webhook_target() {
    WEBHOOK_TARGET_ID=$(docker run -d --name "pumba-webhook-$BATS_TEST_NUMBER-$$-$RANDOM" alpine:latest top)
}

# Start a local HTTP receiver that appends each POST body to $BODIES and its
# Content-Type to $BODIES.ct, answers 204, and sets $WEBHOOK_URL.
start_receiver() {
    command -v python3 >/dev/null 2>&1 || skip "python3 not available for the webhook receiver"
    BODIES="$BATS_TEST_TMPDIR/bodies.ndjson"
    local port_file="$BATS_TEST_TMPDIR/port"
    cat >"$BATS_TEST_TMPDIR/receiver.py" <<'EOF'
import http.server, os, sys
bodies, port_file = sys.argv[1], sys.argv[2]
class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        with open(bodies, "ab") as f:
            f.write(body)
        with open(bodies + ".ct", "a") as f:
            f.write(self.headers.get("Content-Type", "") + "\n")
        self.send_response(204)
        self.end_headers()
    def log_message(self, *args):
        pass
server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
with open(port_file + ".tmp", "w") as f:
    f.write(str(server.server_address[1]))
os.rename(port_file + ".tmp", port_file)
server.serve_forever()
EOF
    python3 "$BATS_TEST_TMPDIR/receiver.py" "$BODIES" "$port_file" >/dev/null 2>&1 3>&- &
    RECEIVER_PID=$!
    for _ in $(seq 1 50); do
        [ -s "$port_file" ] && break
        sleep 0.1
    done
    [ -s "$port_file" ] || fail "webhook receiver did not start"
    WEBHOOK_URL="http://127.0.0.1:$(cat "$port_file")/events?token=s3cr3t"
}

@test "Should POST the same events to --webhook as on stdout in ndjson mode" {
    create_webhook_target
    start_receiver

    run --separate-stderr pumba -o ndjson --webhook "$WEBHOOK_URL" --dry-run kill "$WEBHOOK_TARGET_ID"
    assert_success

    assert_equal "$(cat "$BODIES")" "$output"
    assert_equal "$(jq -sc '[.[].type]' "$BODIES")" '["run.start","target.result","run.end"]'
    assert_equal "$(sort -u "$BODIES.ct")" "application/json"
    [[ "$stderr" != *"s3cr3t"* ]]
}

@test "Should POST events to --webhook in text mode with stdout empty" {
    start_receiver

    run --separate-stderr pumba --webhook "$WEBHOOK_URL" kill webhook_missing_12345
    assert_success
    assert_equal "$output" ""

    assert_equal "$(jq -sc '[.[].type]' "$BODIES")" '["run.start","run.end"]'
    jq -se 'last | .status == "no_targets" and .events_dropped == 0' "$BODIES" >/dev/null
}

@test "Should keep the outcome and warn once without the URL when --webhook is unreachable" {
    create_webhook_target

    run --separate-stderr pumba -o ndjson --webhook "http://127.0.0.1:9/events?token=s3cr3t" --dry-run kill "$WEBHOOK_TARGET_ID"
    assert_success

    echo "$output" | jq -se 'last | .type == "run.end" and .status == "ok" and .events_dropped == 0' >/dev/null
    assert_equal "$(grep -c 'could not be delivered to --webhook' <<<"$stderr")" "1"
    [[ "$stderr" != *"s3cr3t"* ]]
    [[ "$stderr" != *"127.0.0.1:9"* ]]
}

@test "Should reject a non-http --webhook as a usage error" {
    run --separate-stderr pumba -o ndjson --webhook "ftp://example.com/hook?token=s3cr3t" kill webhook_victim
    assert_failure 1

    echo "$output" | jq -se 'last | .status == "usage_error" and .error.code == "invalid_argument"' >/dev/null
    [[ "$output$stderr" != *"s3cr3t"* ]]
}
