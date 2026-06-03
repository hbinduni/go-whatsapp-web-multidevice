package chatstorage

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type IChatStorageRepository interface {
	// Chat operations
	CreateMessage(ctx context.Context, evt *events.Message) error
	StoreChat(chat *Chat) error
	StoreChatsBatch(chats []*Chat, skipExisting bool) (imported, skipped int64, err error) // Batch insert chats
	GetChat(jid string) (*Chat, error)
	GetChats(filter *ChatFilter) ([]*Chat, error)
	DeleteChat(jid string) error

	// Message operations
	StoreMessage(message *Message) error
	StoreMessagesBatch(messages []*Message) error
	ImportMessagesBatch(messages []*Message, skipExisting bool) (imported, skipped int64, err error) // Batch import with skip/upsert mode
	GetMessageByID(id string) (*Message, error)                                                      // New method for efficient ID-only search
	GetMessages(filter *MessageFilter) ([]*Message, error)
	SearchMessages(chatJID, searchText string, limit int) ([]*Message, error) // Database-level search
	DeleteMessage(id, chatJID string) error
	StoreSentMessageWithContext(ctx context.Context, messageID string, senderJID string, recipientJID string, content string, timestamp time.Time) error
	StoreSentMediaMessageWithContext(ctx context.Context, messageID string, senderJID string, recipientJID string, content string, mediaType string, filename string, mediaData []byte, timestamp time.Time) error // Store a sent media message and upload its bytes to S3
	UpdateMessageStatus(ctx context.Context, messageID string, status string, statusTime time.Time) error                                                                                                          // Update message delivery/read status

	// Reaction operations
	StoreReaction(ctx context.Context, reaction *MessageReaction) error                                // Upsert a reaction (empty emoji = delete)
	GetReactionsForMessage(messageID, chatJID string) ([]MessageReaction, error)                       // Get all reactions for a message
	GetReactionsForMessages(messageIDs []string, chatJID string) (map[string][]MessageReaction, error) // Batch get reactions

	// Statistics
	GetChatMessageCount(chatJID string) (int64, error)
	GetTotalMessageCount() (int64, error)
	GetTotalChatCount() (int64, error)
	GetChatNameWithPushName(jid types.JID, chatJID string, senderUser string, pushName string) string
	GetStorageStatistics() (chatCount int64, messageCount int64, err error)

	// Cleanup operations
	TruncateAllChats() error
	TruncateAllDataWithLogging(logPrefix string) error
	CleanupOwnerNamePollution(ownerName string) (int64, error)

	// Admin operations
	DeleteChatsByPattern(pattern string) (chatsDeleted, messagesDeleted int64, deletedJIDs []string, err error)
	DeleteChatsByJIDs(jids []string) (chatsDeleted, messagesDeleted int64, err error)
	GetDetailedStats() (*DetailedStats, error)
	VacuumDatabase() error
	GetDatabaseSize() (int64, error)
	GetMessageDateRange() (oldest, newest *time.Time, err error)
	CountEmptyChats() (int64, error)
	DeleteEmptyChats() (deleted int64, err error)
	DeleteMessagesOlderThan(before time.Time) (deleted int64, err error)

	// Schema operations
	InitializeSchema() error
}
