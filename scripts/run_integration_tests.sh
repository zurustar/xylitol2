#!/bin/bash

# SIP Server Integration Test Runner
# This script runs comprehensive integration tests for the SIP server

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TEST_TIMEOUT="300s"
VERBOSE=${VERBOSE:-false}
COVERAGE=${COVERAGE:-false}
PARALLEL=${PARALLEL:-true}

# Print colored output
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

# Check prerequisites
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
    if [ ! -f "go.mod" ]; then
        print_error "go.mod not found. Please run this script from the project root."
        exit 1
    fi
    
    # Check if integration test directory exists
    if [ ! -d "internal/integration" ]; then
        print_error "Integration test directory not found: internal/integration"
        exit 1
    fi
    
    print_success "Prerequisites check passed"
}

# Build the SIP server
build_server() {
    print_status "Building SIP server..."
    
    if go build -o sipserver ./cmd/sipserver; then
        print_success "SIP server built successfully"
    else
        print_error "Failed to build SIP server"
        exit 1
    fi
}

# Start the SIP server for testing
start_test_server() {
    print_status "Starting test SIP server..."
    
    # Create test configuration
    cat > test_config.yaml << EOF
server:
  udp_port: 5060
  tcp_port: 5060
database:
  path: "./test_sipserver.db"
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
  file: "./test_sipserver.log"
EOF
    
    # Start server in background
    ./sipserver -config test_config.yaml &
    SERVER_PID=$!
    
    # Wait for server to start
    sleep 2
    
    # Check if server is running
    if kill -0 $SERVER_PID 2>/dev/null; then
        print_success "Test SIP server started (PID: $SERVER_PID)"
    else
        print_error "Failed to start test SIP server"
        exit 1
    fi
}

# Stop the test server
stop_test_server() {
    if [ ! -z "$SERVER_PID" ]; then
        print_status "Stopping test SIP server..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
        print_success "Test SIP server stopped"
    fi
}

# Cleanup test files
cleanup() {
    print_status "Cleaning up test files..."
    rm -f sipserver
    rm -f test_config.yaml
    rm -f test_sipserver.db
    rm -f test_sipserver.log
    rm -f coverage.out
    stop_test_server
}

# Run integration tests
run_integration_tests() {
    print_status "Running integration tests..."
    
    # Build test command
    TEST_CMD="go test"
    
    if [ "$VERBOSE" = true ]; then
        TEST_CMD="$TEST_CMD -v"
    fi
    
    if [ "$PARALLEL" = true ]; then
        TEST_CMD="$TEST_CMD -parallel 4"
    fi
    
    if [ "$COVERAGE" = true ]; then
        TEST_CMD="$TEST_CMD -coverprofile=coverage.out"
    fi
    
    TEST_CMD="$TEST_CMD -timeout $TEST_TIMEOUT ./internal/integration"
    
    print_status "Running: $TEST_CMD"
    
    if eval $TEST_CMD; then
        print_success "Integration tests passed!"
        
        if [ "$COVERAGE" = true ] && [ -f "coverage.out" ]; then
            print_status "Generating coverage report..."
            go tool cover -html=coverage.out -o coverage.html
            COVERAGE_PERCENT=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
            print_status "Coverage: $COVERAGE_PERCENT"
            print_status "Coverage report saved to coverage.html"
        fi
    else
        print_error "Integration tests failed!"
        return 1
    fi
}

# Run specific test category
run_test_category() {
    local category=$1
    print_status "Running test category: $category"
    
    TEST_CMD="go test -v -timeout $TEST_TIMEOUT -run $category ./internal/integration"
    
    if eval $TEST_CMD; then
        print_success "Test category '$category' passed!"
    else
        print_error "Test category '$category' failed!"
        return 1
    fi
}

# Show usage
show_usage() {
    echo "Usage: $0 [OPTIONS] [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  all                    Run all integration tests (default)"
    echo "  protocol               Run SIP protocol compliance tests"
    echo "  session-timer          Run Session-Timer enforcement tests"
    echo "  transport              Run transport protocol tests"
    echo "  concurrent             Run concurrent operation tests"
    echo "  error-handling         Run error handling tests"
    echo "  performance            Run performance tests"
    echo "  comprehensive          Run comprehensive integration tests"
    echo ""
    echo "Options:"
    echo "  -v, --verbose          Enable verbose output"
    echo "  -c, --coverage         Enable coverage reporting"
    echo "  -s, --sequential       Run tests sequentially (not in parallel)"
    echo "  -t, --timeout TIMEOUT  Set test timeout (default: 300s)"
    echo "  -h, --help             Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  VERBOSE=true           Enable verbose output"
    echo "  COVERAGE=true          Enable coverage reporting"
    echo "  PARALLEL=false         Disable parallel execution"
}

# Parse command line arguments
parse_args() {
    COMMAND="all"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -c|--coverage)
                COVERAGE=true
                shift
                ;;
            -s|--sequential)
                PARALLEL=false
                shift
                ;;
            -t|--timeout)
                TEST_TIMEOUT="$2"
                shift 2
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            all|protocol|session-timer|transport|concurrent|error-handling|performance|comprehensive)
                COMMAND="$1"
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

# Main execution
main() {
    print_status "SIP Server Integration Test Runner"
    print_status "=================================="
    
    # Set up cleanup trap
    trap cleanup EXIT
    
    # Check prerequisites
    check_prerequisites
    
    # Build server
    build_server
    
    # Start test server
    start_test_server
    
    # Run tests based on command
    case $COMMAND in
        all)
            run_integration_tests
            ;;
        protocol)
            run_test_category "TestSIPProtocolCompliance"
            ;;
        session-timer)
            run_test_category "TestSessionTimerEnforcement"
            ;;
        transport)
            run_test_category "TestTransportProtocol"
            ;;
        concurrent)
            run_test_category "TestConcurrent"
            ;;
        error-handling)
            run_test_category "TestSIPErrorHandling"
            ;;
        performance)
            run_test_category "TestTransportPerformance"
            ;;
        comprehensive)
            run_test_category "TestComprehensiveIntegration"
            ;;
        *)
            print_error "Unknown command: $COMMAND"
            show_usage
            exit 1
            ;;
    esac
    
    print_success "Integration test run completed successfully!"
}

# Parse arguments and run main
parse_args "$@"
main