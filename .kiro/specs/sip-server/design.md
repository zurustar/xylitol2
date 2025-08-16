# SIP Server Design Document

## Overview

This document describes the design for a RFC3261-compliant SIP Server implemented in Go. The server functions as both a Stateful Proxy and Registrar, supporting UDP and TCP transport protocols with mandatory Session-Timer functionality (RFC4028). The architecture follows a modular design pattern to ensure maintainability, scalability, and compliance with SIP specifications.

The server will handle SIP message routing, user registration, session management, and transaction state maintenance while enforcing Session-Timer requirements for all established sessions.

## Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   SIP Clients   │◄──►│   SIP Server    │◄──►│   SIP Clients   │
│  (UDP/TCP)      │    │  (Proxy/Reg)    │    │  (UDP/TCP)      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  Configuration  │
                    │   & Storage     │
                    └─────────────────┘
```

### Core Components

1. **Transport Layer**: Handles UDP and TCP connections
2. **Message Parser**: Parses and validates SIP messages
3. **Transaction Manager**: Manages SIP transactions and state
4. **Database Layer**: SQLite-based persistent storage for users and contacts
5. **User Manager**: Handles user credential management and authentication
6. **Registrar**: Handles user registration and location services
7. **Proxy Engine**: Routes messages between clients
8. **Session Timer Manager**: Enforces RFC4028 session timers
9. **Web Admin Interface**: Web-based user management interface
10. **Configuration Manager**: Manages server configuration

## Components and Interfaces

### Transport Layer
```go
type TransportManager interface {
    StartUDP(port int) error
    StartTCP(port int) error
    SendMessage(msg *SIPMessage, transport string, addr net.Addr) error
    RegisterHandler(handler MessageHandler)
    Stop() error
}
```

### Message Parser
```go
type MessageParser interface {
    Parse(data []byte) (*SIPMessage, error)
    Serialize(msg *SIPMessage) ([]byte, error)
    Validate(msg *SIPMessage) error
}
```

### Transaction Manager
```go
type TransactionManager interface {
    CreateTransaction(msg *SIPMessage) Transaction
    FindTransaction(msg *SIPMessage) Transaction
    CleanupExpired()
}

type Transaction interface {
    GetState() TransactionState
    ProcessMessage(msg *SIPMessage) error
    SendResponse(response *SIPMessage) error
    SetTimer(duration time.Duration, callback func())
}
```###
 Registrar
```go
type Registrar interface {
    Register(contact *Contact, expires int) error
    Unregister(aor string) error
    FindContacts(aor string) ([]*Contact, error)
    CleanupExpired()
}

type Contact struct {
    AOR        string
    URI        string
    Expires    time.Time
    CallID     string
    CSeq       uint32
}
```

### Proxy Engine
```go
type ProxyEngine interface {
    ProcessRequest(req *SIPMessage, transaction Transaction) error
    ForwardRequest(req *SIPMessage, targets []*Contact) error
    ProcessResponse(resp *SIPMessage, transaction Transaction) error
}
```

### Database Layer
```go
type DatabaseManager interface {
    Initialize() error
    Close() error
    CreateUser(user *User) error
    GetUser(username, realm string) (*User, error)
    UpdateUser(user *User) error
    DeleteUser(username, realm string) error
    ListUsers() ([]*User, error)
}

