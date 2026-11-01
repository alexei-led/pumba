#!/usr/bin/env bats

# `pumba schema`: the pumba.cli/v1 document on stdout, no runtime needed.
# See docs/agents.md.

load test_helper

bats_require_minimum_version 1.5.0

# Run jq filter $1 on the schema document in $output; fail unless true.
assert_schema() {
    echo "$output" | jq -e "$1" >/dev/null || fail "schema does not match '$1':
$output"
}

@test "Should print a valid pumba.cli/v1 document" {
    run --separate-stderr pumba schema
    assert_success

    assert_schema '.schema == "pumba.cli/v1" and .event_schema == "pumba.event/v1"'
    assert_schema '.error_codes | index("max_targets_exceeded") and index("cleanup_failed")'
    assert_schema '.run_statuses | index("ok") and index("refused")'
    assert_schema '[.global_flags[].name] | index("max-targets") and index("output") and (index("help") | not)'
}

@test "Should describe netem delay with typed flags" {
    run --separate-stderr pumba schema
    assert_success

    assert_schema '[.. | objects | select(has("path")) | .path] | index("netem delay") and index("iptables loss") and index("schema")'
    assert_schema '.commands[] | select(.name == "netem") | .subcommands[] | select(.name == "delay")
        | .flags[] | select(.name == "time") | .type == "int" and .default == 100'
    assert_schema '.commands[] | select(.name == "netem") | .flags[] | select(.name == "duration") | .type == "duration"'
}

@test "Should print the same document in ndjson output mode" {
    run --separate-stderr pumba schema
    assert_success
    local text_doc=$output

    run --separate-stderr pumba -o ndjson schema
    assert_success
    assert_equal "$output" "$text_doc"
}

@test "Should not need a runtime socket" {
    DOCKER_HOST=unix:///nonexistent/docker.sock run --separate-stderr pumba schema
    assert_success
    assert_schema '.schema == "pumba.cli/v1"'

    run --separate-stderr pumba --runtime containerd --containerd-socket /nonexistent/containerd.sock schema
    assert_success
    assert_schema '.schema == "pumba.cli/v1"'
}
