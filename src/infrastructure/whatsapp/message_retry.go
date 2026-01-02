package whatsapp

import (
	"sync"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// MessageRetryCache stores sent messages for retry handling.
// When WhatsApp can't decrypt a message (e.g., first message to a new contact),
// it requests a retry. whatsmeow needs the original message to resend it.
type MessageRetryCache struct {
	mu       sync.RWMutex
	messages map[string]*cachedMessage
	ttl      time.Duration
}

type cachedMessage struct {
	message   *waE2E.Message
	recipient types.JID
	timestamp time.Time
}

// Global message retry cache (shared across all clients)
var globalRetryCache = NewMessageRetryCache(5 * time.Minute)

// NewMessageRetryCache creates a new cache with the specified TTL
func NewMessageRetryCache(ttl time.Duration) *MessageRetryCache {
	cache := &MessageRetryCache{
		messages: make(map[string]*cachedMessage),
		ttl:      ttl,
	}
	// Start cleanup goroutine
	go cache.cleanupLoop()
	return cache
}

// Store saves a message for potential retry
func (c *MessageRetryCache) Store(messageID string, recipient types.JID, msg *waE2E.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages[messageID] = &cachedMessage{
		message:   msg,
		recipient: recipient,
		timestamp: time.Now(),
	}
}

// Get retrieves a message by ID for retry
func (c *MessageRetryCache) Get(messageID string) *waE2E.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if cached, ok := c.messages[messageID]; ok {
		return cached.message
	}
	return nil
}

// Delete removes a message from cache (after successful delivery)
func (c *MessageRetryCache) Delete(messageID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.messages, messageID)
}

// cleanupLoop periodically removes expired messages
func (c *MessageRetryCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

func (c *MessageRetryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for id, cached := range c.messages {
		if now.Sub(cached.timestamp) > c.ttl {
			delete(c.messages, id)
		}
	}
}

// GetGlobalRetryCache returns the global message retry cache
func GetGlobalRetryCache() *MessageRetryCache {
	return globalRetryCache
}

// StoreMessageForRetry stores a message in the global retry cache
func StoreMessageForRetry(messageID string, recipient types.JID, msg *waE2E.Message) {
	globalRetryCache.Store(messageID, recipient, msg)
}

// GetMessageForRetryHandler returns a callback function for whatsmeow's GetMessageForRetry
// This is used when WhatsApp requests a message retry (e.g., for new encryption sessions)
func GetMessageForRetryHandler() func(requester types.JID, to types.JID, messageID types.MessageID) *waE2E.Message {
	return func(requester types.JID, to types.JID, messageID types.MessageID) *waE2E.Message {
		msg := globalRetryCache.Get(messageID)
		if msg != nil {
			log.Debugf("Found message %s in retry cache for %s -> %s", messageID, requester.String(), to.String())
		} else {
			log.Warnf("Message %s not found in retry cache for %s -> %s", messageID, requester.String(), to.String())
		}
		return msg
	}
}
