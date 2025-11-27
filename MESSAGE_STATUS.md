# Message Status Tracking

This document describes the message status tracking feature that monitors the delivery and read status of WhatsApp messages.

## Overview

The message status tracking system automatically updates the status of messages as they progress through their lifecycle:
- **sent** - Message has been sent from your device
- **delivered** - Message has been delivered to recipient's device
- **read** - Recipient has opened the chat and seen the message
- **played** - View-once media (photo/video) has been opened by recipient

Each status transition is timestamped, enabling analytics and better user experience features.

## Database Schema

### New Columns in `messages` Table

```sql
-- Status tracking columns (added in Migration 3)
status TEXT DEFAULT 'sent'           -- Current message status
delivered_at TIMESTAMP                -- When message was delivered
read_at TIMESTAMP                     -- When message was read
played_at TIMESTAMP                   -- When view-once media was played
```

### Index for Performance

```sql
CREATE INDEX idx_messages_status ON messages(status);
```

## Status Lifecycle

### Normal Message Flow

```
1. Message Sent
   ├─> Status: "sent"
   └─> stored in database

2. WhatsApp Receipt: Delivered
   ├─> Status: "delivered"
   ├─> delivered_at: [timestamp]
   └─> only updates if status was "sent"

3. WhatsApp Receipt: Read
   ├─> Status: "read"
   ├─> read_at: [timestamp]
   └─> only updates if status was "sent" or "delivered"
```

### View-Once Media Flow

```
1. View-Once Message Sent
   ├─> Status: "sent"
   └─> stored in database

2. Recipient Opens Media
   ├─> Status: "played"
   ├─> played_at: [timestamp]
   └─> updates regardless of previous status
```

## Implementation Details

### Domain Model

**File:** `src/domains/chatstorage/chatstorage.go`

```go
type Message struct {
    // ... existing fields ...
    Status        string     `db:"status"`         // sent, delivered, read, played
    DeliveredAt   *time.Time `db:"delivered_at"`   // When delivered
    ReadAt        *time.Time `db:"read_at"`        // When read
    PlayedAt      *time.Time `db:"played_at"`      // When played
}
```

### Repository Interface

**File:** `src/domains/chatstorage/interfaces.go`

```go
UpdateMessageStatus(ctx context.Context, messageID string, status string, statusTime time.Time) error
```

### Status Update Logic

**File:** `src/infrastructure/chatstorage/sqlite_repository.go`

The `UpdateMessageStatus()` method ensures status only progresses forward:

```go
// Delivered: only if currently "sent"
WHERE id = ? AND (status = 'sent' OR delivered_at IS NULL)

// Read: only if currently "sent" or "delivered"
WHERE id = ? AND (status IN ('sent', 'delivered') OR read_at IS NULL)

// Played: updates regardless of current status
WHERE id = ?
```

### Event Handler

**File:** `src/infrastructure/whatsapp/init.go`

The `handleReceipt()` function processes WhatsApp receipt events:

```go
func handleReceipt(ctx context.Context, evt *events.Receipt, chatStorageRepo ...) {
    switch evt.Type {
    case types.ReceiptTypeDelivered:
        // Update status to "delivered"
    case types.ReceiptTypeRead, types.ReceiptTypeReadSelf:
        // Update status to "read"
    case types.ReceiptTypePlayed:
        // Update status to "played"
    }
}
```

## Usage Examples

### Query Messages by Status

**Find all unread messages:**
```sql
SELECT * FROM messages
WHERE status IN ('sent', 'delivered');
```

**Find messages read within last hour:**
```sql
SELECT * FROM messages
WHERE read_at > datetime('now', '-1 hour');
```

**Find view-once messages that were opened:**
```sql
SELECT * FROM messages
WHERE played_at IS NOT NULL;
```

### Calculate Response Time

**Average time to read messages:**
```sql
SELECT
    AVG((julianday(read_at) - julianday(timestamp)) * 24 * 60) as avg_minutes
FROM messages
WHERE read_at IS NOT NULL;
```

**Messages read within 5 minutes:**
```sql
SELECT * FROM messages
WHERE read_at IS NOT NULL
  AND (julianday(read_at) - julianday(timestamp)) * 24 * 60 < 5;
```

### Filter by Status in Code

**Using the repository:**
```go
// Get message with status
message, err := chatStorageRepo.GetMessageByID("message-id")
if err != nil {
    return err
}

// Check status
switch message.Status {
case "sent":
    fmt.Println("Message sent but not delivered")
case "delivered":
    fmt.Println("Message delivered but not read")
case "read":
    fmt.Printf("Message read at: %v\n", message.ReadAt)
case "played":
    fmt.Printf("View-once media played at: %v\n", message.PlayedAt)
}
```

## Migration

