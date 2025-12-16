# Historical Data Storage Feature

This document describes the historical data storage feature that automatically saves WhatsApp chat history to the database.

## Overview

The historical data storage feature allows you to:
- Automatically store historical WhatsApp messages to the database when they are synced
- Configure how far back in time to store messages (default: 3 months)
- Control whether history sync is enabled or disabled
- Query the status of history sync configuration via REST API or MCP

## Configuration

### Environment Variables

Add these to your `.env` file:

```env
# History Sync Configuration
WHATSAPP_HISTORY_SYNC_ENABLED=true
WHATSAPP_HISTORY_SYNC_ON_LOGIN=true
WHATSAPP_HISTORY_SYNC_MAX_DAYS=90
```

### Configuration Options

| Variable | Default | Description |
|----------|---------|-------------|
| `WHATSAPP_HISTORY_SYNC_ENABLED` | `true` | Enable or disable history sync processing |
| `WHATSAPP_HISTORY_SYNC_ON_LOGIN` | `true` | Automatically process history sync on login |
| `WHATSAPP_HISTORY_SYNC_MAX_DAYS` | `90` | Maximum days of history to sync |

### Predefined Time Ranges

You can use these common values for `WHATSAPP_HISTORY_SYNC_MAX_DAYS`:

- `90` - 3 months (default)
- `365` - 1 year
- `730` - 2 years
- `1095` - 3 years
- `-1` - All available history (no limit)

## How It Works

### Automatic Sync on Login

When you connect to WhatsApp, the server automatically sends historical messages based on your account settings. This implementation:

1. **Receives History Sync Events**: WhatsApp sends history sync data in batches when you connect
2. **Filters by Date**: Only messages within the configured `WHATSAPP_HISTORY_SYNC_MAX_DAYS` are processed
3. **Stores to Database**: Messages are stored in the chat storage database (`storages/chatstorage.db`)
4. **Saves JSON Backups**: Raw history sync data is also saved as JSON files in `storages/history-*.json`

### Message Filtering

The system filters messages based on the configured maximum days:

```go
// Messages older than the cutoff time are skipped
cutoffTime = Now - WHATSAPP_HISTORY_SYNC_MAX_DAYS days
```

## REST API Endpoints

### Get History Sync Status

**Endpoint**: `GET /history/status`

**Response**:
```json
{
  "enabled": true,
  "on_login": true,
  "max_days": 90,
  "max_days_label": "3 months"
}
```

### Request History Sync Info

**Endpoint**: `POST /history/sync`

**Request Body**:
```json
{
  "chat_jid": "1234567890@s.whatsapp.net",
  "max_days": 90,
  "message_count": 50
}
```

**Response**:
```json
{
  "status": "info",
  "message": "History sync is configured to process messages from the last 90 days...",
  "requested_at": "2025-12-16T13:45:00Z",
  "max_days": 90,
  "sync_type": "automatic",
  "estimated_cutoff": "2025-09-17T13:45:00Z"
}
```

## MCP (Model Context Protocol) Support

If you're using MCP mode, you can interact with history sync using these tools:

### whatsapp_history_status

Get the current history sync configuration and status.

**Parameters**:
- `action`: "get_status" (required)

**Example**:
```json
{
  "action": "get_status"
}
```

### whatsapp_history_sync

Request information about history sync configuration.

**Parameters**:
- `action`: "request_sync" (required)
- `chat_jid`: Optional chat JID to sync for
- `max_days`: Optional override for max days

**Example**:
```json
{
  "action": "request_sync",
  "max_days": 365
}
```

## Database Storage

Historical messages are stored in the chat storage database with the following information:

- Message ID
- Chat JID
- Sender
- Content (text)
- Timestamp
- Media information (if applicable)
- Read/delivery status

## Limitations

1. **WhatsApp Controls History**: The amount of history available is controlled by WhatsApp's servers, not this application
2. **Automatic Sync Only**: History is synced automatically when WhatsApp sends it (typically on first connect or reconnect)
3. **On-Demand Sync**: On-demand history sync for specific chats is documented but not fully implemented (requires reference message ID)

## Performance Considerations

- **Large History**: Syncing large amounts of history (e.g., 3 years) may take time on first connect
- **Database Size**: Consider the database size when setting very large max_days values
- **Filtering**: Messages outside the configured date range are filtered before database insertion for efficiency

## Troubleshooting

### History Not Being Stored

1. Check that `WHATSAPP_HISTORY_SYNC_ENABLED=true`
2. Check logs for "History sync filtering" messages
3. Verify the chat storage database is writable

### Only Recent Messages Stored

1. Check your `WHATSAPP_HISTORY_SYNC_MAX_DAYS` setting
2. WhatsApp may not send older history depending on account settings
3. Check the history sync JSON files in `storages/` to see what was received

### History Sync Files

JSON files are created in the `storages/` directory with naming pattern:
```
history-{timestamp}-{device_id}-{sync_id}-{sync_type}.json
```

These can be used for debugging or backup purposes.

## Implementation Details

### Key Files

- `src/config/settings.go` - Configuration variables
- `src/infrastructure/whatsapp/init.go` - History sync event handler and processing
- `src/domains/history/` - History domain models and interfaces
- `src/usecase/history.go` - History sync business logic
- `src/ui/rest/history.go` - REST API endpoints
- `src/ui/mcp/history.go` - MCP tools

### Sync Types Processed

The system processes these WhatsApp history sync types:

- `INITIAL_BOOTSTRAP` - Initial history sync on first connect
- `RECENT` - Recent messages sync
- `PUSH_NAME` - Contact name updates

## Future Enhancements

Potential future improvements:

1. On-demand history sync for specific chats with message ID reference
2. Progress tracking for large history syncs
3. Configurable sync types (e.g., only recent, not full bootstrap)
4. History sync scheduling/retry logic
5. Compression for history JSON backups

## References

- [WhatsApp Multi-Device Protocol](https://github.com/tulir/whatsmeow)
- [mautrix-whatsapp Bridge Configuration](https://github.com/element-hq/mautrix-whatsapp/blob/element-main/example-config.yaml)
