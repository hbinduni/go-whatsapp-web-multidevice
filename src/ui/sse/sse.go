package sse

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// EventType defines the type of SSE event
type EventType string

const (
	// Message events
	EventMessageReceived    EventType = "message_received"
	EventMessageSent        EventType = "message_sent"
	EventMessageStatusUpdate EventType = "message_status_update"
	EventMessageDeleted     EventType = "message_deleted"
	EventMessageRevoked     EventType = "message_revoked"

	// Chat events
	EventChatUpdated   EventType = "chat_updated"
	EventChatCreated   EventType = "chat_created"
	EventHistorySync   EventType = "history_sync"

	// Connection events
	EventConnectionStatus EventType = "connection_status"
	EventLoginSuccess     EventType = "login_success"
	EventLogoutComplete   EventType = "logout_complete"
	EventQRCode           EventType = "qr_code"

	// Presence events
	EventPresenceUpdate EventType = "presence_update"

	// Group events
	EventGroupUpdated EventType = "group_updated"

	// Receipt events
	EventReceipt EventType = "receipt"
)

// Event represents an SSE event to be broadcasted
type Event struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Client represents an SSE client connection
type Client struct {
	ID      string
	Channel chan Event
}

// Hub manages SSE client connections and event broadcasting
type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan string
	broadcast  chan Event
	mu         sync.RWMutex
}

var (
	// Global SSE hub instance
	sseHub *Hub
	once   sync.Once
)

// GetHub returns the singleton SSE hub instance
func GetHub() *Hub {
	once.Do(func() {
		sseHub = &Hub{
			clients:    make(map[string]*Client),
			register:   make(chan *Client),
			unregister: make(chan string),
			broadcast:  make(chan Event, 100), // buffered channel for non-blocking broadcast
		}
	})
	return sseHub
}

// Run starts the SSE hub event loop
func (h *Hub) Run() {
	logrus.Info("[SSE] Hub started")
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
			logrus.Infof("[SSE] Client registered: %s (total: %d)", client.ID, len(h.clients))

		case clientID := <-h.unregister:
			h.mu.Lock()
			if client, ok := h.clients[clientID]; ok {
				close(client.Channel)
				delete(h.clients, clientID)
				logrus.Infof("[SSE] Client unregistered: %s (total: %d)", clientID, len(h.clients))
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			h.mu.RLock()
			clientCount := len(h.clients)
			if clientCount > 0 {
				logrus.Debugf("[SSE] Broadcasting event %s to %d clients", event.Type, clientCount)
				for _, client := range h.clients {
					select {
					case client.Channel <- event:
					default:
						// Channel is full, skip this client
						logrus.Warnf("[SSE] Client %s channel full, skipping event", client.ID)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a new client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(clientID string) {
	h.unregister <- clientID
}

// Broadcast sends an event to all connected clients
func (h *Hub) Broadcast(event Event) {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	h.broadcast <- event
}

// NewClient creates a new SSE client
func NewClient() *Client {
	return &Client{
		ID:      uuid.New().String(),
		Channel: make(chan Event, 50), // buffered channel per client
	}
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// FormatSSE formats an event as SSE data
func FormatSSE(event Event) string {
	data, err := json.Marshal(event)
	if err != nil {
		logrus.Errorf("[SSE] Failed to marshal event: %v", err)
		return ""
	}
	return fmt.Sprintf("id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, string(data))
}

// BroadcastMessage is a helper function to broadcast a simple message event
func BroadcastMessage(eventType EventType, code, message string, data any) {
	GetHub().Broadcast(Event{
		Type:    eventType,
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// BroadcastMessageReceived broadcasts a new incoming message event
func BroadcastMessageReceived(messageID, chatJID, sender, content string, timestamp time.Time, isFromMe bool, mediaType string) {
	BroadcastMessage(EventMessageReceived, "MESSAGE_RECEIVED", "New message received", map[string]any{
		"message_id": messageID,
		"chat_jid":   chatJID,
		"sender":     sender,
		"content":    content,
		"timestamp":  timestamp,
		"is_from_me": isFromMe,
		"media_type": mediaType,
	})
}

// BroadcastMessageSent broadcasts a sent message confirmation event
func BroadcastMessageSent(messageID, chatJID, content string, timestamp time.Time) {
	BroadcastMessage(EventMessageSent, "MESSAGE_SENT", "Message sent successfully", map[string]any{
		"message_id": messageID,
		"chat_jid":   chatJID,
		"content":    content,
		"timestamp":  timestamp,
	})
}

// BroadcastMessageStatus broadcasts a message status update event
func BroadcastMessageStatus(messageID, chatJID, status string, timestamp time.Time) {
	BroadcastMessage(EventMessageStatusUpdate, "MESSAGE_STATUS_UPDATE", "Message status updated", map[string]any{
		"message_id": messageID,
		"chat_jid":   chatJID,
		"status":     status,
		"timestamp":  timestamp,
	})
}

// BroadcastChatUpdated broadcasts a chat update event
func BroadcastChatUpdated(chatJID, name string, lastMessageTime time.Time, unreadCount int) {
	BroadcastMessage(EventChatUpdated, "CHAT_UPDATED", "Chat updated", map[string]any{
		"chat_jid":          chatJID,
		"name":              name,
		"last_message_time": lastMessageTime,
		"unread_count":      unreadCount,
	})
}

// BroadcastPresenceUpdate broadcasts a presence update event
func BroadcastPresenceUpdate(jid string, isOnline bool, lastSeen time.Time) {
	BroadcastMessage(EventPresenceUpdate, "PRESENCE_UPDATE", "Presence updated", map[string]any{
		"jid":       jid,
		"is_online": isOnline,
		"last_seen": lastSeen,
	})
}

// BroadcastReceipt broadcasts a receipt event (delivered, read, played)
func BroadcastReceipt(messageIDs []string, chatJID, receiptType string, timestamp time.Time) {
	BroadcastMessage(EventReceipt, "RECEIPT", "Receipt received", map[string]any{
		"message_ids":  messageIDs,
		"chat_jid":     chatJID,
		"receipt_type": receiptType,
		"timestamp":    timestamp,
	})
}

// BroadcastConnectionStatus broadcasts connection status changes
func BroadcastConnectionStatus(isConnected, isLoggedIn bool, deviceID string) {
	BroadcastMessage(EventConnectionStatus, "CONNECTION_STATUS", "Connection status changed", map[string]any{
		"is_connected": isConnected,
		"is_logged_in": isLoggedIn,
		"device_id":    deviceID,
	})
}
