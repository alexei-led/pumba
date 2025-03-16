# Pumba Test Suite Improvements

This document describes the improvements made to the Pumba integration test suite to enhance coverage and reliability.

## New Test Files Added

Several new test files have been added to improve test coverage:

1. **combined_chaos.bats**
   - Tests for using both netem and iptables commands together
   - Verifies the new combined nettools image approach works correctly
   - Tests complex network chaos scenarios with multiple filters

2. **netem_extended.bats**
   - Tests for netem commands not previously covered (duplicate, corrupt)
   - Tests for advanced loss models (loss-state, loss-gemodel)
   - Tests for various distribution options in delay command
   - Tests for all rate limiting options

3. **global_params.bats**
   - Tests for global parameters (--label, --random, --interval, etc.)
   - Verifies logging parameters work correctly (--json, --log-level)
   - Tests error handling options like --skip-error

4. **error_handling.bats**
   - Tests for various error conditions and edge cases
   - Verifies graceful handling of invalid inputs
   - Tests boundary conditions and parameter validation

## Test Suite Consistency Improvements

All tests now follow consistent patterns:

1. **Setup/Teardown Consistency**
   - All test files now properly clean up containers, including nettools containers
   - Helper function to ensure nettools image is available

2. **Test Structure**
   - All tests follow the Given-When-Then pattern in comments
   - All tests include descriptive echo statements for better debugging
   - Assertions include helpful error messages

3. **Naming Conventions**
   - Consistent test naming convention: "Should do something"
   - Consistent container naming conventions
   - Clear distinction between different test categories

## Coverage Improvements

The test suite now has better coverage for:

1. **Network Tools Image Feature**
   - Tests for both Alpine and Debian-based nettools images
   - Tests for using the same image for both netem and iptables commands
   - Tests for multi-container targeting with regex patterns

2. **Parameter Coverage**
   - Tests for all major parameters of each command
   - Tests for parameter combinations and interactions
   - Tests for boundary values and invalid inputs

3. **Error Handling**
   - Tests for graceful handling of error conditions
   - Tests for informative error messages
   - Tests for recovery from error states

## Running the New Tests

You can run the new tests individually:

```bash
bats tests/combined_chaos.bats
bats tests/netem_extended.bats
bats tests/global_params.bats
bats tests/error_handling.bats
```

Or run all tests including the new ones:

```bash
bats tests/*.bats
```

Or use the makefile target:

```bash
make integration-tests
```

## Future Test Improvements

Areas that could be improved in the future:

1. **Performance Testing**
   - Tests for performance characteristics under load
   - Tests for resource usage patterns

2. **Long-Running Tests**
   - Tests for stability over longer durations
   - Tests for repeated chaos events

3. **Configuration File Support**
   - Tests for reading parameters from configuration files
   - Tests for environment variable handling

4. **Platform-Specific Tests**
   - Tests targeting ARM64 platform specifically
   - Tests for behavior in containerized environments like Kubernetes