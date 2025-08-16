# Implementation Plan

- [x] 1. Set up project structure and core interfaces
  - Create directory structure for transport, parser, transaction, registrar, proxy, session timer, web admin, and database components
  - Define core interfaces for all major components (TransportManager, MessageParser, TransactionManager, UserManager, etc.)
  - Set up Go module dependencies including Pure-Go SQLite library (modernc.org/sqlite) and web framework
  - _Requirements: 11.1, 11.2_

- [x] 2. Implement SIP message data models and parsing
  - [x] 2.1 Create SIP message data structures
    - Implement SIPMessage, RequestLine, StatusLine, and Header structures
    - Create message type constants and enums for SIP methods and response codes
    - Write unit tests for data structure creation and field access
    - _Requirements: 4.1, 4.4_

  - [x] 2.2 Implement SIP message parser
    - Write parser for SIP request and response messages
    - Implement header parsing with proper handling of multi-value headers
    - Add message validation according to RFC3261 syntax rules
    - Create comprehensive unit tests for parsing various SIP message formats
    - _Requirements: 4.1, 4.4, 2.1_

  - [x] 2.3 Implement SIP message serialization
    - Write serializer to convert SIPMessage structures back to wire format
    - Ensure proper header ordering and formatting
    - Add unit tests for serialization and round-trip parsing
    - _Requirements: 4.1, 2.2_

- [x] 3. Implement transport layer for UDP and TCP
  - [x] 3.1 Create UDP transport handler
    - Implement UDP listener and message receiving functionality
    - Add UDP message sending with proper destination handling
    - Write unit tests for UDP transport operations
    - _Requirements: 7.1, 7.2, 7.4_

  - [x] 3.2 Create TCP transport handler
    - Implement TCP listener with connection management
    - Add TCP message framing and parsing for stream-based transport
    - Handle connection lifecycle and cleanup
    - Write unit tests for TCP transport operations
    - _Requirements: 7.1, 7.3, 7.5_

  - [x] 3.3 Implement transport manager
    - Create unified transport manager that handles both UDP and TCP
    - Add message routing to appropriate transport handlers
    - Implement transport selection logic based on message size and client preference
    - Write integration tests for multi-transport scenarios
    - _Requirements: 7.1, 7.2, 7.3_

- [x] 4. Implement transaction management
  - [x] 4.1 Create transaction state machine
    - Implement client and server transaction state machines per RFC3261
    - Add timer management for transaction timeouts (Timer A, B, D, E, F, G, H, I, J, K)
    - Create transaction state transition logic
    - Write unit tests for state machine transitions
    - _Requirements: 5.4, 2.2, 3.2_

  - [x] 4.2 Implement transaction manager
    - Create transaction creation and lookup functionality
    - Add transaction matching based on branch parameter and method
    - Implement transaction cleanup and garbage collection
    - Write unit tests for transaction lifecycle management
    - _Requirements: 5.4, 2.2_

- [x] 5. Implement database layer with SQLite
  - [x] 5.1 Create database schema and connection management
    - Design SQLite database schema for users, contacts, and sessions
    - Implement database connection management with Pure-Go SQLite (modernc.org/sqlite)
    - Add database initialization and migration functionality
    - Write unit tests for database operations
    - _Requirements: 9.1, 9.2_

  - [x] 5.2 Implement user management database operations
    - Create user CRUD operations (Create, Read, Update, Delete)
    - Add user authentication credential storage (username, realm, password hash)
    - Implement user lookup and validation functions
    - Write unit tests for user management operations
    - _Requirements: 9.3, 9.4_

- [x] 6. Implement authentication system
  - [x] 6.1 Create digest authentication handler
    - Implement RFC2617 digest authentication algorithm
    - Add nonce generation and validation functionality
    - Integrate with user database for credential verification
    - Write unit tests for authentication flows
    - _Requirements: 1.1, 1.5_

  - [x] 6.2 Integrate authentication with message processing
    - Add authentication challenges for REGISTER and INVITE requests
    - Implement authentication header parsing and validation
    - Add proper error responses for authentication failures
    - Write integration tests for authenticated request flows
    - _Requirements: 1.1, 1.5_

- [x] 7. Implement web-based administration interface
  - [x] 7.1 Create web server and routing
    - Implement HTTP server with REST API endpoints
    - Add web interface routing for user management operations
    - Create basic HTML templates for administration interface
    - Write unit tests for web server functionality
    - _Requirements: 10.1, 10.2_

  - [x] 7.2 Implement user management web interface
    - Create web forms for user creation, editing, and deletion
    - Add user listing and search functionality
    - Implement password change and user status management
    - Add input validation and error handling for web forms
    - Write integration tests for web interface operations
    - _Requirements: 10.3, 10.4_

- [x] 8. Implement registrar functionality
  - [x] 8.1 Create contact storage system
    - Implement SQLite-based contact database with expiration handling
    - Add contact registration, update, and removal operations
    - Create contact lookup functionality by AOR (Address of Record)
    - Write unit tests for contact storage operations
    - _Requirements: 1.2, 1.3, 1.4_

  - [x] 8.2 Implement REGISTER request processing
    - Add REGISTER request validation and authentication
    - Implement contact header parsing and storage
    - Add support for contact expiration and removal
    - Handle multiple contacts per AOR
    - Write unit tests for REGISTER processing
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 9. Implement Session-Timer functionality
  - [x] 9.1 Create session timer manager
    - Implement session storage with timer tracking
    - Add session creation with Session-Expires header processing
    - Create session refresh and cleanup functionality
    - Write unit tests for session timer operations
    - _Requirements: 8.1, 8.3, 8.5_

  - [x] 9.2 Implement Session-Timer enforcement
    - Add Session-Expires header validation in INVITE requests
    - Implement rejection of requests without Session-Timer support
    - Add automatic session termination on timer expiration
    - Write unit tests for Session-Timer enforcement
    - _Requirements: 8.1, 8.2, 8.4_

