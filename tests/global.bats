#!/usr/bin/env bats

# Load the test helper
load test_helper

@test "Pumba help with --help flag" {
    # Run pumba with help flag
    run pumba --help
    
    # Verify command succeeds
    assert_success
    
    # Verify output contains expected sections
    assert_output --partial "USAGE:"
    assert_output --partial "COMMANDS:"
}

@test "Pumba help with help command" {
    # Run pumba help command
    run pumba help
    
    # Verify command succeeds
    assert_success
    
    # Verify output contains expected sections
    assert_output --partial "USAGE:"
    assert_output --partial "COMMANDS:"
}

@test "Pumba version flag" {
    # Run pumba with version flag
    run pumba --version
    
    # Verify command succeeds
    assert_success
    
    # Verify output contains version information
    [[ $output =~ "Version:" ]] || [[ $output =~ "version" ]]
}
