package whatsapp

import (
	"fmt"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// MessageRetryCache stores sent messages for retry handling.
// When WhatsApp can't decrypt a message (e.g., first message to a new contact),
// it requests a retry. whatsmeow needs the original message to resend it.
// Keys are formatted as "devicePhone:messageID" for multi-client isolation.
type MessageRetryCache struct {
	mu       sync.RWMutex
	messages map[string]*cachedMessage
	ttl      time.Duration
}

type cachedMessage struct {
	message   *waE2E.Message
	timestamp time.Time
}

// Global message retry cache (shared across all clients but keyed by device)
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

// makeCacheKey creates a unique cache key combining device phone and message ID
func makeCacheKey(devicePhone, messageID string) string {
	return fmt.Sprintf("%s:%s", devicePhone, messageID)
}

// Store saves a message for potential retry with device isolation
func (c *MessageRetryCache) Store(devicePhone, messageID string, msg *waE2E.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := makeCacheKey(devicePhone, messageID)
	c.messages[key] = &cachedMessage{
		message:   msg,
		timestamp: time.Now(),
	}
}

// Get retrieves a message by device phone and message ID for retry
func (c *MessageRetryCache) Get(devicePhone, messageID string) *waE2E.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := makeCacheKey(devicePhone, messageID)
	if cached, ok := c.messages[key]; ok {
		return cached.message
	}
	return nil
}

// Delete removes a message from cache (after successful delivery)
func (c *MessageRetryCache) Delete(devicePhone, messageID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := makeCacheKey(devicePhone, messageID)
	delete(c.messages, key)
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

// StoreMessageForRetry stores a message in the global retry cache with device isolation
func StoreMessageForRetry(devicePhone, messageID string, msg *waE2E.Message) {
	globalRetryCache.Store(devicePhone, messageID, msg)
}

// GetMessageForRetryHandler returns a callback function for whatsmeow's GetMessageForRetry
// This is used when WhatsApp requests a message retry (e.g., for new encryption sessions)
// The devicePhone is captured in the closure for multi-client isolation
func GetMessageForRetryHandler(devicePhone string) func(requester types.JID, to types.JID, messageID types.MessageID) *waE2E.Message {
	return func(requester types.JID, to types.JID, messageID types.MessageID) *waE2E.Message {
		msg := globalRetryCache.Get(devicePhone, messageID)
		if msg != nil {
			log.Debugf("[%s] Found message %s in retry cache for %s -> %s", devicePhone, messageID, requester.String(), to.String())
		} else {
			log.Warnf("[%s] Message %s not found in retry cache for %s -> %s", devicePhone, messageID, requester.String(), to.String())
		}
		return msg
	}
}