- [x] 10. Implement proxy functionality
  - [x] 10.1 Create request forwarding engine
    - Implement target resolution using registrar database
    - Add request forwarding with proper Via header handling
    - Create response routing back to originating client
    - Write unit tests for request/response forwarding
    - _Requirements: 2.2, 2.3, 2.5_

  - [x] 10.2 Implement proxy state management
    - Add stateful proxy transaction correlation
    - Implement forking for multiple registered contacts
    - Handle response aggregation and best response selection
    - Write unit tests for stateful proxy operations
    - _Requirements: 2.2, 2.5, 5.4_

- [x] 11. Implement SIP method handlers
  - [x] 11.1 Create INVITE/ACK/BYE handler
    - Implement INVITE request processing with Session-Timer validation
    - Add ACK request handling within established dialogs
    - Create BYE request processing for session termination
    - Write unit tests for session establishment and termination flows
    - _Requirements: 2.1, 2.2, 3.2, 3.3, 8.1, 8.2_

  - [x] 11.2 Create OPTIONS/INFO handler
    - Implement OPTIONS request processing with capability advertisement
    - Add INFO request forwarding within established dialogs
    - Handle unsupported methods with proper error responses
    - Write unit tests for auxiliary method handling
    - _Requirements: 4.1, 4.3, 4.4_

- [x] 12. Implement configuration and logging
  - [x] 12.1 Create configuration management
    - Implement configuration file parsing for server settings
    - Add UDP/TCP port configuration and binding
    - Create authentication and Session-Timer parameter configuration
    - Write unit tests for configuration loading and validation
    - _Requirements: 11.1, 11.2, 11.3, 11.4_

  - [x] 12.2 Implement logging system
    - Create structured logging with appropriate detail levels
    - Add transaction and session event logging
    - Implement error logging with context information
    - Write unit tests for logging functionality
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_

- [x] 13. Create main server application
  - [x] 13.1 Implement server startup and initialization
    - Create main server struct that coordinates all components
    - Add graceful startup with configuration loading and validation
    - Implement component initialization in proper order
    - Write integration tests for server startup
    - _Requirements: 11.1, 11.5_

  - [x] 13.2 Implement server shutdown and cleanup
    - Add graceful shutdown handling with proper resource cleanup
    - Implement signal handling for clean server termination
    - Create connection and transaction cleanup on shutdown
    - Write integration tests for server shutdown scenarios
    - _Requirements: 5.3_

- [x] 14. Session-Timer検証優先順位の修正
  - [x] 14.1 ValidationChainの実装
    - RequestValidatorインターフェースの定義
    - 優先順位ベースの検証チェーンの実装
    - SessionTimerValidatorとAuthenticationValidatorの分離
    - _Requirements: 12.1, 12.2, 12.3_

  - [x] 14.2 Session-Timer優先検証の実装
    - INVITEリクエストでのSession-Timer事前検証
    - 421 Extension Required応答の認証前送信
    - RFC4028準拠の優先順位ルールの実装
    - _Requirements: 12.4, 12.5_

  - [x] 14.3 統合テストの実装
    - Session-Timer検証優先順位のテストケース作成
    - 認証とSession-Timer検証の組み合わせテスト
    - エラーケースの検証テスト
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5_

- [x] 15. TCP処理の改善
  - [x] 15.1 TCP接続管理の実装
    - TCPConnectionManagerの実装
    - 接続プールとライフサイクル管理
    - アイドル接続のタイムアウト処理
    - _Requirements: 13.2, 13.4_

  - [x] 15.2 TCPメッセージフレーミングの改善
    - Content-Lengthベースのメッセージ境界検出
    - 部分メッセージの適切な処理
    - ストリーミングデータの再組み立て
    - _Requirements: 13.3_

  - [x] 15.3 TCPタイムアウト処理の実装
    - 読み取り/書き込みタイムアウトの設定
    - タイムアウトエラーからの回復処理
    - 詳細なエラーログの実装
    - _Requirements: 13.1, 13.5_

- [-] 16. エラーハンドリングの完全実装
  - [x] 16.1 ErrorHandlerインターフェースの実装
    - 各種エラータイプに対応したハンドラーの実装
    - ValidationErrorの詳細な定義
    - ErrorResponseBuilderの実装
    - _Requirements: 14.1, 14.2_

  - [x] 16.2 詳細なエラーレスポンス生成
    - 400 Bad Requestの詳細なエラー情報付与
    - 必須ヘッダー不足の具体的な指摘
    - 405 Method Not Allowedの適切な処理
    - _Requirements: 14.3, 14.4_

  - [ ] 16.3 エラーログとモニタリングの改善
    - パースエラーの詳細ログ出力
    - エラー統計の収集
    - デバッグ情報の充実
    - _Requirements: 14.5_



- [ ] 17. Create comprehensive integration tests
  - Write end-to-end tests for complete SIP call flows
  - Add tests for concurrent registration and session handling
  - Create Session-Timer enforcement integration tests
  - Test UDP and TCP transport protocol handling
  - _Requirements: 5.1, 5.2, 7.1, 7.2, 7.3, 8.1, 8.2_
