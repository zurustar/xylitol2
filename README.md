# SIP Server

A RFC3261-compliant SIP Server implemented in Go that functions as both a Stateful Proxy and Registrar with mandatory Session-Timer functionality (RFC4028).

## Project Structure

```
├── cmd/
│   └── sipserver/          # Main application entry point
├── internal/
│   ├── config/             # Configuration management
│   ├── database/           # Database interfaces and user management
│   ├── logging/            # Structured logging interfaces
│   ├── parser/             # SIP message parsing and serialization
│   ├── proxy/              # SIP proxy functionality
│   ├── registrar/          # SIP registration services
│   ├── server/             # Main server coordination
│   ├── sessiontimer/       # Session timer management (RFC4028)
│   ├── transaction/        # SIP transaction management
│   ├── transport/          # UDP/TCP transport layer
│   └── webadmin/           # Web-based administration interface
├── config.sample.yaml      # Sample configuration file
├── go.mod                  # Go module definition
└── README.md               # This file
```

## Features

- RFC3261 compliant SIP proxy and registrar
- UDP and TCP transport support
- Mandatory Session-Timer enforcement (RFC4028)
- SQLite-based persistent storage
- Web-based user management interface
- Digest authentication
- Configurable server parameters

## Dependencies

- `modernc.org/sqlite` - Pure-Go SQLite database driver
- `github.com/gorilla/mux` - HTTP router for web admin interface
- `gopkg.in/yaml.v2` - YAML configuration file parsing

## Configuration

The server is configured via a YAML configuration file. See `config.sample.yaml` for an example configuration.

## Building and Running

```bash
# Build the server
go build -o sipserver cmd/sipserver/main.go

# Run with default configuration
./sipserver

# Run with custom configuration
./sipserver -config /path/to/config.yaml
```

## Development Status

This project is currently in development. The core interfaces and project structure have been established. Implementation of individual components is in progress.
