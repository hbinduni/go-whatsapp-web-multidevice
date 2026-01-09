# Chat Storage Management API

This guide explains how to use the API endpoints for managing the chat storage database, including backup/restore, cleanup, and optimization.

## Overview

The chat storage database (`chatstorage.db`) stores your chat history, messages, and related metadata. These APIs allow you to:

- **Export** chat history for backup
- **Import** a backup to restore chats (merge mode)
- **Analyze** backup files before importing
- **Cleanup** old messages or specific chats
- **Vacuum** the database to optimize performance
- **View statistics** about stored data

## Chat Storage Statistics

### Get Storage Stats

Get detailed statistics about the chat storage database.

```bash
GET /admin/storage/stats
```

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Storage statistics retrieved successfully",
  "results": {
    "total_chats": 156,
    "total_messages": 12847,
    "database_size": "45.2 MB",
    "database_size_bytes": 47407923,
    "oldest_message": "2023-06-15T10:30:00Z",
    "newest_message": "2024-01-15T14:22:35Z",
    "empty_chats": 12,
    "media_messages": 3421,
    "text_messages": 9426
  }
}
```

## Backup & Restore API

### Export Chat Storage

Download the chat storage database as a backup file.

```bash
GET /chat/export
```

**Response:** Binary file download (`.db` file)

**Headers:**
- `Content-Disposition: attachment; filename="chatstorage-20240115-143022.db"`
- `Content-Type: application/octet-stream`

**Example with curl:**
```bash
curl -o chat-backup-$(date +%Y%m%d-%H%M%S).db http://localhost:3000/chat/export
```

### Analyze Backup File

Analyze a backup file before importing to see its contents and structure.

```bash
POST /chat/analyze
Content-Type: multipart/form-data
```

**Form Fields:**
- `file` (required): The backup database file (`.db`, `.sqlite`, `.sqlite3`)

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Analysis completed successfully",
  "results": {
    "filename": "chat-backup.db",
    "size": "25.3 MB",
    "tables": [
      {"name": "chats", "row_count": 89},
      {"name": "messages", "row_count": 5432},
      {"name": "message_reactions", "row_count": 234}
    ],
    "schema": [
      {"table": "chats", "description": "Chat metadata and settings"},
      {"table": "messages", "description": "Message content and metadata"},
      {"table": "message_reactions", "description": "Emoji reactions to messages"}
    ]
  }
}
```

**Example with curl:**
```bash
curl -X POST -F "file=@chat-backup.db" http://localhost:3000/chat/analyze
```

### Import Chat Storage

Import chats from a backup file. Uses **merge mode** - existing data is preserved, only new data is added.

```bash
POST /chat/import
Content-Type: multipart/form-data
```

**Form Fields:**
- `file` (required): The backup database file (`.db`, `.sqlite`, `.sqlite3`)

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Chat storage imported successfully",
  "results": {
    "imported": {
      "chats": 45,
      "messages": 2341
    },
    "skipped": {
      "chats": 44,
      "messages": 3091
    },
    "duration": "2.34s"
  }
}
```

**Example with curl:**
```bash
curl -X POST -F "file=@chat-backup.db" http://localhost:3000/chat/import
```

**Note:** Import uses merge mode:
- Existing chats/messages are not overwritten
- Only new (non-duplicate) data is added
- Safe to run multiple times

## Cleanup API

### Cleanup Storage

Perform cleanup operations based on specified criteria.

```bash
POST /admin/storage/cleanup
Content-Type: application/json
```

**Request Body:**
```json
{
  "pattern": "%@broadcast",
  "older_than": "2023-01-01T00:00:00Z",
  "empty_chats": true,
  "dry_run": true
}
```

**Parameters:**
| Field | Type | Description |
|-------|------|-------------|
| `pattern` | string | SQL LIKE pattern to match chat JIDs (e.g., `%@broadcast`) |
| `older_than` | string | RFC3339 date - delete messages older than this |
| `empty_chats` | boolean | Delete chats with no messages |
| `dry_run` | boolean | Preview only, don't actually delete |

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Cleanup dry run completed (no changes made)",
  "results": {
    "chats_deleted": 15,
    "messages_deleted": 0,
    "dry_run": true
  }
}
```

**Examples:**

```bash
# Preview cleanup of broadcast chats
curl -X POST http://localhost:3000/admin/storage/cleanup \
  -H "Content-Type: application/json" \
  -d '{"pattern": "%@broadcast", "dry_run": true}'

# Delete messages older than 6 months
curl -X POST http://localhost:3000/admin/storage/cleanup \
  -H "Content-Type: application/json" \
  -d '{"older_than": "2023-07-01T00:00:00Z", "dry_run": false}'

# Delete empty chats
curl -X POST http://localhost:3000/admin/storage/cleanup \
  -H "Content-Type: application/json" \
  -d '{"empty_chats": true, "dry_run": false}'
```

### Delete Specific Chats

Delete chats by specific JIDs or pattern.

```bash
DELETE /admin/storage/chats
Content-Type: application/json
```

**Request Body:**
```json
{
  "jids": ["1234567890@s.whatsapp.net", "0987654321@s.whatsapp.net"],
  "pattern": "",
  "dry_run": true
}
```

**Parameters:**
| Field | Type | Description |
|-------|------|-------------|
| `jids` | array | List of specific JIDs to delete |
| `pattern` | string | SQL LIKE pattern to match JIDs |
| `dry_run` | boolean | Preview only, don't actually delete |

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Delete dry run completed (no changes made)",
  "results": {
    "chats_deleted": 2,
    "messages_deleted": 847,
    "deleted_jids": [
      "1234567890@s.whatsapp.net",
      "0987654321@s.whatsapp.net"
    ],
    "dry_run": true
  }
}
```

**Examples:**

```bash
# Preview deletion of specific chats
curl -X DELETE http://localhost:3000/admin/storage/chats \
  -H "Content-Type: application/json" \
  -d '{"jids": ["1234567890@s.whatsapp.net"], "dry_run": true}'

