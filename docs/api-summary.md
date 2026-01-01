# REST API Summary

This document provides a comprehensive overview of all REST API endpoints.

## Route Structure

| Route Group | Pattern | Auth Required | Description |
|-------------|---------|---------------|-------------|
| Auth | `/auth/*` | No | Login, token refresh |
| Admin | `/admin/*` | Yes (JWT) | System management |
| API | `/api/:phone/*` | Yes (JWT) | Per-client WhatsApp operations |
| SSE | `/events` | Optional | Real-time events |
| WebSocket | `/ws` | Optional | WebSocket connection |

## Multi-Client Architecture

All WhatsApp operations require a phone number in the URL path:

```
/api/{phone_number}/{endpoint}
```

**Example:** Send message via client `628192191202`:
```bash
curl -X POST http://localhost:3000/api/628192191202/send/message \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"phone": "628123456789", "message": "Hello!"}'
```

---

## Authentication (`/auth`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/login` | Login with username/password, returns JWT tokens |
| POST | `/auth/refresh` | Refresh access token using refresh token |
| POST | `/auth/logout` | Invalidate current session |

### Login Request
```json
{
  "username": "admin",
  "password": "your-password"
}
```

### Login Response
```json
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "expires_in": 3600
}
```

---

## Admin (`/admin`)

### Storage Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/admin/storage/stats` | Get database statistics (total chats, messages, size) |
| POST | `/admin/storage/cleanup` | Clean old messages, empty chats by pattern/age |
| POST | `/admin/storage/vacuum` | Optimize database, reclaim disk space |
| DELETE | `/admin/storage/chats` | Delete chats by pattern or specific JIDs |

### Client Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/admin/clients` | List all registered WhatsApp clients with status |
| GET | `/admin/clients/:phone/status` | Get specific client's connection status |
| POST | `/admin/clients/:phone/connect` | Connect a disconnected client |
| POST | `/admin/clients/:phone/disconnect` | Disconnect a client |

---

## App Management (`/api/:phone/app`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/app/login` | Get QR code for WhatsApp Web login |
| GET | `/app/login-with-code` | Get 8-digit pairing code for login |
| GET | `/app/logout` | Logout from WhatsApp, clear session |
| GET | `/app/reconnect` | Reconnect to WhatsApp servers |
| GET | `/app/devices` | List all linked devices |
| GET | `/app/status` | Get current connection status |

---

## User (`/api/:phone/user`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/user/info` | Get user profile info by phone number |
| GET | `/user/avatar` | Get user's profile picture URL |
| POST | `/user/avatar` | Upload/change your profile picture |
| POST | `/user/pushname` | Change your display name |
| GET | `/user/my/privacy` | Get your privacy settings |
| GET | `/user/my/groups` | List all groups you're a member of |
| GET | `/user/my/newsletters` | List subscribed WhatsApp channels |
| GET | `/user/my/contacts` | List your synced contacts |
| GET | `/user/check` | Check if phone number is registered on WhatsApp |
| GET | `/user/business-profile` | Get business profile information |

---

## Send Messages (`/api/:phone/send`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/send/message` | Send text message (supports mentions with @phone) |
| POST | `/send/image` | Send image with optional caption |
| POST | `/send/file` | Send document/file attachment |
| POST | `/send/video` | Send video (auto-compressed with FFmpeg) |
| POST | `/send/audio` | Send voice message/audio file |
| POST | `/send/sticker` | Send sticker (auto-converts to WebP) |
| POST | `/send/contact` | Send contact card (vCard) |
| POST | `/send/link` | Send link with preview |
| POST | `/send/location` | Send location pin with coordinates |
| POST | `/send/poll` | Create and send a poll |
| POST | `/send/presence` | Update online/offline/typing presence |
| POST | `/send/chat-presence` | Send typing/recording indicator to chat |

### Send Message Request
```json
{
  "phone": "628123456789",
  "message": "Hello @628111222333!",
  "reply_message_id": "optional-message-id-to-reply"
}
```

---

## Messages (`/api/:phone/message`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/message/:id/reaction` | React to message with emoji |
| POST | `/message/:id/revoke` | Revoke/unsend message for everyone |
| POST | `/message/:id/delete` | Delete message locally |
| POST | `/message/:id/update` | Edit a sent message |
| POST | `/message/:id/read` | Mark message as read |
| POST | `/message/:id/star` | Star/favorite a message |
| POST | `/message/:id/unstar` | Remove star from message |
| GET | `/message/:id/download` | Download media attachment from message |

---

## Chats (`/api/:phone/chat`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/chats` | List all chats with pagination and search |
| GET | `/chat/:jid/messages` | Get messages from a specific chat |
| POST | `/chat/:jid/pin` | Pin or unpin a chat |
| POST | `/chat/:jid/disappearing` | Set disappearing messages timer |

### List Chats Query Parameters
- `limit` - Number of chats to return (default: 25)
- `offset` - Pagination offset (default: 0)
- `search` - Search by chat name
- `has_media` - Filter chats with media

### Get Messages Query Parameters
- `limit` - Number of messages (default: 50)
- `offset` - Pagination offset
- `media_only` - Only return media messages
- `search` - Search message content
- `start_time` - Filter by date (RFC3339)
- `end_time` - Filter by date (RFC3339)
- `is_from_me` - Filter sent/received

---

## Groups (`/api/:phone/group`)

### Group Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/group` | Create a new group |
| GET | `/group/info` | Get group details (name, description, settings) |
| POST | `/group/leave` | Leave a group |
| POST | `/group/photo` | Set group profile photo |
| POST | `/group/name` | Change group name |
| POST | `/group/topic` | Set group description |
| POST | `/group/locked` | Lock/unlock group settings (admin only) |
| POST | `/group/announce` | Enable/disable announce-only mode |
| GET | `/group/invite-link` | Get group invite link |

### Join Group

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/group/info-from-link` | Preview group info from invite link |
| POST | `/group/join-with-link` | Join group using invite link |

### Participant Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/group/participants` | List all group members |
| GET | `/group/participants/export` | Export members as CSV file |
| POST | `/group/participants` | Add members to group |
| POST | `/group/participants/remove` | Remove members from group |
| POST | `/group/participants/promote` | Promote members to admin |
| POST | `/group/participants/demote` | Demote admins to regular member |

### Join Requests

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/group/participant-requests` | List pending join requests |
| POST | `/group/participant-requests/approve` | Approve join request |
| POST | `/group/participant-requests/reject` | Reject join request |

---

## Newsletter (`/api/:phone/newsletter`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/newsletter/unfollow` | Unsubscribe from a WhatsApp channel |

---

## History Sync (`/api/:phone/history`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/history/sync` | Manually trigger history synchronization |
| GET | `/history/status` | Get current sync status and progress |

---

## Real-time Events

### SSE (Server-Sent Events)

Connect to `/events` for real-time message and status updates:

```javascript
const eventSource = new EventSource('/events');
eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event:', data.type, data.payload);
};
```

### WebSocket

Connect to `/ws` for bidirectional real-time communication.

---

## Error Responses

All endpoints return errors in this format:

```json
{
  "code": 400,
  "message": "Error description",
  "results": null
}
```

## Success Responses

```json
{
  "code": 200,
  "message": "Success message",
  "results": { ... }
}
```