type User struct {
    ID           int64
    Username     string
    Realm        string
    PasswordHash string
    Enabled      bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### User Manager
```go
type UserManager interface {
    CreateUser(username, realm, password string) error
    AuthenticateUser(username, realm, password string) bool
    UpdatePassword(username, realm, newPassword string) error
    DeleteUser(username, realm string) error
    ListUsers() ([]*User, error)
    GeneratePasswordHash(username, realm, password string) string
}
```

### Web Admin Interface
```go
type WebAdminServer interface {
    Start(port int) error
    Stop() error
    RegisterRoutes()
}

type UserHandler struct {
    userManager UserManager
}

// HTTP endpoints:
// GET /admin/users - List all users
// POST /admin/users - Create new user
// PUT /admin/users/{id} - Update user
// DELETE /admin/users/{id} - Delete user
// GET /admin - Admin dashboard
```

### Session Timer Manager
```go
type SessionTimerManager interface {
    CreateSession(callID string, sessionExpires int) *Session
    RefreshSession(callID string) error
    CleanupExpiredSessions()
    IsSessionTimerRequired(msg *SIPMessage) bool
}

type Session struct {
    CallID         string
    SessionExpires time.Time
    Refresher      string
    MinSE          int
}
```

## Data Models

### SIP Message Structure
```go
type SIPMessage struct {
    StartLine   StartLine
    Headers     map[string][]string
    Body        []byte
    Transport   string
    Source      net.Addr
    Destination net.Addr
}

type StartLine interface {
    String() string
}

type RequestLine struct {
    Method     string
    RequestURI string
    Version    string
}

type StatusLine struct {
    Version    string
    StatusCode int
    ReasonPhrase string
}
```

### Database Schema

#### Users Table
```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    realm TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(username, realm)
);
```

#### Contacts Table (for registrar)
```sql
CREATE TABLE contacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    aor TEXT NOT NULL,
    contact_uri TEXT NOT NULL,
    expires DATETIME NOT NULL,
    call_id TEXT NOT NULL,
    cseq INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX(aor),
    INDEX(expires)
);
```

### Registration Database
```go
type RegistrationDB interface {
    Store(contact *Contact) error
    Retrieve(aor string) ([]*Contact, error)
    Delete(aor string, contactURI string) error
    CleanupExpired() error
}
```

## Error Handling

### Error Categories
1. **Transport Errors**: Network connectivity issues, malformed packets
2. **Parse Errors**: Invalid SIP message format, missing required headers
3. **Database Errors**: SQLite connection issues, constraint violations
4. **Authentication Errors**: Invalid credentials, expired nonces
5. **Session Timer Errors**: Missing Session-Expires, invalid timer values
6. **Web Interface Errors**: HTTP request validation, form processing errors
7. **Proxy Errors**: Target not found, forwarding failures

### Error Response Strategy
- **400 Bad Request**: Malformed SIP messages
- **401 Unauthorized**: Authentication required
- **404 Not Found**: User not registered
- **421 Extension Required**: Session-Timer not supported
- **503 Service Unavailable**: Server overload or maintenance

### Logging Strategy
```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
}
```

## Testing Strategy

### Unit Testing
- Individual component testing with mocked dependencies
- Message parsing and serialization validation
- Transaction state machine testing
- Session timer logic verification

### Integration Testing
- End-to-end SIP call flow testing
- UDP/TCP transport protocol testing
- Registration and proxy functionality testing
- Session timer enforcement testing

### Load Testing
- Concurrent registration handling
- Multiple simultaneous call processing
- Memory and connection limit testing
- Performance under high load conditions

### Compliance Testing
- RFC3261 compliance verification
- RFC4028 Session-Timer compliance
- SIP torture test suite execution
- Interoperability testing with standard SIP clients

### Web Interface Testing
- User management functionality testing
- Form validation and error handling
- Database integration testing
- Cross-browser compatibility testing
## 5. 残
課題への対応設計

### 5.1 Session-Timer検証優先順位の改善

#### 問題
現在の実装では認証処理が先に実行されるため、Session-Timer検証が後回しになっている。RFC4028の要件に従い、適切な優先順位で処理する必要がある。

#### 設計方針
```go
type MessageProcessor interface {
    ProcessRequest(req *SIPMessage) (*SIPMessage, error)
}

type ValidationChain struct {
    validators []RequestValidator
}

type RequestValidator interface {
    Validate(req *SIPMessage) ValidationResult
    Priority() int
}

type SessionTimerValidator struct{}
type AuthenticationValidator struct{}
```

#### 処理フロー
1. **基本検証**: SIPメッセージの構文チェック
2. **Session-Timer検証**: INVITEリクエストのSession-Timer要件チェック
3. **認証検証**: 認証が必要な場合の認証チェック
4. **ビジネスロジック**: プロキシ/レジストラー処理

### 5.2 TCP処理の改善

#### 問題
TCPトランスポートでタイムアウトが発生し、メッセージ処理が不安定になっている。

#### 設計方針
```go
type TCPConnectionManager struct {
    connections map[string]*TCPConnection
    readTimeout time.Duration
    writeTimeout time.Duration
    idleTimeout time.Duration
}

type TCPConnection struct {
    conn net.Conn
    reader *bufio.Reader
    writer *bufio.Writer
    lastActivity time.Time
    messageBuffer []byte
}

type TCPMessageFramer interface {
    ReadMessage(conn *TCPConnection) (*SIPMessage, error)
    WriteMessage(conn *TCPConnection, msg *SIPMessage) error
}
```

#### 改善点
- **接続プール管理**: 接続の再利用とライフサイクル管理
- **メッセージフレーミング**: Content-Lengthベースの適切なメッセージ境界検出
- **タイムアウト管理**: 読み取り/書き込み/アイドルタイムアウトの適切な設定
- **エラーハンドリング**: 接続エラーからの回復処理

### 5.3 エラーハンドリングの完全実装

#### 問題
不正なメッセージに対する適切なエラーレスポンスが不完全で、クライアントが問題を特定しにくい。

#### 設計方針
```go
type ErrorHandler interface {
    HandleParseError(err error, rawMessage []byte) *SIPMessage
    HandleValidationError(err ValidationError, req *SIPMessage) *SIPMessage
    HandleProcessingError(err error, req *SIPMessage) *SIPMessage
}

type ValidationError struct {
    Code        int
    Reason      string
    Header      string
    Details     string
    Suggestions []string
}

type ErrorResponseBuilder struct {
    templates map[int]ResponseTemplate
}
```

#### エラーカテゴリと対応
- **400 Bad Request**: 構文エラー、必須ヘッダー不足
- **405 Method Not Allowed**: サポートされていないメソッド
- **421 Extension Required**: Session-Timer未対応
- **422 Session Interval Too Small**: Session-Timer値が小さすぎる
- **500 Internal Server Error**: サーバー内部エラー
- **503 Service Unavailable**: サーバー過負荷

## 6. 実装優先順位

### Phase 1: Session-Timer優先順位修正
1. ValidationChainの実装
2. RequestValidatorインターフェースの実装
3. 優先順位ベースの検証フローの実装

### Phase 2: TCP処理改善
1. TCPConnectionManagerの実装
2. メッセージフレーミングの改善
3. タイムアウト管理の実装

### Phase 3: エラーハンドリング完全実装
1. ErrorHandlerインターフェースの実装
2. 詳細なエラーレスポンス生成
3. エラーログの改善