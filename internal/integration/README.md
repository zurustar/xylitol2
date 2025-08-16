# SIP Server Integration Tests

This directory contains comprehensive integration tests for the SIP server implementation. These tests verify end-to-end functionality, protocol compliance, and performance characteristics.

## Test Coverage

The integration test suite covers the following areas:

### 1. End-to-End SIP Call Flows
- **Basic SIP Methods**: OPTIONS, REGISTER, INVITE, ACK, BYE
- **Dialog Management**: Session establishment and termination
- **Transaction Handling**: Request/response correlation and state management
- **Authentication Flows**: Digest authentication challenges and responses

### 2. Concurrent Registration and Session Handling
- **Concurrent REGISTER**: Multiple simultaneous user registrations
- **Concurrent INVITE**: Multiple simultaneous session establishments
- **Mixed Operations**: Concurrent registration and session operations
- **Resource Management**: Server behavior under concurrent load

### 3. Session-Timer Enforcement Integration
- **Mandatory Session-Timer**: Rejection of INVITE without Session-Expires
- **Timer Validation**: Min/max session timer value enforcement
- **Timer Parameters**: Refresher parameter handling
- **Error Responses**: 421 Extension Required, 422 Session Interval Too Small

### 4. UDP and TCP Transport Protocol Handling
- **UDP Message Handling**: Datagram-based message processing
- **TCP Message Handling**: Stream-based message processing with framing
- **Connection Management**: TCP connection lifecycle and reuse
- **Message Size Handling**: Small, medium, and large message processing
- **Transport Selection**: Automatic transport selection based on message size

### 5. SIP Protocol Compliance (RFC3261)
- **Message Parsing**: Valid and malformed SIP message handling
- **Header Processing**: Required headers, multi-value headers, header folding
- **Transaction State Machine**: Client and server transaction states
- **Error Handling**: Appropriate error responses for various conditions

### 6. Performance and Reliability
- **Throughput Testing**: Requests per second under load
- **Response Time**: Average, minimum, and maximum response times
- **Error Rates**: Success/failure ratios under various conditions
- **Resource Usage**: Memory and connection limit testing

## Test Files

- **`integration_test.go`**: Main integration test suite with basic end-to-end flows
- **`sip_protocol_test.go`**: SIP protocol compliance and edge case testing
- **`session_timer_test.go`**: Comprehensive Session-Timer enforcement testing
- **`transport_test.go`**: Transport layer reliability and performance testing
- **`comprehensive_test.go`**: Complete test suite runner with all scenarios
- **`test_helpers.go`**: Utility functions and helpers for test construction

## Running the Tests

### Prerequisites
- Go 1.19 or later
- SIP server built and available
- Ports 5060 (UDP/TCP) and 8080 (HTTP) available for testing

### Quick Start
```bash
# Run all integration tests
go test ./internal/integration

# Run with verbose output
go test -v ./internal/integration

# Run specific test categories
go test -v -run TestEndToEndSIPCallFlow ./internal/integration
go test -v -run TestSessionTimerEnforcement ./internal/integration
go test -v -run TestTransportProtocol ./internal/integration
```

### Using the Test Runner Script
```bash
# Run all tests with the provided script
./scripts/run_integration_tests.sh

# Run specific test categories
./scripts/run_integration_tests.sh protocol
./scripts/run_integration_tests.sh session-timer
./scripts/run_integration_tests.sh transport
./scripts/run_integration_tests.sh performance

# Run with coverage reporting
./scripts/run_integration_tests.sh --coverage

# Run with verbose output
./scripts/run_integration_tests.sh --verbose
```

## Test Configuration

The tests use a temporary SIP server configuration optimized for testing:

```yaml
server:
  udp_port: 5060
  tcp_port: 5060
database:
  path: "./test.db"
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
  level: "error"  # Reduced for test clarity
```

## Test Scenarios

### Basic Protocol Tests
- Valid SIP message processing
- Malformed message rejection
- Required header validation
- Method support verification

### Authentication Tests
- REGISTER without authentication (401 response)
- Digest authentication challenge/response
- Invalid credential handling
- Authentication bypass for OPTIONS

### Session-Timer Tests
- INVITE without Session-Expires (421 response)
- Valid Session-Timer acceptance
- Timer value validation (422 for invalid values)
- Refresher parameter handling

### Transport Tests
- UDP message handling and response
- TCP connection management and message framing
- Large message handling over both transports
- Connection limits and error handling

### Concurrent Tests
- Multiple simultaneous REGISTER requests
- Multiple simultaneous INVITE requests
- Mixed UDP/TCP concurrent operations
- Resource contention handling

### Performance Tests
- Throughput measurement (requests/second)
- Response time analysis
- Error rate under load
- Connection establishment time

## Expected Results

### Success Criteria
- **Protocol Compliance**: All RFC3261 compliance tests pass
- **Session-Timer Enforcement**: Mandatory Session-Timer correctly enforced
- **Transport Reliability**: Both UDP and TCP handle messages correctly
- **Concurrent Handling**: Server handles concurrent operations without errors
- **Performance**: Achieves minimum throughput and response time targets

### Performance Targets
- **Throughput**: > 20 requests/second for OPTIONS requests
- **Response Time**: < 100ms average for simple requests
- **Error Rate**: < 10% under normal load conditions
- **Concurrent Connections**: Support for 20+ simultaneous TCP connections

## Troubleshooting

### Common Issues

1. **Port Already in Use**
   - Ensure ports 5060 and 8080 are available
   - Stop any running SIP server instances
   - Check for other applications using these ports

2. **Test Timeouts**
   - Increase test timeout with `-timeout` flag
   - Check server startup time
   - Verify network connectivity

3. **Authentication Failures**
   - Verify digest authentication implementation
   - Check nonce generation and validation
   - Ensure proper MD5 hash calculation

4. **Session-Timer Issues**
   - Verify Session-Timer implementation
   - Check min/max timer value configuration
   - Ensure proper header parsing

### Debug Mode
Run tests with verbose output and debug logging:
```bash
go test -v -run TestSpecificTest ./internal/integration
```

Enable server debug logging by modifying the test configuration:
```yaml
logging:
  level: "debug"
```

## Contributing

When adding new integration tests:

1. **Follow Naming Conventions**: Use descriptive test names with Test prefix
2. **Use Test Helpers**: Leverage existing helper functions in `test_helpers.go`
3. **Document Test Purpose**: Include clear descriptions of what each test verifies
4. **Handle Cleanup**: Ensure proper resource cleanup in test teardown
5. **Consider Concurrency**: Make tests safe for parallel execution
6. **Verify Requirements**: Map tests to specific requirements from the design document

## Requirements Mapping

These integration tests verify the following requirements from the SIP server specification:

- **Requirement 1**: User registration with authentication
- **Requirement 2**: Session establishment and forwarding
- **Requirement 3**: Session modification and termination
- **Requirement 4**: SIP method handling (OPTIONS, ACK, INFO, etc.)
- **Requirement 5**: Concurrent session handling
- **Requirement 6**: Logging and monitoring
- **Requirement 7**: UDP and TCP transport support
- **Requirement 8**: Mandatory Session-Timer enforcement
- **Requirement 9**: Persistent user credential storage
- **Requirement 10**: Web-based administration interface
- **Requirement 11**: Configurable server parameters

Each test file includes requirement references in the test documentation to ensure complete coverage of the specification.