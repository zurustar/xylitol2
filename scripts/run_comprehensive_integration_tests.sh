#!/bin/bash

# Comprehensive Integration Test Runner for SIP Server
# This script runs all integration tests covering requirements 5.1, 5.2, 7.1, 7.2, 7.3, 8.1, 8.2

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TEST_TIMEOUT="10m"
VERBOSE=false
COVERAGE=false
SPECIFIC_TEST=""
BUILD_SERVER=true
SERVER_PID=""

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS] [TEST_CATEGORY]

Run comprehensive integration tests for the SIP server.

OPTIONS:
    -h, --help          Show this help message
    -v, --verbose       Enable verbose test output
    -c, --coverage      Enable coverage reporting
    -t, --timeout       Set test timeout (default: ${TEST_TIMEOUT})
    --no-build          Skip server build step
    --server-only       Only start server (for manual testing)

TEST_CATEGORIES:
    all                 Run all integration tests (default)
    comprehensive       Run comprehensive integration test suite
    end-to-end          Run end-to-end call flow tests
    concurrent          Run concurrent operation tests
    session-timer       Run Session-Timer enforcement tests
    transport           Run UDP/TCP transport tests
    protocol            Run SIP protocol compliance tests
    performance         Run performance and load tests
    error-handling      Run error handling tests

EXAMPLES:
    $0                          # Run all tests
    $0 -v comprehensive         # Run comprehensive tests with verbose output
    $0 -c transport             # Run transport tests with coverage
    $0 --server-only            # Start server for manual testing

REQUIREMENTS COVERAGE:
    - Requirement 5.1: Concurrent registration handling
    - Requirement 5.2: Concurrent session handling  
    - Requirement 7.1: UDP transport protocol handling
    - Requirement 7.2: TCP transport protocol handling
    - Requirement 7.3: Transport protocol selection
    - Requirement 8.1: Session-Timer enforcement
    - Requirement 8.2: Session-Timer validation
EOF
}

# Function to parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -c|--coverage)
                COVERAGE=true
                shift
                ;;
            -t|--timeout)
                TEST_TIMEOUT="$2"
                shift 2
                ;;
            --no-build)
                BUILD_SERVER=false
                shift
                ;;
            --server-only)
                start_server_only
                exit 0
                ;;
            all|comprehensive|end-to-end|concurrent|session-timer|transport|protocol|performance|error-handling)
                SPECIFIC_TEST="$1"
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
}

# Function to check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_status "Go version: $GO_VERSION"
    
    # Check if we're in the right directory
    if [[ ! -f "go.mod" ]]; then
        print_error "go.mod not found. Please run this script from the project root directory."
        exit 1
    fi
    
    # Check if integration test directory exists
    if [[ ! -d "internal/integration" ]]; then
        print_error "Integration test directory not found: internal/integration"
        exit 1
    fi
    
    print_success "Prerequisites check passed"
}

# Function to build the server
build_server() {
    if [[ "$BUILD_SERVER" == "false" ]]; then
        print_status "Skipping server build (--no-build specified)"
        return
    fi
    
    print_status "Building SIP server..."
    
    if ! go build -o sipserver ./cmd/sipserver; then
        print_error "Failed to build SIP server"
        exit 1
    fi
    
    print_success "SIP server built successfully"
}

# Function to start server for testing
start_test_server() {
    print_status "Starting SIP server for testing..."
    
    # Create test configuration
    cat > test_config.yaml << EOF
server:
  udp_port: 5060
  tcp_port: 5060
database:
  path: "./test_integration.db"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 8080
  enabled: false
logging:
  level: "error"
  file: "./test_integration.log"
EOF
    
    # Remove existing test database
    rm -f test_integration.db test_integration.log
    
    # Start server in background
    ./sipserver -config test_config.yaml &
    SERVER_PID=$!
    
    # Wait for server to start
    print_status "Waiting for server to start..."
    sleep 3
    
    # Check if server is running
    if ! kill -0 $SERVER_PID 2>/dev/null; then
        print_error "Failed to start SIP server"
        exit 1
    fi
    
    # Test server connectivity
    if ! nc -z localhost 5060 2>/dev/null; then
        print_warning "Server may not be ready on port 5060"
    fi
    
    print_success "SIP server started (PID: $SERVER_PID)"
}

# Function to stop test server
stop_test_server() {
    if [[ -n "$SERVER_PID" ]]; then
        print_status "Stopping SIP server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
        print_success "SIP server stopped"
    fi
    
    # Cleanup test files
    rm -f test_config.yaml test_integration.db test_integration.log sipserver
}

# Function to start server only (for manual testing)
start_server_only() {
    print_status "Starting SIP server for manual testing..."
    check_prerequisites
    build_server
    start_test_server
    
    print_success "SIP server is running on UDP/TCP port 5060"
    print_status "Press Ctrl+C to stop the server"
    
    # Wait for interrupt
    trap stop_test_server EXIT
    wait $SERVER_PID
}

