// Package whatsapp provides WhatsApp Web API integration using the whatsmeow library.
// This package is split into multiple files for better organization:
//   - client.go: Client initialization and global state management
//   - cleanup.go: Database and file cleanup operations
//   - handlers.go: Event handlers for WhatsApp events
//   - history_sync.go: History synchronization processing
//   - jid.go: JID normalization utilities
//   - logger.go: Custom logger implementation
//   - webhook.go, webhook_forward.go: Webhook forwarding
//   - event_*.go: Event-specific webhook payloads
package whatsapp
