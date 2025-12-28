package chatstorage

import "time"

// Chat represents a WhatsApp chat/conversation
type Chat struct {
	JID                 string    `db:"jid"`
	Name                string    `db:"name"`
	LastMessageTime     time.Time `db:"last_message_time"`
	EphemeralExpiration uint32    `db:"ephemeral_expiration"`
	CreatedAt           time.Time `db:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"`
	// Last message preview fields (populated by GetChats query)
	LastMessage       *string `db:"last_message"`
	LastMessageFromMe *bool   `db:"last_message_from_me"`
	LastMessageType   *string `db:"last_message_type"`
	UnreadCount       *int    `db:"unread_count"`
}

// Message represents a WhatsApp message
type Message struct {
	ID            string     `db:"id"`
	ChatJID       string     `db:"chat_jid"`
	Sender        string     `db:"sender"`
	Content       string     `db:"content"`
	Timestamp     time.Time  `db:"timestamp"`
	IsFromMe      bool       `db:"is_from_me"`
	MediaType     string     `db:"media_type"`
	Filename      string     `db:"filename"`
	URL           string     `db:"url"`
	MediaKey      []byte     `db:"media_key"`
	FileSHA256    []byte     `db:"file_sha256"`
	FileEncSHA256 []byte     `db:"file_enc_sha256"`
	FileLength    uint64     `db:"file_length"`
	Status        string     `db:"status"`       // Message status: sent, delivered, read, played
	DeliveredAt   *time.Time `db:"delivered_at"` // When message was delivered
	ReadAt        *time.Time `db:"read_at"`      // When message was read
	PlayedAt      *time.Time `db:"played_at"`    // When view-once message was played
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	// Reactions is populated when fetching messages (not stored in messages table)
	Reactions []MessageReaction `db:"-" json:"reactions,omitempty"`
}

// MediaInfo represents downloadable media information
type MediaInfo struct {
	MessageID     string
	ChatJID       string
	MediaType     string
	Filename      string
	URL           string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	FileLength    uint64
}

// MessageReaction represents a reaction to a message
type MessageReaction struct {
	MessageID string    `db:"message_id" json:"message_id"` // Target message ID being reacted to
	ChatJID   string    `db:"chat_jid" json:"chat_jid"`
	SenderJID string    `db:"sender_jid" json:"sender_jid"` // Who sent the reaction
	Emoji     string    `db:"emoji" json:"emoji"`           // The reaction emoji (empty = removed)
	Timestamp time.Time `db:"timestamp" json:"timestamp"`
}

// MessageFilter represents query filters for messages
type MessageFilter struct {
	ChatJID   string
	Limit     int
	Offset    int
	StartTime *time.Time
	EndTime   *time.Time
	MediaOnly bool
	IsFromMe  *bool
}

// ChatFilter represents query filters for chats
type ChatFilter struct {
	Limit      int
	Offset     int
	SearchName string
	HasMedia   bool
}

// DetailedStats represents detailed storage statistics for admin operations
type DetailedStats struct {
	TotalChats        int64      `json:"total_chats"`
	TotalMessages     int64      `json:"total_messages"`
	DatabaseSizeBytes int64      `json:"database_size_bytes"`
	OldestMessage     *time.Time `json:"oldest_message,omitempty"`
	NewestMessage     *time.Time `json:"newest_message,omitempty"`
	EmptyChats        int64      `json:"empty_chats"`
	MediaMessages     int64      `json:"media_messages"`
	TextMessages      int64      `json:"text_messages"`
}