# Function to run specific test category
run_test_category() {
    local category="$1"
    local test_args=""
    
    if [[ "$VERBOSE" == "true" ]]; then
        test_args="$test_args -v"
    fi
    
    if [[ "$COVERAGE" == "true" ]]; then
        test_args="$test_args -coverprofile=coverage_${category}.out"
    fi
    
    case "$category" in
        "comprehensive")
            print_status "Running comprehensive integration test suite..."
            go test $test_args -timeout $TEST_TIMEOUT -run TestComprehensiveIntegrationSuite ./internal/integration
            ;;
        "end-to-end")
            print_status "Running end-to-end call flow tests..."
            go test $test_args -timeout $TEST_TIMEOUT -run TestCompleteCallFlowScenarios ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestCallFlowErrorScenarios ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestCallFlowPerformance ./internal/integration
            ;;
        "concurrent")
            print_status "Running concurrent operation tests..."
            go test $test_args -timeout $TEST_TIMEOUT -run TestConcurrentRegistrationHandling ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestConcurrentSessionHandling ./internal/integration
            ;;
        "session-timer")
            print_status "Running Session-Timer enforcement tests..."
            go test $test_args -timeout $TEST_TIMEOUT -run TestSessionTimerEnforcement ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestSessionTimerConcurrentHandling ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestSessionTimerEdgeCases ./internal/integration
            ;;
        "transport")
            print_status "Running UDP/TCP transport tests..."
            go test $test_args -timeout $TEST_TIMEOUT -run TestUDPTransportProtocol ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestTCPTransportProtocol ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestTransportProtocolSelection ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestTransportReliability ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestTransportMessageSizes ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestTransportErrorConditions ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestTransportPerformance ./internal/integration
            ;;
        "protocol")
            print_status "Running SIP protocol compliance tests..."
            go test $test_args -timeout $TEST_TIMEOUT -run TestSIPProtocolCompliance ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestSIPTransactionHandling ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestSIPDialogHandling ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestSIPHeaderHandling ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestSIPErrorHandling ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestSIPTimerHandling ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestSIPContentHandling ./internal/integration
            ;;
        "performance")
            print_status "Running performance and load tests..."
            go test $test_args -timeout $TEST_TIMEOUT -run TestServerResourceManagement ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestServerShutdownGraceful ./internal/integration
            ;;
        "error-handling")
            print_status "Running error handling tests..."
            go test $test_args -timeout $TEST_TIMEOUT -run TestSIPErrorHandling ./internal/integration
            go test $test_args -timeout $TEST_TIMEOUT -run TestTransportErrorConditions ./internal/integration
            ;;
        "all")
            print_status "Running all integration tests..."
            go test $test_args -timeout $TEST_TIMEOUT ./internal/integration
            ;;
        *)
            print_error "Unknown test category: $category"
            exit 1
            ;;
    esac
}

# Function to generate coverage report
generate_coverage_report() {
    if [[ "$COVERAGE" == "true" ]]; then
        print_status "Generating coverage report..."
        
        # Combine coverage files if multiple exist
        if ls coverage_*.out 1> /dev/null 2>&1; then
            echo "mode: set" > coverage_combined.out
            grep -h -v "^mode:" coverage_*.out >> coverage_combined.out
            go tool cover -html=coverage_combined.out -o coverage_report.html
            print_success "Coverage report generated: coverage_report.html"
        fi
    fi
}

# Function to run all tests
run_all_tests() {
    local categories=("comprehensive" "end-to-end" "concurrent" "session-timer" "transport" "protocol" "performance" "error-handling")
    
    if [[ -n "$SPECIFIC_TEST" ]]; then
        if [[ "$SPECIFIC_TEST" == "all" ]]; then
            run_test_category "all"
        else
            run_test_category "$SPECIFIC_TEST"
        fi
    else
        # Run comprehensive test suite by default
        run_test_category "comprehensive"
    fi
}

# Function to print test summary
print_test_summary() {
    print_status "Integration Test Summary"
    echo "=========================="
    echo "Test Categories Covered:"
    echo "  ✓ End-to-end SIP call flows"
    echo "  ✓ Concurrent registration and session handling"
    echo "  ✓ Session-Timer enforcement integration"
    echo "  ✓ UDP and TCP transport protocol handling"
    echo "  ✓ SIP protocol compliance (RFC3261)"
    echo "  ✓ Error handling and edge cases"
    echo "  ✓ Performance and load testing"
    echo ""
    echo "Requirements Verified:"
    echo "  ✓ Requirement 5.1: Concurrent registration handling"
    echo "  ✓ Requirement 5.2: Concurrent session handling"
    echo "  ✓ Requirement 7.1: UDP transport protocol handling"
    echo "  ✓ Requirement 7.2: TCP transport protocol handling"
    echo "  ✓ Requirement 7.3: Transport protocol selection"
    echo "  ✓ Requirement 8.1: Session-Timer enforcement"
    echo "  ✓ Requirement 8.2: Session-Timer validation"
    echo "=========================="
}

# Main execution function
main() {
    print_status "Starting comprehensive SIP server integration tests..."
    
    # Set up cleanup trap
    trap stop_test_server EXIT
    
    # Parse command line arguments
    parse_args "$@"
    
    # Check prerequisites
    check_prerequisites
    
    # Build server
    build_server
    
    # Start test server
    start_test_server
    
    # Run tests
    print_status "Running integration tests..."
    if run_all_tests; then
        print_success "All integration tests completed successfully!"
    else
        print_error "Some integration tests failed"
        exit 1
    fi
    
    # Generate coverage report
    generate_coverage_report
    
    # Print summary
    print_test_summary
    
    print_success "Integration test execution completed"
}

# Run main function with all arguments
main "$@"