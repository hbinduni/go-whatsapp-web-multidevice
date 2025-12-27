# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Building and Running

- **Build binary**: `cd src && go build -o whatsapp` (Linux/macOS) or `go build -o whatsapp.exe` (Windows)
- **Run REST API server**: `cd src && go run . rest` or `./whatsapp rest`
- **Run with Docker**: `docker-compose up -d --build`

### Testing

- **Run all tests**: `cd src && go test ./...`
- **Run specific package tests**: `cd src && go test ./validations`
- **Run tests with coverage**: `cd src && go test -cover ./...`

### Development

- **Format code**: `cd src && go fmt ./...`
- **Get dependencies**: `cd src && go mod tidy`
- **Check for issues**: `cd src && go vet ./...`

## Project Architecture

This is a Go-based WhatsApp Web REST API server with SSE real-time events, S3/MinIO media storage, and chat history synchronization.

### Core Architecture Pattern

- **Domain-Driven Design**: Business logic separated into domain packages (`domains/`)
- **Clean Architecture**: Clear separation between UI, use cases, and infrastructure layers
- **Cobra CLI**: Command pattern for the `rest` server mode

### Key Directories

- `src/`: Main source code directory
- `src/cmd/`: CLI commands (root, rest)
- `src/domains/`: Business domain logic (app, chat, group, message, send, user, newsletter)
- `src/infrastructure/`: External integrations (WhatsApp, database)
- `src/ui/`: User interface layers (REST API, SSE, WebSocket)
- `src/usecase/`: Application use cases bridging domains and UI
- `src/validations/`: Input validation logic
- `src/pkg/`: Shared utilities and helpers

### Configuration

- **Environment Variables**: See `.env.example` for all available options
- **Command Line Flags**: All env vars can be overridden with CLI flags
- **Config Priority**: CLI flags > Environment variables > `.env` file

### Database

- **Main DB**: WhatsApp connection data (SQLite by default, supports PostgreSQL)
- **Chat Storage**: Separate SQLite database for chat history (`storages/chatstorage.db`)
- **Database URIs**: Configurable via `DB_URI` and `DB_KEYS_URI` environment variables

### Key Dependencies

- `go.mau.fi/whatsmeow`: WhatsApp Web protocol implementation
- `github.com/gofiber/fiber/v2`: Web framework for REST API
- `github.com/spf13/cobra`: CLI framework
- `github.com/spf13/viper`: Configuration management
- `github.com/minio/minio-go/v7`: S3/MinIO storage client

### WhatsApp Integration

- Uses whatsmeow library for WhatsApp Web protocol
- Supports multi-device WhatsApp accounts
- Auto-reconnection and connection monitoring
- Media compression and webhook support

## Security Guidelines (PUBLIC REPO)

**This is a public repository. Never commit secrets or credentials.**

### What NOT to commit:
- Passwords, API keys, access tokens
- S3/MinIO credentials (access key, secret key)
- Database connection strings with passwords
- Private endpoints or internal URLs
- `.env` files (use `.env.example` with placeholders)

### Use placeholders in documentation:
```
# Good
mc alias set mys3 https://your-s3-endpoint.com YOUR_ACCESS_KEY YOUR_SECRET_KEY

# Bad - real credentials
mc alias set mys3 https://s3.example.com admin realpassword123
```

### If credentials are accidentally committed:
1. Rotate the credentials immediately
2. Use `git-filter-repo` to remove from history
3. Force push to update remote
4. Contact GitHub to purge cached commits if needed

### Environment variables for scripts:
Scripts should use environment variables, not hardcoded values:
```bash
WA_USER=xxx WA_PASS=yyy bun run script.ts
```

## Important Notes

- All source code must be in the `src/` directory
- Media files are stored in `src/statics/media/` and `src/storages/`
- HTML templates and assets are embedded in the binary using Go's embed feature
- FFmpeg is required for media processing (installation varies by platform)
