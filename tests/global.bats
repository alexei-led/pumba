#!/usr/bin/env bats

# Load the test helper
load test_helper

@test "Pumba help with --help flag" {
    # Run pumba with help flag
    run pumba --help
    
    # Verify command succeeds
    [ $status -eq 0 ]
    
    # Verify output contains expected sections
    [[ $output =~ "USAGE:" ]]
    [[ $output =~ "COMMANDS:" ]]
}

@test "Pumba help with help command" {
    # Run pumba help command
    run pumba help
    
    # Verify command succeeds
    [ $status -eq 0 ]
    
    # Verify output contains expected sections
    [[ $output =~ "USAGE:" ]]
    [[ $output =~ "COMMANDS:" ]]
}

@test "Pumba version flag" {
    # Run pumba with version flag
    run pumba --version
    
    # Verify command succeeds
    [ $status -eq 0 ]
    
    # Verify output contains version information
    [[ $output =~ "Version:" ]] || [[ $output =~ "version" ]]
}
