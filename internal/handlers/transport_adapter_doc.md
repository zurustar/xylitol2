# TransportAdapter

The TransportAdapter is a bridge component that integrates the validation chain with the transport layer for SIP message processing.

## Purpose

The TransportAdapter serves as the main entry point for incoming SIP messages from the transport layer. It:

1. Receives raw SIP message data from the transport layer
2. Parses the messages using the SIP message parser
3. Routes requests through the validation chain and method handlers
4. Manages SIP transactions for both requests and responses
5. Handles error responses when validation or processing fails

## Architecture

```
Transport Layer → TransportAdapter → ValidationChain → MethodHandlers
                       ↓
                 TransactionManager
```

## Key Components

### Dependencies

- **HandlerManager**: Manages method handlers and validation chain (typically ValidatedManager)
- **TransactionManager**: Creates and manages SIP transactions
- **MessageParser**: Parses raw SIP message data
- **TransportManager**: Handles UDP/TCP transport operations

### Main Functions

- `HandleMessage(data []byte, transport string, addr net.Addr)`: Main entry point for incoming messages
- `RegisterMethodHandler(handler MethodHandler)`: Registers SIP method handlers
- `Start()`: Initializes the adapter and registers with transport layer
- `GetSupportedMethods()`: Returns list of supported SIP methods

## Usage Example

```go
// Create components
validatedManager := handlers.NewValidatedManager()
transactionManager := transaction.NewManager()
messageParser := parser.NewParser()
transportManager := transport.NewManager()

// Create and configure TransportAdapter
adapter := handlers.NewTransportAdapter(
    validatedManager,
    transactionManager, 
    messageParser,
    transportManager,
)

// Register method handlers
inviteHandler := handlers.NewInviteHandler()
registerHandler := handlers.NewRegisterHandler()

adapter.RegisterMethodHandler(inviteHandler)
adapter.RegisterMethodHandler(registerHandler)

// Start the adapter
err := adapter.Start()
if err != nil {
    log.Fatal("Failed to start transport adapter:", err)
}

// The adapter is now ready to handle incoming SIP messages
```

## Message Flow

### Request Processing

1. Transport layer receives SIP message data
2. TransportAdapter.HandleMessage() is called
3. Message is parsed using MessageParser
4. Transaction is found or created using TransactionManager
5. Request is processed through HandlerManager (includes validation chain)
6. If validation fails, error response is sent via transaction
7. If validation passes, request is routed to appropriate method handler

### Response Processing

1. Transport layer receives SIP response data
2. TransportAdapter.HandleMessage() is called
3. Message is parsed using MessageParser
4. Existing transaction is found using TransactionManager
5. Response is processed through the transaction

## Error Handling

The TransportAdapter handles several types of errors:

- **Parse Errors**: When SIP message parsing fails
- **Transaction Errors**: When no transaction is found for responses
- **Handler Errors**: When method handler processing fails
- **Validation Errors**: Handled by the validation chain in HandlerManager

For handler errors, the adapter automatically generates a 500 Internal Server Error response.

## Integration with Validation Chain

The TransportAdapter works seamlessly with the ValidatedManager, which includes:

- Syntax validation
- Session-Timer validation (with proper priority)
- Authentication validation
- Custom validation rules

The validation chain is executed before method handlers, ensuring that only valid requests reach the business logic.

## Testing

The TransportAdapter includes comprehensive unit tests covering:

- Message parsing and routing
- Transaction management
- Error handling scenarios
- Integration with validation chain
- Method handler registration

Tests are located in `test/transport_adapter/` to avoid conflicts with existing test infrastructure.