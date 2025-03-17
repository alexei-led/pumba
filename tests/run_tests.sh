#!/bin/bash

# Test runner for Pumba integration tests
# This script runs the specified bats test(s) or all bats tests and generates a report

# Define colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Define report file
REPORT_FILE="/tmp/pumba_test_report.txt"
TEST_DIR="$(dirname "$0")"

# Clear any existing report
> "${REPORT_FILE}"

# Function to run tests
run_tests() {
    local test_file=$1
    local test_name=$(basename "${test_file}" .bats)
    
    echo -e "${BLUE}Running tests in ${test_name}...${NC}"
    echo "======= Testing ${test_name} =======" >> "${REPORT_FILE}"
    
    # Run the tests
    bats "${test_file}" | tee -a "${REPORT_FILE}"
    
    # Get test status from the last command
    local status=${PIPESTATUS[0]}
    
    if [ $status -eq 0 ]; then
        echo -e "${GREEN}✓ ${test_name} tests passed${NC}"
        echo "✓ All tests in ${test_name} passed" >> "${REPORT_FILE}"
    else
        echo -e "${RED}✗ ${test_name} tests failed${NC}"
        echo "✗ Tests in ${test_name} failed" >> "${REPORT_FILE}"
    fi
    
    echo "" >> "${REPORT_FILE}"
    return $status
}

# Print header
echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}   Pumba Integration Tests      ${NC}"
echo -e "${BLUE}================================${NC}"
echo "Pumba Integration Test Report - $(date)" > "${REPORT_FILE}"
echo "" >> "${REPORT_FILE}"

# Initialize counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Check if specific test files were provided
if [ $# -gt 0 ] && [ "$1" != "--all" ]; then
    # Run only the specified test files
    for test_arg in "$@"; do
        test_file="${TEST_DIR}/${test_arg}"
        
        # Make sure path includes the tests directory
        if [[ "$test_arg" != *"/"* ]]; then
            test_file="${TEST_DIR}/${test_arg}"
        elif [[ "$test_arg" != "${TEST_DIR}"* ]]; then
            test_file="${TEST_DIR}/${test_arg#*/}"
        fi
        
        if [[ -f "${test_file}" ]]; then
            run_tests "${test_file}"
            
            if [ $? -eq 0 ]; then
                PASSED_TESTS=$((PASSED_TESTS + 1))
            else
                FAILED_TESTS=$((FAILED_TESTS + 1))
            fi
            
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
        else
            echo -e "${RED}Error: Test file not found: ${test_file}${NC}"
            exit 1
        fi
    done
else
    # Run each test file in the directory
    for test_file in "${TEST_DIR}"/*.bats; do
        if [[ -f "${test_file}" ]]; then
            # Skip stress tests unless specifically requested
            if [[ "${test_file}" == *"stress.bats"* && "$1" != "--all" ]]; then
                echo -e "${YELLOW}Skipping ${test_file} (use --all to include it)${NC}"
                continue
            fi
            
            # Run the tests in this file
            run_tests "${test_file}"
            
            if [ $? -eq 0 ]; then
                PASSED_TESTS=$((PASSED_TESTS + 1))
            else
                FAILED_TESTS=$((FAILED_TESTS + 1))
            fi
            
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
        fi
    done
fi

# Print summary
echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}   Test Summary                 ${NC}"
echo -e "${BLUE}================================${NC}"
echo -e "Total test files: ${TOTAL_TESTS}"
echo -e "Passed: ${GREEN}${PASSED_TESTS}${NC}"
echo -e "Failed: ${RED}${FAILED_TESTS}${NC}"

echo "" >> "${REPORT_FILE}"
echo "================================" >> "${REPORT_FILE}"
echo "   Test Summary                 " >> "${REPORT_FILE}"
echo "================================" >> "${REPORT_FILE}"
echo "Total test files: ${TOTAL_TESTS}" >> "${REPORT_FILE}"
echo "Passed: ${PASSED_TESTS}" >> "${REPORT_FILE}"
echo "Failed: ${FAILED_TESTS}" >> "${REPORT_FILE}"

echo -e "${BLUE}Test report saved to ${REPORT_FILE}${NC}"

# Exit with error if any tests failed
if [ $FAILED_TESTS -gt 0 ]; then
    exit 1
fi

exit 0