# Delete all status broadcasts
curl -X DELETE http://localhost:3000/admin/storage/chats \
  -H "Content-Type: application/json" \
  -d '{"pattern": "status@broadcast", "dry_run": false}'

# Delete all group chats
curl -X DELETE http://localhost:3000/admin/storage/chats \
  -H "Content-Type: application/json" \
  -d '{"pattern": "%@g.us", "dry_run": false}'
```

## Database Optimization

### Vacuum Database

Run SQLite VACUUM to optimize the database and reclaim unused space.

```bash
POST /admin/storage/vacuum
```

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Database vacuum completed successfully",
  "results": {
    "size_before": "125.4 MB",
    "size_after": "45.2 MB",
    "size_before_bytes": 131502489,
    "size_after_bytes": 47407923,
    "reclaimed": "80.2 MB",
    "reclaimed_bytes": 84094566
  }
}
```

**Example with curl:**
```bash
curl -X POST http://localhost:3000/admin/storage/vacuum
```

**Tip:** Run vacuum after cleanup operations to reclaim the freed space.

## Chat Query API

### List All Chats

Get a list of all stored chats.

```bash
GET /chats
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | integer | Maximum number of chats to return |
| `offset` | integer | Number of chats to skip |

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Chats retrieved successfully",
  "results": [
    {
      "jid": "1234567890@s.whatsapp.net",
      "name": "John Doe",
      "last_message_time": "2024-01-15T14:22:35Z",
      "unread_count": 0
    }
  ]
}
```

### Get Chat Messages

Get messages from a specific chat.

```bash
GET /chat/:chat_jid/messages
```

**Path Parameters:**
- `chat_jid`: The JID of the chat (e.g., `1234567890@s.whatsapp.net`)

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | integer | Maximum messages to return (default: 50) |
| `before` | string | Get messages before this timestamp |
| `after` | string | Get messages after this timestamp |

**Example:**
```bash
curl "http://localhost:3000/chat/1234567890@s.whatsapp.net/messages?limit=100"
```

## Complete Workflows

### Regular Backup Workflow

```bash
# 1. Check current statistics
curl http://localhost:3000/admin/storage/stats

# 2. Export backup
curl -o chat-backup-$(date +%Y%m%d).db http://localhost:3000/chat/export

# 3. Verify backup by analyzing it
curl -X POST -F "file=@chat-backup-$(date +%Y%m%d).db" http://localhost:3000/chat/analyze
```

### Cleanup and Optimize Workflow

```bash
# 1. Check current size
curl http://localhost:3000/admin/storage/stats

# 2. Preview cleanup (dry run)
curl -X POST http://localhost:3000/admin/storage/cleanup \
  -H "Content-Type: application/json" \
  -d '{"older_than": "2023-01-01T00:00:00Z", "empty_chats": true, "dry_run": true}'

# 3. Perform actual cleanup
curl -X POST http://localhost:3000/admin/storage/cleanup \
  -H "Content-Type: application/json" \
  -d '{"older_than": "2023-01-01T00:00:00Z", "empty_chats": true, "dry_run": false}'

# 4. Vacuum to reclaim space
curl -X POST http://localhost:3000/admin/storage/vacuum

# 5. Verify new size
curl http://localhost:3000/admin/storage/stats
```

### Merge Chats from Another Backup

```bash
# 1. Analyze the backup first
curl -X POST -F "file=@other-device-backup.db" http://localhost:3000/chat/analyze

# 2. Import (merge mode - won't overwrite existing)
curl -X POST -F "file=@other-device-backup.db" http://localhost:3000/chat/import

# 3. Check updated statistics
curl http://localhost:3000/admin/storage/stats
```

## Pattern Matching Reference

The `pattern` parameter uses SQL LIKE syntax:

| Pattern | Matches |
|---------|---------|
| `%@s.whatsapp.net` | All individual chats |
| `%@g.us` | All group chats |
| `%@broadcast` | All broadcast lists |
| `status@broadcast` | Status updates |
| `1234%` | JIDs starting with 1234 |
| `%5678@s.whatsapp.net` | JIDs ending with 5678 |

## Error Handling

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| "pattern cannot be empty" | Empty pattern in cleanup | Provide a valid pattern |
| "invalid older_than format" | Wrong date format | Use RFC3339 format |
| "Invalid file type" | Wrong file extension | Use `.db`, `.sqlite`, or `.sqlite3` |
| "Failed to open database" | Corrupt backup file | Try a different backup |

### Error Response Format

```json
{
  "status": 400,
  "code": "BAD_REQUEST",
  "message": "Invalid request body: invalid older_than format, expected RFC3339",
  "results": null
}
```

## Best Practices

1. **Always use dry_run first**: Preview cleanup operations before executing them.

2. **Regular backups**: Schedule daily or weekly exports of your chat storage.

3. **Vacuum after cleanup**: Run vacuum after deleting data to reclaim disk space.

4. **Analyze before import**: Always analyze backup files before importing to verify contents.

5. **Keep multiple backups**: Maintain rolling backups (daily, weekly, monthly).

6. **Monitor database size**: Check statistics periodically to manage storage growth.

## UI Access

These features are also available through the web UI:

1. Open the application homepage
2. Find the **Chat Storage Backup** card (teal color) for export/import/analyze
3. Find the **Storage Management** card (purple color) for stats/cleanup/vacuum
4. Both are in the main cards area on the homepage
