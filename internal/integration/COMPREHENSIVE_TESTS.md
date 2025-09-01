# Comprehensive Integration Tests

This document describes the comprehensive integration test suite for the SIP server, covering all requirements specified in task 17.

## Overview

The comprehensive integration test suite validates the complete SIP server functionality through end-to-end testing scenarios. These tests verify that all components work together correctly under various conditions including normal operation, concurrent load, error conditions, and edge cases.

## Requirements Coverage

This test suite covers the following requirements from the SIP server specification:

### Requirement 5.1 - Concurrent Registration Handling
- **Test Coverage**: `TestConcurrentRegistrationHandling`
- **Scenarios**: Multiple simultaneous REGISTER requests
- **Validation**: Server handles concurrent registrations without errors
- **Files**: `comprehensive_integration_test.go`, `integration_test.go`

### Requirement 5.2 - Concurrent Session Handling  
- **Test Coverage**: `TestConcurrentSessionHandling`
- **Scenarios**: Multiple simultaneous INVITE requests and session establishment
- **Validation**: Server processes concurrent sessions correctly
- **Files**: `comprehensive_integration_test.go`, `integration_test.go`

### Requirement 7.1 - UDP Transport Protocol Handling
- **Test Coverage**: `TestUDPTransportProtocol`
- **Scenarios**: UDP message processing, large messages, packet handling
- **Validation**: Proper UDP transport layer functionality
- **Files**: `comprehensive_integration_test.go`, `transport_test.go`

### Requirement 7.2 - TCP Transport Protocol Handling
- **Test Coverage**: `TestTCPTransportProtocol`
- **Scenarios**: TCP connection management, message framing, stream processing
- **Validation**: Reliable TCP transport with proper connection lifecycle
- **Files**: `comprehensive_integration_test.go`, `transport_test.go`

### Requirement 7.3 - Transport Protocol Selection
- **Test Coverage**: `TestTransportProtocolSelection`
- **Scenarios**: Parallel UDP/TCP operations, transport-specific behavior
- **Validation**: Both transports work simultaneously and independently
- **Files**: `comprehensive_integration_test.go`, `transport_test.go`

### Requirement 8.1 - Session-Timer Enforcement
- **Test Coverage**: `TestSessionTimerEnforcement`
- **Scenarios**: INVITE without Session-Timer rejection, mandatory timer validation
- **Validation**: 421 Extension Required for missing Session-Timer
- **Files**: `comprehensive_integration_test.go`, `session_timer_test.go`

### Requirement 8.2 - Session-Timer Validation
- **Test Coverage**: `TestSessionTimerValidation`
- **Scenarios**: Timer value validation, Min-SE handling, refresher parameters
- **Validation**: Proper timer value enforcement and error responses
- **Files**: `comprehensive_integration_test.go`, `session_timer_test.go`

## Test Structure

### Main Test Files

1. **`comprehensive_integration_test.go`**
   - Primary comprehensive test suite
   - Covers all major test categories
   - Entry point: `TestComprehensiveIntegrationSuite`

2. **`end_to_end_call_flows_test.go`**
   - Complete call flow scenarios
   - Authentication flows
   - Call termination scenarios
   - Entry point: `TestCompleteCallFlowScenarios`

3. **`integration_test.go`**
   - Basic integration tests
   - Server setup and teardown utilities
   - Core test infrastructure

4. **`session_timer_test.go`**
   - Session-Timer specific tests
   - Concurrent Session-Timer handling
   - Edge cases and error conditions

5. **`transport_test.go`**
   - Transport layer reliability tests
   - Performance characteristics
   - Error condition handling

6. **`sip_protocol_test.go`**
   - SIP protocol compliance tests
   - RFC3261 validation
   - Message format verification

7. **`test_helpers.go`**
   - Utility functions and helpers
   - SIP message builders
   - Test infrastructure

### Test Categories

#### 1. End-to-End SIP Call Flows
```go
func testEndToEndSIPCallFlows(t *testing.T, suite *TestSuite)
```
- **Complete REGISTER Flow**: Registration with authentication challenges
- **Complete INVITE Flow**: Session establishment with Session-Timer
- **OPTIONS Server Capabilities**: Server capability discovery
- **BYE Session Termination**: Proper session cleanup
- **ACK Request Handling**: No-response ACK processing

