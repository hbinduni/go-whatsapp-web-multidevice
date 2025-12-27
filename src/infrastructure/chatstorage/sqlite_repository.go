// Package chatstorage provides SQLite-based storage for chat history and messages.
// The storage functionality is split across multiple files for better organization:
//   - sqlite_repository.go: Core struct, constructor, shared helpers
//   - chat_ops.go: Chat CRUD operations (StoreChat, GetChat, GetChats, DeleteChat)
//   - message_ops.go: Message CRUD operations (StoreMessage, GetMessages, SearchMessages)
//   - reaction_ops.go: Reaction operations (StoreReaction, GetReactionsForMessage)
//   - schema.go: Database schema initialization and migrations
package chatstorage

import (
	"database/sql"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
)

// ErrNoRows is an alias for sql.ErrNoRows for cleaner error handling
var ErrNoRows = sql.ErrNoRows

// SQLiteRepository implements Repository using SQLite
type SQLiteRepository struct {
	db *sql.DB
}

// NewStorageRepository creates a new SQLite repository
func NewStorageRepository(db *sql.DB) domainChatStorage.IChatStorageRepository {
	return &SQLiteRepository{db: db}
}

// getCount is a private helper for count queries
func (r *SQLiteRepository) getCount(query string, args ...any) (int64, error) {
	var count int64
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// scanMessage is a private helper for scanning message rows
func (r *SQLiteRepository) scanMessage(scanner interface{ Scan(...any) error }) (*domainChatStorage.Message, error) {
	message := &domainChatStorage.Message{}
	err := scanner.Scan(
		&message.ID, &message.ChatJID, &message.Sender, &message.Content,
		&message.Timestamp, &message.IsFromMe, &message.MediaType, &message.Filename,
		&message.URL, &message.MediaKey, &message.FileSHA256, &message.FileEncSHA256,
		&message.FileLength, &message.Status, &message.DeliveredAt, &message.ReadAt,
		&message.PlayedAt, &message.CreatedAt, &message.UpdatedAt,
	)
	return message, err
}

// scanChat is a private helper for scanning chat rows
func (r *SQLiteRepository) scanChat(scanner interface{ Scan(...any) error }) (*domainChatStorage.Chat, error) {
	chat := &domainChatStorage.Chat{}
	err := scanner.Scan(
		&chat.JID, &chat.Name, &chat.LastMessageTime, &chat.EphemeralExpiration,
		&chat.CreatedAt, &chat.UpdatedAt,
	)
	return chat, err
}

// scanChatWithLastMessage scans a chat row that includes last message preview fields
func (r *SQLiteRepository) scanChatWithLastMessage(scanner interface{ Scan(...any) error }) (*domainChatStorage.Chat, error) {
	chat := &domainChatStorage.Chat{}
	err := scanner.Scan(
		&chat.JID, &chat.Name, &chat.LastMessageTime, &chat.EphemeralExpiration,
		&chat.CreatedAt, &chat.UpdatedAt,
		&chat.LastMessage, &chat.LastMessageFromMe, &chat.LastMessageType,
	)
	return chat, err
}
