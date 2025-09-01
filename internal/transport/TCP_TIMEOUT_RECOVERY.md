# TCP Timeout and Error Recovery Implementation

This document describes the enhanced TCP timeout handling and error recovery functionality implemented for the SIP server.

## Overview

The enhanced TCP transport now includes comprehensive timeout handling and error recovery mechanisms to ensure reliable SIP message processing over TCP connections, addressing requirements 20.1 and 20.5.

## Features Implemented

### 1. Configurable Timeouts

- **Read Timeout**: Configurable timeout for reading SIP messages from TCP connections
- **Write Timeout**: Configurable timeout for sending SIP messages over TCP
- **Idle Timeout**: Configurable timeout for idle connection cleanup
- **Accept Timeout**: Configurable timeout for accepting new connections

### 2. Enhanced Error Recovery

#### Timeout Recovery
- **Automatic Retry**: Failed operations due to timeouts are automatically retried
- **Progressive Timeout**: Timeout values are progressively increased for retry attempts
- **Max Retry Limit**: Configurable maximum number of timeout retries before giving up
- **Recovery Delay**: Configurable delay between timeout recovery attempts

#### Connection Recovery
- **Connection Retry**: Failed connections are automatically retried with backoff
- **Exponential Backoff**: Retry delays increase exponentially with configurable multiplier
- **Max Retry Delay**: Configurable maximum delay between retry attempts
- **Recoverable Error Detection**: Automatic detection of recoverable vs non-recoverable errors

### 3. Detailed Error Logging

- **Contextual Logging**: Detailed error messages with connection IDs, addresses, and timing
- **Error Classification**: Errors are classified by type (timeout, connection, read, write)
- **Recovery Logging**: Detailed logging of recovery attempts and outcomes
- **Performance Metrics**: Logging of operation durations and timeout values

### 4. Error Statistics

- **Real-time Tracking**: Real-time tracking of error counts by type
- **Recovery Metrics**: Tracking of recovery attempts and success rates
- **Timestamp Tracking**: Last error and recovery timestamps
- **Statistics API**: Programmatic access to error statistics

## Configuration

### Default Configuration

```go
config := DefaultEnhancedTCPConfig()
// Timeout settings
config.ReadTimeout = 30 * time.Second
config.WriteTimeout = 30 * time.Second
config.IdleTimeout = 5 * time.Minute

// Retry settings
config.MaxRetries = 3
config.RetryDelay = 1 * time.Second
config.MaxRetryDelay = 30 * time.Second
config.BackoffMultiplier = 2.0

// Recovery settings
config.TimeoutRecoveryEnabled = true
config.TimeoutRecoveryDelay = 500 * time.Millisecond
config.MaxTimeoutRetries = 5
config.ConnectionRecoveryEnabled = true
config.ConnectionRecoveryDelay = 2 * time.Second

// Logging and statistics
config.DetailedErrorLogging = true
config.ErrorStatistics = true
```

### Custom Configuration

```go
config := &EnhancedTCPConfig{
    ReadTimeout:               1 * time.Second,
    WriteTimeout:              2 * time.Second,
    MaxRetries:                5,
    RetryDelay:                200 * time.Millisecond,
    BackoffMultiplier:         1.5,
    TimeoutRecoveryEnabled:    true,
    MaxTimeoutRetries:         3,
    ConnectionRecoveryEnabled: true,
    DetailedErrorLogging:      true,
    ErrorStatistics:           true,
    Logger:                    myLogger,
}
```

## Error Types and Recovery

### Recoverable Errors

1. **Timeout Errors**: Network timeouts that may succeed on retry
2. **Connection Refused**: Temporary connection issues
3. **Connection Reset**: Network interruptions
4. **Temporary Network Errors**: Transient network problems

### Non-Recoverable Errors

1. **Invalid Address**: Malformed network addresses
2. **Permission Denied**: Security or permission issues
3. **Protocol Errors**: Fundamental protocol violations

## Usage Examples

### Basic Usage

```go
transport := NewEnhancedTCPTransport(nil) // Uses default config
err := transport.Start(5060)
if err != nil {
    log.Fatal(err)
}
defer transport.Stop()

// Send message with automatic retry and recovery
message := []byte("INVITE sip:user@example.com SIP/2.0\r\n...")
addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:5060")
err = transport.SendMessage(message, addr)
if err != nil {
    log.Printf("Failed to send message after retries: %v", err)
}
```

### Monitoring Error Statistics

```go
stats := transport.GetErrorStatistics()
fmt.Printf("Timeout errors: %d\n", stats["timeout_errors"])
fmt.Printf("Connection errors: %d\n", stats["connection_errors"])
fmt.Printf("Recovery attempts: %d\n", stats["recovery_attempts"])
fmt.Printf("Successful recoveries: %d\n", stats["successful_recoveries"])
```

### Dynamic Timeout Adjustment

```go
// Adjust timeouts based on network conditions
transport.SetTimeouts(
    5*time.Second,  // read timeout
    10*time.Second, // write timeout
    30*time.Minute, // idle timeout
)
```

## Implementation Details

### Error Recovery Flow

1. **Error Detection**: Errors are detected and classified by type
2. **Recoverability Check**: Determine if error is recoverable
3. **Retry Logic**: Apply appropriate retry strategy with backoff
4. **Recovery Attempt**: Execute recovery with adjusted parameters
5. **Statistics Update**: Update error and recovery statistics
6. **Logging**: Log detailed information about the recovery process

### Timeout Handling

1. **Progressive Timeouts**: Timeout values increase for retry attempts
2. **Timeout Capping**: Maximum timeout limits prevent excessive delays
3. **Context Cancellation**: Proper handling of context cancellation
4. **Resource Cleanup**: Automatic cleanup of timed-out resources

## Testing

The implementation includes comprehensive tests covering:

- Timeout recovery scenarios
- Connection error recovery
- Error statistics tracking
- Configuration validation
- Integration testing
- Performance under load

Run tests with:
```bash
go test ./internal/transport -run "TestTCPTimeout|TestEnhancedTCPTransport_.*Recovery"
```

## Performance Considerations

- **Minimal Overhead**: Error tracking adds minimal performance overhead
- **Efficient Retry**: Exponential backoff prevents excessive retry attempts
- **Resource Management**: Proper cleanup prevents resource leaks
- **Concurrent Safety**: All operations are thread-safe

## Compliance

This implementation addresses the following requirements:

- **Requirement 20.1**: Process TCP messages without timeout errors through automatic retry and recovery
- **Requirement 20.5**: Log detailed error information and recover gracefully from TCP processing errors

The implementation ensures reliable TCP message processing while providing comprehensive error handling and recovery capabilities.