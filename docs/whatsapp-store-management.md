# WhatsApp Store Management API

This guide explains how to use the API endpoints for managing the WhatsApp client connection and backing up/restoring the WhatsApp store database.

## Overview

The WhatsApp store database (`whatsapp.db`) contains your device credentials, encryption keys, and session data. These APIs allow you to:

- **Stop/Start** the WhatsApp client without logging out (session remains valid)
- **Export** the store for backup
- **Import** a backup to restore credentials
- **Vacuum** the database to optimize performance

## Client Control API

### Get Client Status

Check the current connection status of the WhatsApp client.

```bash
GET /app/status
```

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Client is connected and logged in",
  "results": {
    "is_connected": true,
    "is_logged_in": true,
    "is_stopped": false,
    "device_jid": "1234567890:12@s.whatsapp.net",
    "status_message": "Client is connected and logged in"
  }
}
```

### Stop Client

Disconnect the WhatsApp client without logging out. The session remains valid and can be reconnected later.

```bash
POST /app/stop
```

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Client stopped successfully. Session remains valid.",
  "results": {
    "is_connected": false,
    "is_logged_in": true,
    "is_stopped": true,
    "device_jid": "1234567890:12@s.whatsapp.net",
    "status_message": "Client stopped successfully. Session remains valid."
  }
}
```

**Note:** When the client is stopped:
- Auto-reconnect is disabled
- No messages will be sent or received
- The session remains valid (no need to re-scan QR code)

### Start Client

Reconnect the WhatsApp client after being stopped.

```bash
POST /app/start
```

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "Client started successfully.",
  "results": {
    "is_connected": true,
    "is_logged_in": true,
    "is_stopped": false,
    "device_jid": "1234567890:12@s.whatsapp.net",
    "status_message": "Client started successfully."
  }
}
```

## WhatsApp Store API

### Get Store Statistics

Get information about the WhatsApp store database.

```bash
GET /admin/whatsapp-store/stats
```

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "WhatsApp store statistics retrieved successfully",
  "results": {
    "database_size": "2.5 MB",
    "database_size_bytes": 2621440,
    "device_count": 1,
    "has_keys_db": false,
    "is_connected": true,
    "is_logged_in": true,
    "device_jid": "1234567890:12@s.whatsapp.net"
  }
}
```

### Export WhatsApp Store

Download the WhatsApp store database as a backup file.

```bash
GET /admin/whatsapp-store/export
```

**Response:** Binary file download (`.db` file)

**Headers:**
- `Content-Disposition: attachment; filename="whatsapp-store-20240115-143022.db"`
- `Content-Type: application/octet-stream`
- `X-Export-Warning: Client is connected. For consistent backup, consider stopping the client first.` (if connected)

**Example with curl:**
```bash
curl -o whatsapp-backup.db http://localhost:3000/admin/whatsapp-store/export
```

### Import WhatsApp Store

Restore the WhatsApp store from a backup file.

```bash
POST /admin/whatsapp-store/import
Content-Type: multipart/form-data
```

**Form Fields:**
- `file` (required): The backup database file (`.db`, `.sqlite`, `.sqlite3`)
- `keys_file` (optional): Separate keys database file if applicable

**Prerequisites:**
- Client must be stopped first (`POST /app/stop`)

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "WhatsApp store imported successfully",
  "results": {
    "status": "success",
    "message": "WhatsApp store imported successfully",
    "main_db_imported": true,
    "keys_db_imported": false,
    "requires_restart": false
  }
}
```

**Example with curl:**
```bash
# First stop the client
curl -X POST http://localhost:3000/app/stop

# Import the backup
curl -X POST -F "file=@whatsapp-backup.db" http://localhost:3000/admin/whatsapp-store/import

# Start the client again
curl -X POST http://localhost:3000/app/start
```

### Vacuum WhatsApp Store

Optimize the database and reclaim unused space.

```bash
POST /admin/whatsapp-store/vacuum
```

**Response:**
```json
{
  "status": 200,
  "code": "SUCCESS",
  "message": "WhatsApp store vacuum completed successfully",
  "results": {
    "main_db": {
      "size_before": "5.2 MB",
      "size_after": "2.1 MB",
      "size_before_bytes": 5452595,
      "size_after_bytes": 2202009,
      "reclaimed": "3.1 MB",
      "reclaimed_bytes": 3250586
    },
    "keys_db": null,
    "warning": ""
  }
}
```

## Complete Backup & Restore Workflow

### Backup Procedure

```bash
# 1. Check current status
curl http://localhost:3000/app/status

# 2. (Optional) Stop client for consistent backup
curl -X POST http://localhost:3000/app/stop

# 3. Export the database
curl -o backup-$(date +%Y%m%d-%H%M%S).db http://localhost:3000/admin/whatsapp-store/export

# 4. (Optional) Start client again
curl -X POST http://localhost:3000/app/start
```

### Restore Procedure

```bash
# 1. Stop the client (required)
curl -X POST http://localhost:3000/app/stop

# 2. Import the backup
curl -X POST -F "file=@backup-20240115-143022.db" http://localhost:3000/admin/whatsapp-store/import

# 3. Start the client
curl -X POST http://localhost:3000/app/start

# 4. Verify connection
curl http://localhost:3000/app/status
```

## Migration Between Servers

To migrate your WhatsApp session to a new server:

### On the Source Server:

```bash
# Stop client and export
curl -X POST http://source-server:3000/app/stop
curl -o whatsapp-migration.db http://source-server:3000/admin/whatsapp-store/export
```

### On the Target Server:

```bash
# Stop any existing client
curl -X POST http://target-server:3000/app/stop

# Import the backup
curl -X POST -F "file=@whatsapp-migration.db" http://target-server:3000/admin/whatsapp-store/import

# Start the client
curl -X POST http://target-server:3000/app/start
```

**Important:** Only one server should be running with the same WhatsApp session at a time. Running on multiple servers simultaneously may cause session conflicts.

## Error Handling

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| "client is already stopped" | Calling stop when already stopped | Check status first |
| "client not initialized" | Client not properly initialized | Restart the application |
| "WhatsApp client is still connected" | Import while connected | Stop the client first |
| "Invalid file type" | Wrong file extension | Use `.db`, `.sqlite`, or `.sqlite3` |

### Error Response Format

```json
{
  "status": 400,
  "code": "BAD_REQUEST",
  "message": "WhatsApp client is still connected. Please disconnect first.",
  "results": null
}
```

## Security Considerations

1. **Protect backup files**: The database contains encryption keys and session credentials. Store backups securely.

2. **Use HTTPS**: Always use HTTPS in production to protect API calls.

3. **Restrict access**: Use authentication/firewall rules to limit access to admin endpoints.

4. **Regular backups**: Schedule regular backups to prevent data loss.

5. **Test restores**: Periodically test the restore process to ensure backups are valid.

## UI Access

These features are also available through the web UI:

1. Open the application homepage
2. Scroll to the **Admin** section
3. Click on **WhatsApp Store Backup** card
4. Use the **Client Control** section to stop/start
5. Use Export/Import/Vacuum buttons as needed
