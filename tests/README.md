# Pumba Integration Tests

This directory contains integration tests for Pumba using the [bats-core](https://github.com/bats-core/bats-core) testing framework.

## Overview

These tests verify Pumba functionality by running tests against a real Docker engine.

## Requirements

- Docker engine running
- [bats-core](https://github.com/bats-core/bats-core) installed

## Test Structure

- `test_helper.bash`: Contains helper functions used across test files
- Individual `.bats` files: Test specific Pumba commands
- `run_tests.sh`: Script to run all tests and generate a report

## Running Tests

### Using make

```bash
# Build and run all integration tests
make integration-tests

# Build and run all tests including stress tests
make integration-tests-all
```

### Using bats directly

```bash
# Run a specific test file
bats tests/stop.bats

# Run all test files
bats tests/*.bats
```

### Using the test runner

```bash
# Run all tests (excluding stress tests)
./tests/run_tests.sh

# Run all tests including stress tests
./tests/run_tests.sh --all
```

## Test Reports

The test runner generates a report at `/tmp/pumba_test_report.txt` that includes:
- Results of each test file
- Pass/fail status for each test
- Summary of overall test results

## Writing New Tests

When writing new tests, please follow these practices:

1. Use the existing structure as a template
2. Include setup and teardown functions to clean up containers
3. Use helper functions from `test_helper.bash` when possible
4. Follow the Given-When-Then pattern in test comments
5. Make sure tests properly clean up after themselves

### Example Test Structure

```bash
#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "test_pattern"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "test_pattern"
}

@test "Should do something specific" {
    # Given (some precondition)
    create_test_container "test_container"
    
    # When (some action is performed)
    run pumba some_command test_container
    
    # Then (expected outcome)
    [ $status -eq 0 ]
    
    # And (more assertions)
    run docker inspect -f {{.State.Status}} test_container
    [ "$output" = "expected_status" ]
}
```

## Troubleshooting

If tests are failing, check:

1. Docker engine is running
2. Pumba binary is in your PATH
3. No leftover containers from previous test runs
4. The Docker API version is compatible with Pumba