#### 2. Concurrent Registration and Session Handling
```go
func testConcurrentRegistrationAndSessionHandling(t *testing.T, suite *TestSuite)
```
- **Concurrent REGISTER Requests**: 15 simultaneous registrations
- **Concurrent INVITE Requests**: 10 simultaneous session establishments
- **Mixed Concurrent Operations**: Combined OPTIONS, REGISTER, INVITE operations

#### 3. Session-Timer Enforcement Integration
```go
func testSessionTimerEnforcementIntegration(t *testing.T, suite *TestSuite)
```
- **INVITE Requires SessionTimer**: Mandatory Session-Timer validation
- **SessionTimer Value Validation**: Min/max timer value enforcement
- **SessionTimer Refresher Parameter**: UAC/UAS refresher handling

#### 4. UDP/TCP Transport Protocol Handling
```go
func testUDPTCPTransportProtocolHandling(t *testing.T, suite *TestSuite)
```
- **UDP Message Processing**: Datagram-based message handling
- **TCP Message Processing**: Stream-based message handling
- **TCP Connection Reuse**: Multiple messages over single connection
- **Large Message Handling**: Size-appropriate transport selection
- **Parallel UDP/TCP Operations**: Simultaneous transport usage

#### 5. SIP Protocol Compliance (RFC3261)
```go
func testSIPProtocolComplianceRFC3261(t *testing.T, suite *TestSuite)
```
- **Required Headers Validation**: Missing header detection
- **Method Support Validation**: Supported vs unsupported methods
- **SIP Version Validation**: Version compatibility checking
- **Content Length Validation**: Message integrity verification

#### 6. Error Handling and Edge Cases
```go
func testErrorHandlingAndEdgeCases(t *testing.T, suite *TestSuite)
```
- **Malformed SIP Messages**: Invalid message format handling
- **Invalid URI Formats**: URI validation and error responses
- **Header Edge Cases**: Empty headers, long headers, case sensitivity

#### 7. Performance and Load Testing
```go
func testPerformanceAndLoadTesting(t *testing.T, suite *TestSuite)
```
- **Throughput Measurement**: Requests per second under load
- **Response Time Analysis**: Average, min, max response times
- **Concurrent Load Handling**: Performance under concurrent operations

## Test Execution

### Running All Tests
```bash
# Run comprehensive integration test suite
go test -v ./internal/integration -run TestComprehensiveIntegrationSuite

# Run all integration tests
go test -v ./internal/integration

# Run with coverage
go test -v -coverprofile=coverage.out ./internal/integration
```

### Running Specific Test Categories
```bash
# End-to-end call flows
go test -v ./internal/integration -run TestCompleteCallFlowScenarios

# Concurrent operations
go test -v ./internal/integration -run TestConcurrentRegistrationHandling
go test -v ./internal/integration -run TestConcurrentSessionHandling

# Session-Timer enforcement
go test -v ./internal/integration -run TestSessionTimerEnforcement

# Transport protocols
go test -v ./internal/integration -run TestUDPTransportProtocol
go test -v ./internal/integration -run TestTCPTransportProtocol

# Protocol compliance
go test -v ./internal/integration -run TestSIPProtocolCompliance
```

### Using the Test Runner Script
```bash
# Run all tests with the comprehensive test runner
./scripts/run_comprehensive_integration_tests.sh

# Run specific categories
./scripts/run_comprehensive_integration_tests.sh comprehensive
./scripts/run_comprehensive_integration_tests.sh end-to-end
./scripts/run_comprehensive_integration_tests.sh concurrent
./scripts/run_comprehensive_integration_tests.sh session-timer
./scripts/run_comprehensive_integration_tests.sh transport
./scripts/run_comprehensive_integration_tests.sh protocol
./scripts/run_comprehensive_integration_tests.sh performance

# Run with verbose output and coverage
./scripts/run_comprehensive_integration_tests.sh -v -c comprehensive
```

## Test Infrastructure

### Test Suite Setup
The `TestSuite` struct provides common infrastructure for all tests:
- Temporary server configuration
- UDP and TCP client connections
- Server lifecycle management
- Message sending utilities

### SIP Message Builders
Helper functions for creating test messages:
- `CreateREGISTERMessage()`: REGISTER request with contact
- `CreateINVITEMessage()`: INVITE request with Session-Timer
- `CreateOPTIONSMessage()`: OPTIONS request for capabilities
- `NewSIPMessageBuilder()`: Flexible message construction