### Automatic Schema Update

The schema is automatically updated when the application starts. The migration:

1. Adds four new columns to the `messages` table
2. Sets default value `'sent'` for existing messages
3. Creates an index for status queries
4. Is idempotent and safe to run multiple times

**Migration Location:** `src/infrastructure/chatstorage/sqlite_repository.go` (Migration 3)

### Migration Details

```sql
-- Add status column (sent, delivered, read, played)
ALTER TABLE messages ADD COLUMN status TEXT DEFAULT 'sent';

-- Add timestamp columns for status tracking
ALTER TABLE messages ADD COLUMN delivered_at TIMESTAMP;
ALTER TABLE messages ADD COLUMN read_at TIMESTAMP;
ALTER TABLE messages ADD COLUMN played_at TIMESTAMP;

-- Create index for status queries
CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
```

## Monitoring and Logging

### Log Messages

**Message status updates:**
```
DEBUG: Updated message ABC123 status to delivered
DEBUG: Updated message ABC123 status to read
```

**Receipt events:**
```
INFO: ["MSG_ID"] was delivered to 1234567890@s.whatsapp.net at 2025-11-28T10:30:00Z
INFO: ["MSG_ID"] was read by 1234567890@s.whatsapp.net at 2025-11-28T10:32:15Z
```

**Errors:**
```
ERROR: Failed to update status for message ABC123 to read: [error details]
```

### Debug Mode

To see detailed status update logs, set log level to DEBUG:
```bash
# In .env file
LOG_LEVEL=debug
```

## API Integration

### REST API Response Format

Messages returned by the API now include status fields:

```json
{
  "id": "MESSAGE_ID",
  "chat_jid": "1234567890@s.whatsapp.net",
  "content": "Hello!",
  "timestamp": "2025-11-28T10:30:00Z",
  "status": "read",
  "delivered_at": "2025-11-28T10:30:05Z",
  "read_at": "2025-11-28T10:32:15Z",
  "played_at": null
}
```

### Webhook Payload

Receipt events are still forwarded to webhooks (if configured) with the original WhatsApp event data.

## Performance Considerations

### Indexes

The `idx_messages_status` index optimizes queries that filter by status:

```sql
-- Fast query thanks to index
SELECT * FROM messages WHERE status = 'delivered';
```

### Batch Updates

Receipt events can contain multiple message IDs. The system efficiently processes them:

```go
for _, messageID := range evt.MessageIDs {
    chatStorageRepo.UpdateMessageStatus(ctx, messageID, status, evt.Timestamp)
}
```

### Context Support

All database operations support context cancellation for graceful shutdown:

```go
UpdateMessageStatus(ctx context.Context, messageID string, ...)
```

## Troubleshooting

### Status Not Updating

**Problem:** Messages stay in "sent" status
**Possible Causes:**
- WhatsApp hasn't sent a receipt event yet
- Recipient hasn't received/read the message
- Network issues preventing receipt events

**Solution:** Wait for the recipient to receive/read the message. Check logs for receipt events.

### Migration Errors

**Problem:** Schema migration fails on startup
**Solution:**
1. Check database permissions (write access required)
2. Backup database and try again
3. Check logs for specific error details

### Query Performance

**Problem:** Slow queries when filtering by status
**Solution:**
- Ensure `idx_messages_status` index exists
- Use `EXPLAIN QUERY PLAN` to verify index usage
- Consider adding composite indexes if needed

## Future Enhancements

Potential improvements to the status tracking system:

1. **Status History Table** - Track all status changes with full audit trail
2. **Delivery Reports** - Generate reports on message delivery rates
3. **Real-time UI Updates** - WebSocket notifications when status changes
4. **Retry Logic** - Automatic retry for failed status updates
5. **Analytics Dashboard** - Visual representation of message statistics

## Technical Notes

### Why Nullable Timestamps?

The timestamp columns (`delivered_at`, `read_at`, `played_at`) are nullable (`*time.Time` in Go) because:
- Not all messages will reach all status states
- NULL clearly indicates "not yet happened" vs. a zero timestamp
- Enables queries like `WHERE delivered_at IS NULL` to find undelivered messages

### Status Progression Rules

The WHERE clauses in update queries prevent backwards progression:
- Can't go from "read" back to "delivered"
- Can't go from "delivered" back to "sent"
- Ensures data integrity and logical consistency

### Thread Safety

All repository operations use database transactions and are thread-safe. Multiple goroutines can update message statuses concurrently without conflicts.

## Related Documentation

- [Database Schema](./readme.md#database) - Main database documentation
- [Event Handlers](./src/infrastructure/whatsapp/init.go) - WhatsApp event handling
- [Chat Storage](./src/infrastructure/chatstorage/) - Message storage implementation
