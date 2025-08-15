# Vultisig Relay

A high-performance communication relay service for routing Threshold Signature Scheme (TSS) communications in the Vultisig ecosystem, supporting both key generation (keygen) and key signing (keysign) operations.

## Overview

Vultisig Relay is a secure messaging relay server built in Go that facilitates communication between participants in TSS ceremonies. It provides a stateless, Redis-backed message routing system that enables secure multi-party computation for cryptocurrency operations.

## Features

- **Message Routing**: Routes messages between TSS participants during keygen/keysign ceremonies
- **Session Management**: Creates and manages TSS sessions with participant tracking
- **Redis Integration**: Uses Redis for scalable message storage and retrieval
- **Payload Handling**: Secure hash-verified payload message handling
- **RESTful API**: Clean HTTP API for all operations
- **Docker Support**: Containerized deployment with Docker Compose
- **High Performance**: Built with Echo framework for optimal performance
- **CORS Support**: Cross-origin resource sharing for web applications

## Architecture

The relay consists of several key components:

- **Server Layer** (`server/handler.go`): HTTP handlers for API endpoints
- **Storage Layer** (`storage/`): Abstracted storage interface with Redis implementation
- **Models** (`model/`): Data structures for messages and sessions
- **Configuration** (`config/config.go`): JSON-based configuration management

## API Endpoints

### Session Management
- `POST /:sessionID` - Start a new TSS session
- `GET /:sessionID` - Get session participants
- `DELETE /:sessionID` - Delete a session

### Message Operations
- `POST /message/:sessionID` - Post a message to a session
- `GET /message/:sessionID/:participantID` - Get messages for a participant
- `DELETE /message/:sessionID/:participantID/:hash` - Delete a specific message

### TSS Operations
- `POST /start/:sessionID` - Mark TSS session as started
- `GET /start/:sessionID` - Get TSS session start status
- `POST /complete/:sessionID` - Mark TSS session as complete
- `GET /complete/:sessionID` - Get TSS session completion status

### Keysign Operations
- `POST /complete/:sessionID/keysign` - Mark keysign as finished
- `GET /complete/:sessionID/keysign` - Get keysign completion status

### Payload Operations
- `POST /payload/:hash` - Store payload with hash verification
- `GET /payload/:hash` - Retrieve payload by hash

### Setup Messages
- `POST /setup-message/:sessionID` - Post setup message
- `GET /setup-message/:sessionID` - Get setup message

### Health Check
- `GET /ping` - Health check endpoint

## Quick Start

### Prerequisites

- Go 1.21.7 or higher
- Redis server
- Docker and Docker Compose (optional)

### Configuration

Create a `config.json` file:

```json
{
  "port": 8080,
  "redis_server": {
    "addr": "localhost:6379",
    "user": "",
    "password": "",
    "db": 0
  }
}
```

### Running with Docker Compose

```bash
# Start Redis and configure environment
docker-compose up -d

# Set Redis port (if different from default)
export REDIS_PORT=6379
```

### Building and Running

```bash
# Build the application
go build -o vultisig-relay cmd/router/main.go

# Run with custom config
./vultisig-relay -config config.json
```

### Development

```bash
# Install dependencies
go mod download

# Run directly
go run cmd/router/main.go -config config.json
```

## Configuration Options

| Field | Type | Description |
|-------|------|-------------|
| `port` | int64 | HTTP server port |
| `redis_server.addr` | string | Redis server address |
| `redis_server.user` | string | Redis username (optional) |
| `redis_server.password` | string | Redis password (optional) |
| `redis_server.db` | int | Redis database number |

## Message Flow

1. **Session Creation**: Clients create a TSS session with participant list
2. **Message Exchange**: Participants send encrypted messages through the relay
3. **Message Retrieval**: Participants poll for messages addressed to them
4. **Session Cleanup**: Sessions and messages expire automatically or can be deleted

## Security Features

- **Hash Verification**: Payload messages are verified against SHA-256 hashes
- **Message Deduplication**: Prevents duplicate message storage
- **Automatic Expiration**: Messages and sessions expire to prevent data leakage
- **Context Cancellation**: Proper handling of request cancellations

## Storage

The relay supports two storage backends:

- **Redis Storage** (default): Production-ready with persistence and clustering support
- **In-Memory Storage**: For testing and development environments

Redis storage includes:
- Automatic expiration (5 minutes for sessions, 1 hour for user data)
- Message deduplication
- List-based message queues
- Key-value storage for payloads

## Dependencies

- [Echo](https://github.com/labstack/echo) - HTTP web framework
- [Redis Go Client](https://github.com/redis/go-redis) - Redis client
- [Go Cache](https://github.com/patrickmn/go-cache) - In-memory caching

## Development

### Project Structure

```
vultisig-relay/
├── cmd/router/           # Application entry point
├── config/              # Configuration management
├── contexthelper/       # Context utilities
├── model/              # Data models
├── server/             # HTTP handlers
├── storage/            # Storage layer
├── docker-compose.yml  # Docker configuration
├── go.mod             # Go module definition
└── README.md          # This file
```

### Code Quality

The project includes:
- Qodana static analysis configuration
- Proper error handling throughout
- Context cancellation support
- Comprehensive logging

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Support

For issues and questions, please open an issue on the GitHub repository.