### Response Parsing
Utilities for parsing and validating SIP responses:
- `NewSIPResponseParser()`: Response parsing utilities
- `GetStatusCode()`: Extract response status code
- `GetHeader()`: Extract specific header values
- `HasHeader()`: Check header presence

## Expected Results

### Success Criteria
- **Protocol Compliance**: All RFC3261 compliance tests pass
- **Session-Timer Enforcement**: Mandatory Session-Timer correctly enforced
- **Transport Reliability**: Both UDP and TCP handle messages correctly
- **Concurrent Handling**: Server handles concurrent operations without errors
- **Performance**: Achieves minimum throughput and response time targets

### Performance Targets
- **Throughput**: > 5 requests/second for OPTIONS requests
- **Response Time**: < 200ms average for simple requests
- **Error Rate**: < 20% under normal load conditions
- **Concurrent Connections**: Support for 10+ simultaneous operations

### Error Handling Validation
- **400 Bad Request**: Malformed messages properly rejected
- **401 Unauthorized**: Authentication challenges correctly issued
- **404 Not Found**: Unregistered users properly handled
- **405 Method Not Allowed**: Unsupported methods rejected with Allow header
- **421 Extension Required**: Missing Session-Timer properly detected
- **422 Session Interval Too Small**: Invalid timer values rejected

## Troubleshooting

### Common Issues

1. **Port Already in Use**
   - Ensure ports 5060 are available
   - Stop any running SIP server instances
   - Check for other applications using these ports

2. **Test Timeouts**
   - Increase test timeout with `-timeout` flag
   - Check server startup time
   - Verify network connectivity

3. **Authentication Failures**
   - Tests use dummy authentication for validation
   - Focus on authentication challenge/response flow
   - Verify WWW-Authenticate header presence

4. **Session-Timer Issues**
   - Verify Session-Timer implementation
   - Check min/max timer value configuration
   - Ensure proper header parsing

### Debug Mode
Enable verbose output for debugging:
```bash
go test -v -run TestSpecificTest ./internal/integration
```

View server logs during testing:
```bash
tail -f test_integration.log
```

## Continuous Integration

### Test Automation
The comprehensive test suite is designed for CI/CD integration:
- Automated server startup and shutdown
- Configurable timeouts and retry logic
- Coverage reporting
- Structured test output

### Test Reliability
Tests are designed to be reliable and repeatable:
- Isolated test environments
- Proper resource cleanup
- Timeout handling
- Error recovery

## Contributing

When adding new integration tests:

1. **Follow Naming Conventions**: Use descriptive test names with Test prefix
2. **Use Test Helpers**: Leverage existing helper functions in `test_helpers.go`
3. **Document Test Purpose**: Include clear descriptions of what each test verifies
4. **Handle Cleanup**: Ensure proper resource cleanup in test teardown
5. **Consider Concurrency**: Make tests safe for parallel execution
6. **Verify Requirements**: Map tests to specific requirements from the design document

## Test Coverage Matrix

| Requirement | Test Function | File | Status |
|-------------|---------------|------|--------|
| 5.1 | `TestConcurrentRegistrationHandling` | `comprehensive_integration_test.go` | ✅ |
| 5.2 | `TestConcurrentSessionHandling` | `comprehensive_integration_test.go` | ✅ |
| 7.1 | `TestUDPTransportProtocol` | `comprehensive_integration_test.go` | ✅ |
| 7.2 | `TestTCPTransportProtocol` | `comprehensive_integration_test.go` | ✅ |
| 7.3 | `TestTransportProtocolSelection` | `transport_test.go` | ✅ |
| 8.1 | `TestSessionTimerEnforcement` | `comprehensive_integration_test.go` | ✅ |
| 8.2 | `TestSessionTimerValidation` | `session_timer_test.go` | ✅ |

## Summary

This comprehensive integration test suite provides complete coverage of the SIP server functionality as specified in task 17. The tests validate:

- ✅ End-to-end SIP call flows
- ✅ Concurrent registration and session handling  
- ✅ Session-Timer enforcement integration
- ✅ UDP and TCP transport protocol handling
- ✅ SIP protocol compliance (RFC3261)
- ✅ Error handling and edge cases
- ✅ Performance characteristics under load

All requirements (5.1, 5.2, 7.1, 7.2, 7.3, 8.1, 8.2) are thoroughly tested with appropriate validation and error handling.