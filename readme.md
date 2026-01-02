<!-- markdownlint-disable MD041 -->
<!-- markdownlint-disable-next-line MD033 -->
<div align="center">
  <!-- markdownlint-disable-next-line MD033 -->
  <img src="src/views/assets/gowa.svg" alt="GoWA Logo" width="200" height="200">

## GoWA-SSE - WhatsApp Web API with SSE Support

*Built with Go for efficient memory use*

</div>

> **Note:** This is an independent project, originally forked from [aldinokemal/go-whatsapp-web-multidevice](https://github.com/aldinokemal/go-whatsapp-web-multidevice) at v7.11.0. It has since diverged significantly with a different architecture focused on SSE real-time events, S3/MinIO media storage, and chat history synchronization.
>
> For the original project, please visit the upstream repository.

## Support for `ARM` & `AMD` Architecture

Download:

- [Release](https://github.com/aldinokemal/go-whatsapp-web-multidevice/releases/latest)
- [Docker Hub](https://hub.docker.com/r/aldinokemal2104/go-whatsapp-web-multidevice/tags)
- [GitHub Container Registry](https://github.com/aldinokemal/go-whatsapp-web-multidevice/pkgs/container/go-whatsapp-web-multidevice)

## Support n8n package (n8n.io)

- [n8n package](https://www.npmjs.com/package/@aldinokemal2104/n8n-nodes-gowa)
- Go to Settings -> Community Nodes -> Input `@aldinokemal2104/n8n-nodes-gowa` -> Install

## Breaking Changes

- `v6`
  - For REST mode, you need to run `<binary> rest` instead of `<binary>`
    - for example: `./whatsapp rest` instead of ~~./whatsapp~~
- `v7`
  - Starting version 7.x we are using goreleaser to build the binary, so you can download the binary
      from [release](https://github.com/aldinokemal/go-whatsapp-web-multidevice/releases/latest)

## Feature

- Send WhatsApp message via http API, [docs/openapi.yml](./docs/openapi.yaml) for more details
- Mention someone
  - `@phoneNumber`
  - example: `Hello @628974812XXXX, @628974812XXXX`
- Post Whatsapp Status
- **Send Stickers** - Automatically converts images to WebP sticker format
  - Supports JPG, JPEG, PNG, WebP, and GIF formats
  - Automatic resizing to 512x512 pixels
  - Preserves transparency for PNG images
- Compress image before send
- Compress video before send
- Change OS name become your app (it's the device name when connect via mobile)
  - `--os=Chrome` or `--os=MyApplication`
- Basic Auth (able to add multi credentials)
  - `--basic-auth=kemal:secret,toni:password,userName:secretPassword`, or you can simplify
  - `-b=kemal:secret,toni:password,userName:secretPassword`
- Subpath deployment support
  - `--base-path="/gowa"` (allows deployment under a specific path like `/gowa/sub/path`)
- Customizable port and debug mode
  - `--port 8000`
  - `--debug true`
- Auto reply message
  - `--autoreply="Don't reply this message"`
- Auto mark read incoming messages
  - `--auto-mark-read=true` (automatically marks incoming messages as read)
- Auto download media from incoming messages
  - `--auto-download-media=false` (disable automatic media downloads, default: `true`)
- **Message Status Tracking** - Track delivery and read status of messages
  - Automatically updates message status: sent → delivered → read → played
  - Records timestamps for each status change
  - Enables analytics on message delivery and response times
  - See [Message Status Documentation](./MESSAGE_STATUS.md) for details
- Webhook for received message
  - `--webhook="http://yourwebhook.site/handler"`, or you can simplify
  - `-w="http://yourwebhook.site/handler"`
  - for more detail, see [Webhook Payload Documentation](./docs/webhook-payload.md)
- Webhook Secret
  Our webhook will be sent to you with an HMAC header and a sha256 default key `secret`.

  You may modify this by using the option below:
  - `--webhook-secret="secret"`
- **Webhook Payload Documentation**
  For detailed webhook payload schemas, security implementation, and integration examples,
  see [Webhook Payload Documentation](./docs/webhook-payload.md)
- **Webhook TLS Configuration**

  If you encounter TLS certificate verification errors when using webhooks (e.g., with Cloudflare tunnels or self-signed certificates):
  ```
  tls: failed to verify certificate: x509: certificate signed by unknown authority
  ```

  You can disable TLS certificate verification using:
  - `--webhook-insecure-skip-verify=true`
  - Or environment variable: `WHATSAPP_WEBHOOK_INSECURE_SKIP_VERIFY=true`

  **Security Warning**: This option disables TLS certificate verification and should only be used in:
  - Development/testing environments
  - Cloudflare tunnels (which provide their own security layer)
  - Internal networks with self-signed certificates

  **For production environments**, it's strongly recommended to use proper SSL certificates (e.g., Let's Encrypt) instead of disabling verification.

## Configuration

You can configure the application using either command-line flags (shown above) or environment variables. Configuration
can be set in three ways (in order of priority):

1. Command-line flags (highest priority)
2. Environment variables
3. `.env` file (lowest priority)

### Environment Variables

You can configure the application using environment variables. Configuration can be set in three ways (in order of
priority):

1. Command-line flags (highest priority)
2. Environment variables
3. `.env` file (lowest priority)

To use environment variables:

1. Copy `.env.example` to `.env` in your project root (`cp src/.env.example src/.env`)
2. Modify the values in `.env` according to your needs
3. Or set the same variables as system environment variables

#### Available Environment Variables

| Variable                      | Description                                 | Default   | Example                                     |
|-------------------------------|---------------------------------------------|-----------|---------------------------------------------|
| `APP_PORT`                    | Application port                            | `3000`    | `APP_PORT=8080`                             |
| `APP_DEBUG`                   | Enable debug logging                        | `false`   | `APP_DEBUG=true`                            |
| `APP_OS`                      | OS name (device name in WhatsApp)           | `Chrome`  | `APP_OS=MyApp`                              |
| `APP_BASE_PATH`               | Base path for subpath deployment            | -         | `APP_BASE_PATH=/gowa`                       |
| `APP_TRUSTED_PROXIES`         | Trusted proxy IP ranges for reverse proxy   | -         | `APP_TRUSTED_PROXIES=0.0.0.0/0`             |
| `DATABASE_URL`                | PostgreSQL connection URI (**required**)    | -         | `DATABASE_URL=postgres://user:pass@host:5432/db` |
| `AUTH_SECRET`                 | JWT secret for authentication               | -         | `AUTH_SECRET=your-secret-key`               |
| `AUTH_USERNAME`               | Admin username                              | `admin`   | `AUTH_USERNAME=admin`                       |
| `AUTH_PASSWORD_HASH`          | Bcrypt hash of admin password               | -         | (use hash-password script)                  |
| `WHATSAPP_CLIENTS`            | Initial seed phone numbers (see [Phone Requirements](#phone-number-requirements)) | - | `WHATSAPP_CLIENTS=6281234567890` |
| `MAX_CLIENTS`                 | Maximum clients per instance (for K8s scaling) | `10`    | `MAX_CLIENTS=10`                            |
| `WHATSAPP_AUTO_REPLY`         | Auto-reply message                          | -         | `WHATSAPP_AUTO_REPLY="Auto reply message"`  |
| `WHATSAPP_AUTO_MARK_READ`     | Auto-mark incoming messages as read         | `false`   | `WHATSAPP_AUTO_MARK_READ=true`              |
| `WHATSAPP_AUTO_DOWNLOAD_MEDIA`| Auto-download media from incoming messages  | `true`    | `WHATSAPP_AUTO_DOWNLOAD_MEDIA=false`        |
| `WHATSAPP_WEBHOOK`            | Webhook URL(s) for events (comma-separated) | -         | `WHATSAPP_WEBHOOK=https://webhook.site/xxx` |
| `WHATSAPP_WEBHOOK_SECRET`     | Webhook secret for validation               | `secret`  | `WHATSAPP_WEBHOOK_SECRET=super-secret-key`  |
| `WHATSAPP_ACCOUNT_VALIDATION` | Enable account validation                   | `true`    | `WHATSAPP_ACCOUNT_VALIDATION=false`         |

Note: Command-line flags will override any values set in environment variables or `.env` file.

- For more command `./whatsapp --help`

## Requirements

### System Requirements

- **PostgreSQL 14+** (required for data storage)
- **Go 1.24.0 or higher** (for building from source)
- **FFmpeg** (for media processing)

### Database Setup

This application requires PostgreSQL. Create a database and configure the connection:

```bash
# Create database
createdb whatsapp

# Set environment variable
export DATABASE_URL="postgres://user:password@localhost:5432/whatsapp?sslmode=disable"
```

The application will automatically create the required tables on first run.

### Platform Support

- Linux (x86_64, ARM64)
- macOS (Intel, Apple Silicon)
- Windows (x86_64) - WSL recommended

### Dependencies (without docker)

- Mac OS:
  - `brew install postgresql ffmpeg`
- Linux:
  - `sudo apt update`
  - `sudo apt install postgresql ffmpeg`
- Windows (not recommended, prefer using [WSL](https://docs.microsoft.com/en-us/windows/wsl/install)):
  - Install PostgreSQL: [download here](https://www.postgresql.org/download/windows/)
  - Install FFmpeg: [download here](https://www.ffmpeg.org/download.html#build-windows)
  - Add both to your PATH environment variable

## How to use

### Basic

1. Clone this repo: `git clone https://github.com/aldinokemal/go-whatsapp-web-multidevice`
2. Open the folder that was cloned via cmd/terminal.
3. run `cd src`
4. run `go run . rest` (for REST API mode)
5. Open `http://localhost:3000`

### Using Makefile (Recommended)

For easier building, running, and dependency management, use the included Makefile:

```bash
# Clone the repository
git clone https://github.com/aldinokemal/go-whatsapp-web-multidevice
cd go-whatsapp-web-multidevice

# Build and run in REST mode
make build
make run-rest

# Or simply
make run

# Run with hot reload (requires air)
make dev-rest

# Update dependencies
make update-deps

# Run tests
make test

# View all available commands
make help
```

**Documentation**:
- [Makefile Quick Reference](MAKEFILE_QUICKREF.md) - Common commands
- [Complete Makefile Guide](MAKEFILE_GUIDE.md) - Detailed documentation

### Docker (you don't need to install in required)

1. Clone this repo: `git clone https://github.com/aldinokemal/go-whatsapp-web-multidevice`
2. Open the folder that was cloned via cmd/terminal.
3. run `docker-compose up -d --build`
4. open `http://localhost:3000`

### Build your own binary

1. Clone this repo `git clone https://github.com/aldinokemal/go-whatsapp-web-multidevice`
2. Open the folder that was cloned via cmd/terminal.
3. run `cd src`
4. run
    1. Linux & MacOS: `go build -o whatsapp`
    2. Windows (CMD / PowerShell): `go build -o whatsapp.exe`
5. run
    1. Linux & MacOS: `./whatsapp rest` (for REST API mode)
        1. run `./whatsapp --help` for more detail flags
    2. Windows: `.\whatsapp.exe rest` (for REST API mode)
        1. run `.\whatsapp.exe --help` for more detail flags
6. open `http://localhost:3000` in browser

### Production Mode REST (docker)

Using Docker Hub:

```bash
docker run --detach --publish=3000:3000 --name=whatsapp --restart=always --volume=$(docker volume create --name=whatsapp):/app/storages aldinokemal2104/go-whatsapp-web-multidevice rest --autoreply="Dont't reply this message please"
```

Using GitHub Container Registry:

```bash
docker run --detach --publish=3000:3000 --name=whatsapp --restart=always --volume=$(docker volume create --name=whatsapp):/app/storages ghcr.io/aldinokemal/go-whatsapp-web-multidevice rest --autoreply="Dont't reply this message please"
```

### Production Mode REST (docker compose)

create `docker-compose.yml` file with the following configuration:

Using Docker Hub:

```yml
services:
  whatsapp:
    image: aldinokemal2104/go-whatsapp-web-multidevice
    container_name: whatsapp
    restart: always
    ports:
      - "3000:3000"
    volumes:
      - whatsapp:/app/storages
    command:
      - rest
      - --basic-auth=admin:admin
      - --port=3000
      - --debug=true
      - --os=Chrome
      - --account-validation=false

volumes:
  whatsapp:
```

Using GitHub Container Registry:

```yml
services:
  whatsapp:
    image: ghcr.io/aldinokemal/go-whatsapp-web-multidevice
    container_name: whatsapp
    restart: always
    ports:
      - "3000:3000"
    volumes:
      - whatsapp:/app/storages
    command:
      - rest
      - --basic-auth=admin:admin
      - --port=3000
      - --debug=true
      - --os=Chrome
      - --account-validation=false

volumes:
  whatsapp:
```

### Production with PostgreSQL (Recommended)

```yml
services:
  postgres:
    image: postgres:16-alpine
    container_name: whatsapp-db
    restart: always
    environment:
      POSTGRES_USER: whatsapp
      POSTGRES_PASSWORD: secretpassword
      POSTGRES_DB: whatsapp
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U whatsapp"]
      interval: 5s
      timeout: 5s
      retries: 5

  whatsapp:
    image: aldinokemal2104/go-whatsapp-web-multidevice
    container_name: whatsapp
    restart: always
    ports:
      - "3000:3000"
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      - DATABASE_URL=postgres://whatsapp:secretpassword@postgres:5432/whatsapp?sslmode=disable
      - AUTH_SECRET=your-jwt-secret-here
      - AUTH_USERNAME=admin
      - AUTH_PASSWORD_HASH=your-bcrypt-hash-here
      - WHATSAPP_CLIENTS=6281234567890
      - APP_PORT=3000
      - APP_DEBUG=false

volumes:
  postgres_data:
```

### Production Mode (binary)

- download binary from [release](https://github.com/aldinokemal/go-whatsapp-web-multidevice/releases)

You can fork or edit this source code !

## Current API

### HTTP REST API

- Check [docs/openapi.yml](./docs/openapi.yaml) for detailed API specifications.
- Use [SwaggerEditor](https://editor.swagger.io) to visualize the API.

### Multi-Client Architecture

All WhatsApp operations require a phone number in the URL path:

```
/api/{phone_number}/{endpoint}
```

Example: To send a message via client `628192191202`:
```bash
POST /api/628192191202/send/message
```

### Phone Number Requirements

Client IDs in `WHATSAPP_CLIENTS` must be **valid phone numbers** that match the WhatsApp account you'll scan with.

#### Validation Rules

| Rule | Requirement |
|------|-------------|
| Format | Digits only (after normalization) |
| Minimum | 7 digits |
| Maximum | 15 digits (E.164 standard) |
| Normalization | `+`, spaces, dashes are stripped automatically |

#### Valid Examples

```bash
# All these formats are accepted and normalized to digits:
WHATSAPP_CLIENTS=6281234567890              # Without +
WHATSAPP_CLIENTS=+6281234567890             # With +
WHATSAPP_CLIENTS=+62 812-3456-7890          # With formatting
WHATSAPP_CLIENTS=6281234567890,6289876543210  # Multiple clients
```

#### Invalid Examples (Will Fail on Startup)

```bash
WHATSAPP_CLIENTS=b1,b2                      # ❌ Not phone numbers
WHATSAPP_CLIENTS=client-1                   # ❌ Contains letters
WHATSAPP_CLIENTS=123456                     # ❌ Too short (< 7 digits)
```

#### Phone Mismatch Protection

When scanning the QR code, the system verifies that the scanned WhatsApp account matches the configured client ID:

```
Config: WHATSAPP_CLIENTS=6281234567890
User scans with: 6289999999999

Result: ❌ LOGIN_REJECTED
Message: "Phone mismatch: configured client 6281234567890 but scanned with 6289999999999"
```

This prevents session loss from misconfigured client IDs. If you need to use a different phone number, update `WHATSAPP_CLIENTS` first.

### Dynamic Client Management

Clients can be added and removed dynamically via the Admin API without restarting the server:

```bash
# Add a new client
curl -X POST http://localhost:3000/admin/clients \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"phone": "628123456789", "display_name": "Client 3"}'

# Response
{
  "code": 200,
  "message": "Client added successfully",
  "data": {
    "phone": "628123456789",
    "status": "awaiting_login",
    "message": "Client registered. Use /api/628123456789/app/login to get QR code"
  }
}

# Remove a client (chat history preserved)
curl -X DELETE http://localhost:3000/admin/clients/628123456789 \
  -H "Authorization: Bearer $TOKEN"

# Response
{
  "code": 200,
  "message": "Client removed successfully",
  "data": {
    "phone": "628123456789",
    "message": "Client disconnected. Chat history preserved."
  }
}
```

**Key Features:**
- **Persistence**: Dynamically added clients are stored in the database and survive restarts
- **MAX_CLIENTS**: Limit clients per instance (default: 10) for horizontal scaling with K8s
- **Chat History**: Removing a client only disconnects it; all chat history is preserved
- **Env as Seed**: `WHATSAPP_CLIENTS` serves as initial seed for first-time setup only

### Authentication Routes

| Method | URL              | Description                    |
|--------|------------------|--------------------------------|
| POST   | /auth/login      | Login with username/password   |
| POST   | /auth/refresh    | Refresh access token           |
| POST   | /auth/logout     | Invalidate session             |

### Admin Routes

| Method | URL                             | Description                    |
|--------|---------------------------------|--------------------------------|
| GET    | /admin/storage/stats            | Get database statistics        |
| POST   | /admin/storage/cleanup          | Clean old messages/chats       |
| POST   | /admin/storage/vacuum           | Optimize database              |
| DELETE | /admin/storage/chats            | Delete chats by pattern/JID    |
| GET    | /admin/clients                  | List all WhatsApp clients      |
| POST   | /admin/clients                  | Add new client dynamically     |
| DELETE | /admin/clients/:phone           | Remove client (keeps history)  |
| GET    | /admin/clients/:phone/status    | Get client connection status   |
| POST   | /admin/clients/:phone/connect   | Connect a client               |
| POST   | /admin/clients/:phone/disconnect| Disconnect a client            |

### WhatsApp API Routes (`/api/:phone/...`)

| Feature | Menu                           | Method | URL                                      |
|---------|--------------------------------|--------|------------------------------------------|
| ✅      | Login with Scan QR             | GET    | /api/:phone/app/login                    |
| ✅      | Login With Pair Code           | GET    | /api/:phone/app/login-with-code          |
| ✅      | Logout                         | GET    | /api/:phone/app/logout                   |
| ✅      | Reconnect                      | GET    | /api/:phone/app/reconnect                |
| ✅      | Devices                        | GET    | /api/:phone/app/devices                  |
| ✅      | Connection Status              | GET    | /api/:phone/app/status                   |
| ✅      | User Info                      | GET    | /api/:phone/user/info                    |
| ✅      | User Avatar                    | GET    | /api/:phone/user/avatar                  |
| ✅      | User Change Avatar             | POST   | /api/:phone/user/avatar                  |
| ✅      | User Change PushName           | POST   | /api/:phone/user/pushname                |
| ✅      | User My Groups                 | GET    | /api/:phone/user/my/groups               |
| ✅      | User My Newsletter             | GET    | /api/:phone/user/my/newsletters          |
| ✅      | User My Privacy Setting        | GET    | /api/:phone/user/my/privacy              |
| ✅      | User My Contacts               | GET    | /api/:phone/user/my/contacts             |
| ✅      | User Check                     | GET    | /api/:phone/user/check                   |
| ✅      | User Business Profile          | GET    | /api/:phone/user/business-profile        |
| ✅      | Send Message                   | POST   | /api/:phone/send/message                 |
| ✅      | Send Image                     | POST   | /api/:phone/send/image                   |
| ✅      | Send Audio                     | POST   | /api/:phone/send/audio                   |
| ✅      | Send File                      | POST   | /api/:phone/send/file                    |
| ✅      | Send Video                     | POST   | /api/:phone/send/video                   |
| ✅      | Send Sticker                   | POST   | /api/:phone/send/sticker                 |
| ✅      | Send Contact                   | POST   | /api/:phone/send/contact                 |
| ✅      | Send Link                      | POST   | /api/:phone/send/link                    |
| ✅      | Send Location                  | POST   | /api/:phone/send/location                |
| ✅      | Send Poll / Vote               | POST   | /api/:phone/send/poll                    |
| ✅      | Send Presence                  | POST   | /api/:phone/send/presence                |
| ✅      | Send Typing Indicator          | POST   | /api/:phone/send/chat-presence           |
| ✅      | Revoke Message                 | POST   | /api/:phone/message/:id/revoke           |
| ✅      | React Message                  | POST   | /api/:phone/message/:id/reaction         |
| ✅      | Delete Message                 | POST   | /api/:phone/message/:id/delete           |
| ✅      | Edit Message                   | POST   | /api/:phone/message/:id/update           |
| ✅      | Mark as Read                   | POST   | /api/:phone/message/:id/read             |
| ✅      | Star Message                   | POST   | /api/:phone/message/:id/star             |
| ✅      | Unstar Message                 | POST   | /api/:phone/message/:id/unstar           |
| ✅      | Download Media                 | GET    | /api/:phone/message/:id/download         |
| ✅      | Get Chat List                  | GET    | /api/:phone/chats                        |
| ✅      | Get Chat Messages              | GET    | /api/:phone/chat/:jid/messages           |
| ✅      | Pin Chat                       | POST   | /api/:phone/chat/:jid/pin                |
| ✅      | Set Disappearing Messages      | POST   | /api/:phone/chat/:jid/disappearing       |
| ✅      | Create Group                   | POST   | /api/:phone/group                        |
| ✅      | Join Group With Link           | POST   | /api/:phone/group/join-with-link         |
| ✅      | Group Info From Link           | GET    | /api/:phone/group/info-from-link         |
| ✅      | Group Info                     | GET    | /api/:phone/group/info                   |
| ✅      | Leave Group                    | POST   | /api/:phone/group/leave                  |
| ✅      | List Participants              | GET    | /api/:phone/group/participants           |
| ✅      | Export Participants (CSV)      | GET    | /api/:phone/group/participants/export    |
| ✅      | Add Participants               | POST   | /api/:phone/group/participants           |
| ✅      | Remove Participants            | POST   | /api/:phone/group/participants/remove    |
| ✅      | Promote Participants           | POST   | /api/:phone/group/participants/promote   |
| ✅      | Demote Participants            | POST   | /api/:phone/group/participants/demote    |
| ✅      | List Join Requests             | GET    | /api/:phone/group/participant-requests   |
| ✅      | Approve Join Request           | POST   | /api/:phone/group/participant-requests/approve |
| ✅      | Reject Join Request            | POST   | /api/:phone/group/participant-requests/reject  |
| ✅      | Set Group Photo                | POST   | /api/:phone/group/photo                  |
| ✅      | Set Group Name                 | POST   | /api/:phone/group/name                   |
| ✅      | Set Group Locked               | POST   | /api/:phone/group/locked                 |
| ✅      | Set Group Announce             | POST   | /api/:phone/group/announce               |
| ✅      | Set Group Topic                | POST   | /api/:phone/group/topic                  |
| ✅      | Get Group Invite Link          | GET    | /api/:phone/group/invite-link            |
| ✅      | Unfollow Newsletter            | POST   | /api/:phone/newsletter/unfollow          |
| ✅      | Trigger History Sync           | POST   | /api/:phone/history/sync                 |
| ✅      | Get History Sync Status        | GET    | /api/:phone/history/status               |

```txt
✅ = Available
❌ = Not Available Yet
```

## User Interface

### HTTP REST API UI

| Description          | Image                                                         |
|----------------------|---------------------------------------------------------------|
| Homepage             | ![Homepage](./gallery/homepage.png)                           |
| Login                | ![Login](./gallery/login.png)                                 |
| Login With Code      | ![Login With Code](./gallery/login-with-code.png)             |
| Send Message         | ![Send Message](./gallery/send-message.png)                   |
| Send Image           | ![Send Image](./gallery/send-image.png)                       |
| Send File            | ![Send File](./gallery/send-file.png)                         |
| Send Video           | ![Send Video](./gallery/send-video.png)                       |
| Send Sticker         | ![Send Sticker](./gallery/send-sticker.png)                   |
| Send Contact         | ![Send Contact](./gallery/send-contact.png)                   |
| Send Location        | ![Send Location](./gallery/send-location.png)                 |
| Send Audio           | ![Send Audio](./gallery/send-audio.png)                       |
| Send Poll            | ![Send Poll](./gallery/send-poll.png)                         |
| Send Presence        | ![Send Presence](./gallery/send-presence.png)                 |
| Send Link            | ![Send Link](./gallery/send-link.png)                         |
| My Group             | ![My Group](./gallery/group-list.png)                         |
| Group Info From Link | ![Group Info From Link](./gallery/group-info-from-link.png)   |
| Create Group         | ![Create Group](./gallery/group-create.png)                   |
| Join Group with Link | ![Join Group with Link](./gallery/group-join-link.png)        |
| Manage Participant   | ![Manage Participant](./gallery/group-manage-participant.png) |
| My Newsletter        | ![My Newsletter](./gallery/newsletter-list.png)               |
| My Contacts          | ![My Contacts](./gallery/contact-list.png)                    |
| Business Profile     | ![Business Profile](./gallery/business-profile.png)           |

## Important

- This project is unofficial and not affiliated with WhatsApp.
- Please use official WhatsApp API to avoid any issues.
