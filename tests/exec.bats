#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "exec_target"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "exec_target"
    cleanup_containers "exec_target_1"
    cleanup_containers "exec_target_2"
}

@test "Should display exec help" {
    run pumba exec --help
    [ $status -eq 0 ]
    
    # Verify help contains expected options
    [[ $output =~ "command" ]]
    [[ $output =~ "args" ]]
    [[ $output =~ "limit" ]]
}

@test "Should execute command in container" {
    # Given a running container
    create_test_container "exec_target"
    
    # Verify container is running
    assert_container_state "exec_target" "running"
    
    # When running exec with default command
    echo "Running exec with default command..."
    run pumba --dry-run exec exec_target
    
    # Then command should succeed
    echo "Exec status: $status"
    [ $status -eq 0 ]
}

@test "Should execute custom command in container" {
    # Given a running container
    create_test_container "exec_target"
    
    # Verify container is running
    assert_container_state "exec_target" "running"
    
    # When running exec with custom command
    echo "Running exec with custom command..."
    run pumba --dry-run exec --command "echo" exec_target
    
    # Then command should succeed
    echo "Custom command status: $status"
    [ $status -eq 0 ]
}

@test "Should execute command with single argument" {
    # Given a running container
    create_test_container "exec_target"
    
    # Verify container is running
    assert_container_state "exec_target" "running"
    
    # When running exec with command and a single argument
    echo "Running exec with command and single argument..."
    run pumba -l debug --dry-run exec --command "echo" --args "hello" exec_target
    
    # Then command should succeed
    echo "Command with args status: $status"
    [ $status -eq 0 ]
    
    # Verify debug output contains the argument
    [[ $output =~ "args" && $output =~ "hello" ]]
}

@test "Should execute command with multiple arguments using repeated flags" {
    # Given a running container
    create_test_container "exec_target"
    
    # Verify container is running
    assert_container_state "exec_target" "running"
    
    # When running exec with command and multiple arguments using repeated --args flags
    echo "Running exec with multiple arguments using repeated flags..."
    run pumba -l debug --dry-run exec --command "ls" --args "-la" --args "/etc" exec_target
    
    # Then command should succeed
    echo "Multiple args status: $status"
    [ $status -eq 0 ]
    
    # Verify debug output shows arguments were passed
    [[ $output =~ "args" ]]
}

@test "Should respect limit parameter" {
    # Given multiple running containers
    create_test_container "exec_target_1"
    create_test_container "exec_target_2"
    
    # Verify containers are running
    assert_container_state "exec_target_1" "running"
    assert_container_state "exec_target_2" "running"
    
    # When running exec with limit=1
    echo "Running exec with limit=1..."
    run pumba -l debug --dry-run exec --limit 1 "re2:exec_target_.*"
    
    # Then command should succeed
    echo "Limit parameter status: $status"
    [ $status -eq 0 ]
    
    # And output should mention limiting to 1 container
    [[ $output =~ "limit" ]] && [[ $output =~ "1" ]]
}

@test "Should actually execute command in running container" {
    # Given a running container
    create_test_container "exec_target"
    assert_container_state "exec_target" "running"

    # When executing a real command (not dry-run)
    run pumba exec --command "touch" --args "/tmp/pumba_was_here" exec_target
    [ $status -eq 0 ]

    # Then the file should exist inside the container
    run docker exec exec_target ls /tmp/pumba_was_here
    [ $status -eq 0 ]
}

@test "Should handle gracefully when targeting non-existent container" {
    # When targeting a non-existent container
    run pumba exec --command "echo" nonexistent_container

    # Then command should succeed (exit code 0)
    [ $status -eq 0 ]

    # And output should indicate no containers were found
    echo "Command output: $output"
    [[ $output =~ "no containers to exec" ]]
}