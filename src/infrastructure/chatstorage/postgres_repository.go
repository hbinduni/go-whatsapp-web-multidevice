// Package chatstorage provides PostgreSQL-based storage for multi-client chat history.
// This repository adds device_id column to all tables for multi-tenant isolation.
package chatstorage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// PostgresRepository implements Repository using PostgreSQL with device_id isolation
type PostgresRepository struct {
	db       *sql.DB
	deviceID string // Phone number or device identifier for multi-client isolation
}

// NewPostgresRepository creates a new PostgreSQL repository with device isolation
func NewPostgresRepository(db *sql.DB, deviceID string) domainChatStorage.IChatStorageRepository {
	return &PostgresRepository{db: db, deviceID: deviceID}
}

// getCount is a private helper for count queries
func (r *PostgresRepository) getCount(query string, args ...any) (int64, error) {
	var count int64
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// scanMessage is a private helper for scanning message rows
func (r *PostgresRepository) scanMessage(scanner interface{ Scan(...any) error }) (*domainChatStorage.Message, error) {
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
func (r *PostgresRepository) scanChat(scanner interface{ Scan(...any) error }) (*domainChatStorage.Chat, error) {
	chat := &domainChatStorage.Chat{}
	err := scanner.Scan(
		&chat.JID, &chat.Name, &chat.LastMessageTime, &chat.EphemeralExpiration,
		&chat.CreatedAt, &chat.UpdatedAt,
	)
	return chat, err
}

// scanChatWithLastMessage scans a chat row that includes last message preview fields
func (r *PostgresRepository) scanChatWithLastMessage(scanner interface{ Scan(...any) error }) (*domainChatStorage.Chat, error) {
	chat := &domainChatStorage.Chat{}
	err := scanner.Scan(
		&chat.JID, &chat.Name, &chat.LastMessageTime, &chat.EphemeralExpiration,
		&chat.CreatedAt, &chat.UpdatedAt,
		&chat.LastMessage, &chat.LastMessageFromMe, &chat.LastMessageType,
	)
	return chat, err
}

// ============================================================================
// Schema Operations
// ============================================================================

// InitializeSchema creates or migrates the database schema for PostgreSQL
func (r *PostgresRepository) InitializeSchema() error {
	version, err := r.getSchemaVersion()
	if err != nil {
		return err
	}

	migrations := r.getMigrations()
	for i := version; i < len(migrations); i++ {
		if err := r.runMigration(migrations[i], i+1); err != nil {
			return fmt.Errorf("failed to run migration %d: %w", i+1, err)
		}
	}

	return nil
}

func (r *PostgresRepository) getSchemaVersion() (int, error) {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_info (
			version INTEGER PRIMARY KEY DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return 0, err
	}

	var version int
	err = r.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_info").Scan(&version)
	if err != nil {
		return 0, err
	}

	return version, nil
}

func (r *PostgresRepository) runMigration(migration string, version int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(migration); err != nil {
		return err
	}

	// PostgreSQL uses INSERT ... ON CONFLICT
	if _, err := tx.Exec(`
		INSERT INTO schema_info (version) VALUES ($1)
		ON CONFLICT (version) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
	`, version); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresRepository) getMigrations() []string {
	return []string{
		// Migration 1: Initial schema with device_id for multi-client support
		`
		-- Create chats table with device_id
		CREATE TABLE IF NOT EXISTS chats (
			device_id TEXT NOT NULL,
			jid TEXT NOT NULL,
			name TEXT NOT NULL,
			last_message_time TIMESTAMP NOT NULL,
			ephemeral_expiration INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (device_id, jid)
		);

		-- Create messages table with device_id
		CREATE TABLE IF NOT EXISTS messages (
			device_id TEXT NOT NULL,
			id TEXT NOT NULL,
			chat_jid TEXT NOT NULL,
			sender TEXT NOT NULL,
			content TEXT,
			timestamp TIMESTAMP NOT NULL,
			is_from_me BOOLEAN DEFAULT FALSE,
			media_type TEXT,
			filename TEXT,
			url TEXT,
			media_key BYTEA,
			file_sha256 BYTEA,
			file_enc_sha256 BYTEA,
			file_length INTEGER DEFAULT 0,
			status TEXT DEFAULT 'sent',
			delivered_at TIMESTAMP,
			read_at TIMESTAMP,
			played_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (device_id, id, chat_jid),
			FOREIGN KEY (device_id, chat_jid) REFERENCES chats(device_id, jid) ON DELETE CASCADE
		);

		-- Create message_reactions table with device_id
		CREATE TABLE IF NOT EXISTS message_reactions (
			device_id TEXT NOT NULL,
			message_id TEXT NOT NULL,
			chat_jid TEXT NOT NULL,
			sender_jid TEXT NOT NULL,
			emoji TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			PRIMARY KEY (device_id, message_id, chat_jid, sender_jid)
		);

		-- Create indexes for performance
		CREATE INDEX IF NOT EXISTS idx_chats_device ON chats(device_id);
		CREATE INDEX IF NOT EXISTS idx_chats_last_message ON chats(device_id, last_message_time);
		CREATE INDEX IF NOT EXISTS idx_messages_device ON messages(device_id);
		CREATE INDEX IF NOT EXISTS idx_messages_chat_jid ON messages(device_id, chat_jid);
		CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(device_id, timestamp);
		CREATE INDEX IF NOT EXISTS idx_messages_id ON messages(id);
		CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(device_id, status);
		CREATE INDEX IF NOT EXISTS idx_reactions_message ON message_reactions(device_id, message_id, chat_jid);
		`,
	}
}

// ============================================================================
// Chat Operations
// ============================================================================

// StoreChat creates or updates a chat
func (r *PostgresRepository) StoreChat(chat *domainChatStorage.Chat) error {
	now := time.Now()
	chat.UpdatedAt = now

	query := `
		INSERT INTO chats (device_id, jid, name, last_message_time, ephemeral_expiration, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT(device_id, jid) DO UPDATE SET
			name = EXCLUDED.name,
			last_message_time = EXCLUDED.last_message_time,
			ephemeral_expiration = EXCLUDED.ephemeral_expiration,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.Exec(query, r.deviceID, chat.JID, chat.Name, chat.LastMessageTime, chat.EphemeralExpiration, now, chat.UpdatedAt)
	return err
}

// StoreChatsBatch inserts multiple chats in a single transaction
func (r *PostgresRepository) StoreChatsBatch(chats []*domainChatStorage.Chat, skipExisting bool) (imported, skipped int64, err error) {
	if len(chats) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var query string
	if skipExisting {
		query = `
			INSERT INTO chats (device_id, jid, name, last_message_time, ephemeral_expiration, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT(device_id, jid) DO NOTHING
		`
	} else {
		query = `
			INSERT INTO chats (device_id, jid, name, last_message_time, ephemeral_expiration, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT(device_id, jid) DO UPDATE SET
				name = EXCLUDED.name,
				last_message_time = EXCLUDED.last_message_time,
				ephemeral_expiration = EXCLUDED.ephemeral_expiration,
				updated_at = EXCLUDED.updated_at
		`
	}

	stmt, err := tx.Prepare(query)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, chat := range chats {
		chat.CreatedAt = now
		chat.UpdatedAt = now

		result, err := stmt.Exec(
			r.deviceID, chat.JID, chat.Name, chat.LastMessageTime,
			chat.EphemeralExpiration, chat.CreatedAt, chat.UpdatedAt,
		)
		if err != nil {
			return imported, skipped, fmt.Errorf("failed to store chat %s: %w", chat.JID, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			imported++
		} else {
			skipped++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return imported, skipped, nil
}

// GetChat retrieves a chat by JID
func (r *PostgresRepository) GetChat(jid string) (*domainChatStorage.Chat, error) {
	query := `
		SELECT jid, name, last_message_time, ephemeral_expiration, created_at, updated_at
		FROM chats
		WHERE device_id = $1 AND jid = $2
	`

	chat, err := r.scanChat(r.db.QueryRow(query, r.deviceID, jid))
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return chat, err
}

// GetChats retrieves chats with filtering
func (r *PostgresRepository) GetChats(filter *domainChatStorage.ChatFilter) ([]*domainChatStorage.Chat, error) {
	var conditions []string
	var args []any
	argNum := 1

	conditions = append(conditions, fmt.Sprintf("c.device_id = $%d", argNum))
	args = append(args, r.deviceID)
	argNum++

	query := `
		SELECT
			c.jid, c.name, c.last_message_time, c.ephemeral_expiration, c.created_at, c.updated_at,
			lm.content AS last_message,
			lm.is_from_me AS last_message_from_me,
			lm.media_type AS last_message_type
		FROM chats c
		LEFT JOIN (
			SELECT m1.device_id, m1.chat_jid, m1.content, m1.is_from_me, m1.media_type
			FROM messages m1
			INNER JOIN (
				SELECT device_id, chat_jid, MAX(timestamp) as max_ts
				FROM messages
				GROUP BY device_id, chat_jid
			) m2 ON m1.device_id = m2.device_id AND m1.chat_jid = m2.chat_jid AND m1.timestamp = m2.max_ts
		) lm ON c.device_id = lm.device_id AND c.jid = lm.chat_jid
	`

	if filter.SearchName != "" {
		conditions = append(conditions, fmt.Sprintf("c.name ILIKE $%d", argNum))
		args = append(args, "%"+filter.SearchName+"%")
		argNum++
	}

	if filter.HasMedia {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM messages m WHERE m.device_id = c.device_id AND m.chat_jid = c.jid AND m.media_type != '')")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY c.last_message_time DESC"

	if filter.Limit > 0 {
		if filter.Limit > 1000 {
			filter.Limit = 1000
		}
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filter.Limit)
		argNum++

		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argNum)
			args = append(args, filter.Offset)
		}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []*domainChatStorage.Chat
	for rows.Next() {
		chat, err := r.scanChatWithLastMessage(rows)
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}

	return chats, rows.Err()
}

// DeleteChat deletes a chat and all its messages
func (r *PostgresRepository) DeleteChat(jid string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Delete reactions first
	_, err = tx.Exec("DELETE FROM message_reactions WHERE device_id = $1 AND chat_jid = $2", r.deviceID, jid)
	if err != nil {
		logrus.Debugf("Failed to delete reactions for chat %s: %v", jid, err)
	}

	// Delete messages
	_, err = tx.Exec("DELETE FROM messages WHERE device_id = $1 AND chat_jid = $2", r.deviceID, jid)
	if err != nil {
		return err
	}

	// Delete chat
	_, err = tx.Exec("DELETE FROM chats WHERE device_id = $1 AND jid = $2", r.deviceID, jid)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetTotalChatCount returns the total number of chats for this device
func (r *PostgresRepository) GetTotalChatCount() (int64, error) {
	return r.getCount("SELECT COUNT(*) FROM chats WHERE device_id = $1", r.deviceID)
}

// TruncateAllChats deletes all chats for this device
func (r *PostgresRepository) TruncateAllChats() error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete reactions
	_, err = tx.Exec("DELETE FROM message_reactions WHERE device_id = $1", r.deviceID)
	if err != nil {
		logrus.Debugf("Failed to delete message_reactions: %v", err)
	}

	// Delete messages
	_, err = tx.Exec("DELETE FROM messages WHERE device_id = $1", r.deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	// Delete chats
	_, err = tx.Exec("DELETE FROM chats WHERE device_id = $1", r.deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete chats: %w", err)
	}

	return tx.Commit()
}

// GetChatNameWithPushName determines the appropriate name for a chat
func (r *PostgresRepository) GetChatNameWithPushName(jid types.JID, chatJID string, senderUser string, pushName string) string {
	existingChat, err := r.GetChat(chatJID)
	if err == nil && existingChat != nil && existingChat.Name != "" {
		if pushName != "" && (existingChat.Name == jid.User || existingChat.Name == senderUser) {
			return pushName
		}
		return existingChat.Name
	}

	var name string
	switch jid.Server {
	case "g.us":
		name = fmt.Sprintf("Group %s", jid.User)
	case "newsletter":
		name = fmt.Sprintf("Newsletter %s", jid.User)
	default:
		if pushName != "" && pushName != senderUser && pushName != jid.User {
			name = pushName
		} else if senderUser != "" {
			name = senderUser
		} else {
			name = jid.User
		}
	}

	return name
}

// TruncateAllDataWithLogging performs truncation with detailed logging
func (r *PostgresRepository) TruncateAllDataWithLogging(logPrefix string) error {
	chatCount, messageCount, err := r.GetStorageStatistics()
	if err != nil {
		logrus.Warnf("[%s] Failed to get storage statistics before truncation: %v", logPrefix, err)
	} else {
		logrus.Infof("[%s] Storage before truncation: %d chats, %d messages", logPrefix, chatCount, messageCount)
	}

	if err := r.TruncateAllChats(); err != nil {
		return fmt.Errorf("failed to truncate chatstorage data: %w", err)
	}

	chatCountAfter, messageCountAfter, err := r.GetStorageStatistics()
	if err != nil {
		logrus.Warnf("[%s] Failed to get storage statistics after truncation: %v", logPrefix, err)
	} else {
		logrus.Infof("[%s] Storage after truncation: %d chats, %d messages", logPrefix, chatCountAfter, messageCountAfter)
	}

	return nil
}

// ============================================================================
// Message Operations
// ============================================================================

// StoreMessage creates or updates a message
func (r *PostgresRepository) StoreMessage(message *domainChatStorage.Message) error {
	now := time.Now()
	message.CreatedAt = now
	message.UpdatedAt = now

	if message.Status == "" {
		message.Status = "sent"
	}

	if message.Content == "" && message.MediaType == "" {
		return nil
	}

	query := `
		INSERT INTO messages (
			device_id, id, chat_jid, sender, content, timestamp, is_from_me,
			media_type, filename, url, media_key, file_sha256,
			file_enc_sha256, file_length, status, delivered_at, read_at,
			played_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT(device_id, id, chat_jid) DO UPDATE SET
			sender = EXCLUDED.sender,
			content = EXCLUDED.content,
			timestamp = EXCLUDED.timestamp,
			is_from_me = EXCLUDED.is_from_me,
			media_type = EXCLUDED.media_type,
			filename = EXCLUDED.filename,
			url = EXCLUDED.url,
			media_key = EXCLUDED.media_key,
			file_sha256 = EXCLUDED.file_sha256,
			file_enc_sha256 = EXCLUDED.file_enc_sha256,
			file_length = EXCLUDED.file_length,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.Exec(query,
		r.deviceID, message.ID, message.ChatJID, message.Sender, message.Content,
		message.Timestamp, message.IsFromMe, message.MediaType, message.Filename,
		message.URL, message.MediaKey, message.FileSHA256, message.FileEncSHA256,
		message.FileLength, message.Status, message.DeliveredAt, message.ReadAt,
		message.PlayedAt, message.CreatedAt, message.UpdatedAt,
	)

	return err
}

// StoreMessagesBatch creates or updates multiple messages in a single transaction
func (r *PostgresRepository) StoreMessagesBatch(messages []*domainChatStorage.Message) error {
	if len(messages) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT INTO messages (
			device_id, id, chat_jid, sender, content, timestamp, is_from_me,
			media_type, filename, url, media_key, file_sha256,
			file_enc_sha256, file_length, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT(device_id, id, chat_jid) DO UPDATE SET
			sender = EXCLUDED.sender,
			content = EXCLUDED.content,
			timestamp = EXCLUDED.timestamp,
			is_from_me = EXCLUDED.is_from_me,
			media_type = EXCLUDED.media_type,
			filename = EXCLUDED.filename,
			url = EXCLUDED.url,
			media_key = EXCLUDED.media_key,
			file_sha256 = EXCLUDED.file_sha256,
			file_enc_sha256 = EXCLUDED.file_enc_sha256,
			file_length = EXCLUDED.file_length,
			updated_at = EXCLUDED.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, message := range messages {
		if message.Content == "" && message.MediaType == "" {
			continue
		}

		message.CreatedAt = now
		message.UpdatedAt = now

		_, err = stmt.Exec(
			r.deviceID, message.ID, message.ChatJID, message.Sender, message.Content,
			message.Timestamp, message.IsFromMe, message.MediaType, message.Filename,
			message.URL, message.MediaKey, message.FileSHA256, message.FileEncSHA256,
			message.FileLength, message.CreatedAt, message.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to store message %s: %w", message.ID, err)
		}
	}

	return tx.Commit()
}

// ImportMessagesBatch inserts multiple messages for imports
func (r *PostgresRepository) ImportMessagesBatch(messages []*domainChatStorage.Message, skipExisting bool) (imported, skipped int64, err error) {
	if len(messages) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var query string
	if skipExisting {
		query = `
			INSERT INTO messages (
				device_id, id, chat_jid, sender, content, timestamp, is_from_me,
				media_type, filename, url, file_length, status,
				delivered_at, read_at, played_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			ON CONFLICT(device_id, id, chat_jid) DO NOTHING
		`
	} else {
		query = `
			INSERT INTO messages (
				device_id, id, chat_jid, sender, content, timestamp, is_from_me,
				media_type, filename, url, file_length, status,
				delivered_at, read_at, played_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			ON CONFLICT(device_id, id, chat_jid) DO UPDATE SET
				sender = EXCLUDED.sender,
				content = EXCLUDED.content,
				timestamp = EXCLUDED.timestamp,
				is_from_me = EXCLUDED.is_from_me,
				media_type = EXCLUDED.media_type,
				filename = EXCLUDED.filename,
				url = EXCLUDED.url,
				file_length = EXCLUDED.file_length,
				status = EXCLUDED.status,
				delivered_at = EXCLUDED.delivered_at,
				read_at = EXCLUDED.read_at,
				played_at = EXCLUDED.played_at,
				updated_at = EXCLUDED.updated_at
		`
	}

	stmt, err := tx.Prepare(query)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, msg := range messages {
		if msg.Content == "" && msg.MediaType == "" {
			skipped++
			continue
		}

		msg.CreatedAt = now
		msg.UpdatedAt = now

		if msg.Status == "" {
			msg.Status = "sent"
		}

		result, err := stmt.Exec(
			r.deviceID, msg.ID, msg.ChatJID, msg.Sender, msg.Content,
			msg.Timestamp, msg.IsFromMe, msg.MediaType, msg.Filename,
			msg.URL, msg.FileLength, msg.Status,
			msg.DeliveredAt, msg.ReadAt, msg.PlayedAt,
			msg.CreatedAt, msg.UpdatedAt,
		)
		if err != nil {
			return imported, skipped, fmt.Errorf("failed to store message %s: %w", msg.ID, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			imported++
		} else {
			skipped++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return imported, skipped, nil
}

// GetMessageByID retrieves a message by its ID
func (r *PostgresRepository) GetMessageByID(id string) (*domainChatStorage.Message, error) {
	query := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me,
			media_type, filename, url, media_key, file_sha256,
			file_enc_sha256, file_length, status, delivered_at, read_at,
			played_at, created_at, updated_at
		FROM messages
		WHERE device_id = $1 AND id = $2
		LIMIT 1
	`

	message, err := r.scanMessage(r.db.QueryRow(query, r.deviceID, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return message, err
}

// GetMessages retrieves messages with filtering
func (r *PostgresRepository) GetMessages(filter *domainChatStorage.MessageFilter) ([]*domainChatStorage.Message, error) {
	var conditions []string
	var args []any
	argNum := 1

	conditions = append(conditions, fmt.Sprintf("device_id = $%d", argNum))
	args = append(args, r.deviceID)
	argNum++

	conditions = append(conditions, fmt.Sprintf("chat_jid = $%d", argNum))
	args = append(args, filter.ChatJID)
	argNum++

	if filter.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argNum))
		args = append(args, *filter.StartTime)
		argNum++
	}

	if filter.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argNum))
		args = append(args, *filter.EndTime)
		argNum++
	}

	if filter.MediaOnly {
		conditions = append(conditions, "media_type != ''")
	}

	if filter.IsFromMe != nil {
		conditions = append(conditions, fmt.Sprintf("is_from_me = $%d", argNum))
		args = append(args, *filter.IsFromMe)
		argNum++
	}

	query := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me,
			media_type, filename, url, media_key, file_sha256,
			file_enc_sha256, file_length, status, delivered_at, read_at,
			played_at, created_at, updated_at
		FROM messages
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY timestamp DESC
	`

	if filter.Limit > 0 {
		if filter.Limit > 1000 {
			filter.Limit = 1000
		}
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filter.Limit)
		argNum++

		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argNum)
			args = append(args, filter.Offset)
		}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*domainChatStorage.Message
	var messageIDs []string
	for rows.Next() {
		message, err := r.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
		messageIDs = append(messageIDs, message.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetch reactions
	if len(messageIDs) > 0 {
		reactionsMap, err := r.GetReactionsForMessages(messageIDs, filter.ChatJID)
		if err != nil {
			logrus.Warnf("Failed to fetch reactions for messages: %v", err)
		} else {
			for _, msg := range messages {
				if reactions, ok := reactionsMap[msg.ID]; ok {
					msg.Reactions = reactions
				}
			}
		}
	}

	return messages, nil
}

// SearchMessages performs database-level search for messages
func (r *PostgresRepository) SearchMessages(chatJID, searchText string, limit int) ([]*domainChatStorage.Message, error) {
	if strings.TrimSpace(searchText) == "" {
		return []*domainChatStorage.Message{}, nil
	}

	query := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me,
			media_type, filename, url, media_key, file_sha256,
			file_enc_sha256, file_length, status, delivered_at, read_at,
			played_at, created_at, updated_at
		FROM messages
		WHERE device_id = $1 AND chat_jid = $2 AND LOWER(content) LIKE $3
		ORDER BY timestamp DESC
	`

	args := []any{r.deviceID, chatJID, "%" + strings.ToLower(searchText) + "%"}

	if limit > 0 {
		if limit > 1000 {
			limit = 1000
		}
		query += " LIMIT $4"
		args = append(args, limit)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}
	defer rows.Close()

	var messages []*domainChatStorage.Message
	for rows.Next() {
		message, err := r.scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, message)
	}

	return messages, rows.Err()
}

// DeleteMessage deletes a specific message
func (r *PostgresRepository) DeleteMessage(id, chatJID string) error {
	_, err := r.db.Exec("DELETE FROM messages WHERE device_id = $1 AND id = $2 AND chat_jid = $3", r.deviceID, id, chatJID)
	return err
}

// GetChatMessageCount returns the number of messages in a chat
func (r *PostgresRepository) GetChatMessageCount(chatJID string) (int64, error) {
	return r.getCount("SELECT COUNT(*) FROM messages WHERE device_id = $1 AND chat_jid = $2", r.deviceID, chatJID)
}

// GetTotalMessageCount returns the total number of messages
func (r *PostgresRepository) GetTotalMessageCount() (int64, error) {
	return r.getCount("SELECT COUNT(*) FROM messages WHERE device_id = $1", r.deviceID)
}

// CreateMessage creates a message from a WhatsApp event
func (r *PostgresRepository) CreateMessage(ctx context.Context, evt *events.Message) error {
	if evt == nil || evt.Message == nil {
		return nil
	}

	client := whatsapp.GetClient()

	normalizedChatJID := whatsapp.NormalizeJIDFromLID(ctx, evt.Info.Chat, client)
	normalizedSender := whatsapp.NormalizeJIDFromLID(ctx, evt.Info.Sender, client)

	if normalizedChatJID.Server == types.BroadcastServer {
		return nil
	}

	chatJID := normalizedChatJID.String()
	sender := normalizedSender.String()
	chatName := r.GetChatNameWithPushName(normalizedChatJID, chatJID, normalizedSender.User, evt.Info.PushName)

	existingChat, err := r.GetChat(chatJID)
	if err != nil {
		return fmt.Errorf("failed to get existing chat: %w", err)
	}

	ephemeralExpiration := utils.ExtractEphemeralExpiration(evt.Message)

	chat := &domainChatStorage.Chat{
		JID:             chatJID,
		Name:            chatName,
		LastMessageTime: evt.Info.Timestamp,
	}

	if ephemeralExpiration > 0 {
		chat.EphemeralExpiration = ephemeralExpiration
	} else if existingChat != nil {
		chat.EphemeralExpiration = existingChat.EphemeralExpiration
	}

	if err := r.StoreChat(chat); err != nil {
		return fmt.Errorf("failed to store chat: %w", err)
	}

	content := utils.ExtractMessageTextFromProto(evt.Message)
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := utils.ExtractMediaInfo(evt.Message)

	if content == "" && mediaType == "" {
		return nil
	}

	message := &domainChatStorage.Message{
		ID:            evt.Info.ID,
		ChatJID:       chatJID,
		Sender:        sender,
		Content:       content,
		Timestamp:     evt.Info.Timestamp,
		IsFromMe:      evt.Info.IsFromMe,
		MediaType:     mediaType,
		Filename:      filename,
		URL:           url,
		MediaKey:      mediaKey,
		FileSHA256:    fileSHA256,
		FileEncSHA256: fileEncSHA256,
		FileLength:    fileLength,
	}

	return r.StoreMessage(message)
}

// GetStorageStatistics returns current storage statistics
func (r *PostgresRepository) GetStorageStatistics() (chatCount int64, messageCount int64, err error) {
	chatCount, err = r.GetTotalChatCount()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get chat count: %w", err)
	}

	messageCount, err = r.GetTotalMessageCount()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get message count: %w", err)
	}

	return chatCount, messageCount, nil
}

// StoreSentMessageWithContext stores a message that was sent by the user
func (r *PostgresRepository) StoreSentMessageWithContext(ctx context.Context, messageID string, senderJID string, recipientJID string, content string, timestamp time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	jid, err := types.ParseJID(recipientJID)
	if err != nil {
		return fmt.Errorf("invalid JID format: %w", err)
	}

	client := whatsapp.GetClient()
	normalizedJID := whatsapp.NormalizeJIDFromLID(ctx, jid, client)
	chatJID := normalizedJID.String()
	chatName := r.GetChatNameWithPushName(normalizedJID, chatJID, normalizedJID.User, "")

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	existingChat, err := r.GetChat(chatJID)
	if err != nil {
		return fmt.Errorf("failed to get existing chat: %w", err)
	}

	chat := &domainChatStorage.Chat{
		JID:             chatJID,
		Name:            chatName,
		LastMessageTime: timestamp,
	}

	if existingChat != nil {
		chat.EphemeralExpiration = existingChat.EphemeralExpiration
	}

	if err := r.StoreChat(chat); err != nil {
		return fmt.Errorf("failed to store chat: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	message := &domainChatStorage.Message{
		ID:        messageID,
		ChatJID:   chatJID,
		Sender:    senderJID,
		Content:   content,
		Timestamp: timestamp,
		IsFromMe:  true,
	}

	return r.StoreMessage(message)
}

// UpdateMessageStatus updates the status of a message
func (r *PostgresRepository) UpdateMessageStatus(ctx context.Context, messageID string, status string, statusTime time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var query string
	switch status {
	case "delivered":
		query = `
			UPDATE messages
			SET status = $1, delivered_at = $2, updated_at = $3
			WHERE device_id = $4 AND id = $5 AND (status = 'sent' OR delivered_at IS NULL)
		`
	case "read":
		query = `
			UPDATE messages
			SET status = $1, read_at = $2, updated_at = $3
			WHERE device_id = $4 AND id = $5 AND (status IN ('sent', 'delivered') OR read_at IS NULL)
		`
	case "played":
		query = `
			UPDATE messages
			SET status = $1, played_at = $2, updated_at = $3
			WHERE device_id = $4 AND id = $5
		`
	default:
		return fmt.Errorf("invalid status: %s", status)
	}

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, status, statusTime, now, r.deviceID, messageID)
	return err
}

// ============================================================================
// Reaction Operations
// ============================================================================

// StoreReaction stores or updates a reaction
func (r *PostgresRepository) StoreReaction(ctx context.Context, reaction *domainChatStorage.MessageReaction) error {
	if reaction.Emoji == "" {
		// Empty emoji means remove reaction
		_, err := r.db.ExecContext(ctx, `
			DELETE FROM message_reactions
			WHERE device_id = $1 AND message_id = $2 AND chat_jid = $3 AND sender_jid = $4
		`, r.deviceID, reaction.MessageID, reaction.ChatJID, reaction.SenderJID)
		return err
	}

	query := `
		INSERT INTO message_reactions (device_id, message_id, chat_jid, sender_jid, emoji, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(device_id, message_id, chat_jid, sender_jid) DO UPDATE SET
			emoji = EXCLUDED.emoji,
			timestamp = EXCLUDED.timestamp
	`

	_, err := r.db.ExecContext(ctx, query, r.deviceID, reaction.MessageID, reaction.ChatJID, reaction.SenderJID, reaction.Emoji, reaction.Timestamp)
	return err
}

// GetReactionsForMessage returns all reactions for a message
func (r *PostgresRepository) GetReactionsForMessage(messageID, chatJID string) ([]domainChatStorage.MessageReaction, error) {
	query := `
		SELECT message_id, chat_jid, sender_jid, emoji, timestamp
		FROM message_reactions
		WHERE device_id = $1 AND message_id = $2 AND chat_jid = $3
		ORDER BY timestamp DESC
	`

	rows, err := r.db.Query(query, r.deviceID, messageID, chatJID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []domainChatStorage.MessageReaction
	for rows.Next() {
		var reaction domainChatStorage.MessageReaction
		if err := rows.Scan(&reaction.MessageID, &reaction.ChatJID, &reaction.SenderJID, &reaction.Emoji, &reaction.Timestamp); err != nil {
			return nil, err
		}
		reactions = append(reactions, reaction)
	}

	return reactions, rows.Err()
}

// GetReactionsForMessages returns reactions for multiple messages
func (r *PostgresRepository) GetReactionsForMessages(messageIDs []string, chatJID string) (map[string][]domainChatStorage.MessageReaction, error) {
	if len(messageIDs) == 0 {
		return map[string][]domainChatStorage.MessageReaction{}, nil
	}

	// Build parameterized query
	placeholders := make([]string, len(messageIDs))
	args := []any{r.deviceID, chatJID}
	for i, id := range messageIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		SELECT message_id, chat_jid, sender_jid, emoji, timestamp
		FROM message_reactions
		WHERE device_id = $1 AND chat_jid = $2 AND message_id IN (%s)
		ORDER BY timestamp DESC
	`, strings.Join(placeholders, ", "))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]domainChatStorage.MessageReaction)
	for rows.Next() {
		var reaction domainChatStorage.MessageReaction
		if err := rows.Scan(&reaction.MessageID, &reaction.ChatJID, &reaction.SenderJID, &reaction.Emoji, &reaction.Timestamp); err != nil {
			return nil, err
		}
		result[reaction.MessageID] = append(result[reaction.MessageID], reaction)
	}

	return result, rows.Err()
}

// ============================================================================
// Admin Operations
// ============================================================================

// DeleteChatsByPattern deletes chats matching a pattern
func (r *PostgresRepository) DeleteChatsByPattern(pattern string) (chatsDeleted, messagesDeleted int64, deletedJIDs []string, err error) {
	// Find matching chats
	rows, err := r.db.Query("SELECT jid FROM chats WHERE device_id = $1 AND jid LIKE $2", r.deviceID, pattern)
	if err != nil {
		return 0, 0, nil, err
	}

	for rows.Next() {
		var jid string
		if err := rows.Scan(&jid); err != nil {
			rows.Close()
			return 0, 0, nil, err
		}
		deletedJIDs = append(deletedJIDs, jid)
	}
	rows.Close()

	if len(deletedJIDs) == 0 {
		return 0, 0, nil, nil
	}

	// Delete in transaction
	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, jid := range deletedJIDs {
		result, _ := tx.Exec("DELETE FROM messages WHERE device_id = $1 AND chat_jid = $2", r.deviceID, jid)
		affected, _ := result.RowsAffected()
		messagesDeleted += affected

		_, _ = tx.Exec("DELETE FROM chats WHERE device_id = $1 AND jid = $2", r.deviceID, jid)
		chatsDeleted++
	}

	return chatsDeleted, messagesDeleted, deletedJIDs, tx.Commit()
}

// DeleteChatsByJIDs deletes multiple chats by their JIDs
func (r *PostgresRepository) DeleteChatsByJIDs(jids []string) (chatsDeleted, messagesDeleted int64, err error) {
	if len(jids) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, jid := range jids {
		result, _ := tx.Exec("DELETE FROM messages WHERE device_id = $1 AND chat_jid = $2", r.deviceID, jid)
		affected, _ := result.RowsAffected()
		messagesDeleted += affected

		result, _ = tx.Exec("DELETE FROM chats WHERE device_id = $1 AND jid = $2", r.deviceID, jid)
		affected, _ = result.RowsAffected()
		chatsDeleted += affected
	}

	return chatsDeleted, messagesDeleted, tx.Commit()
}

// GetDetailedStats returns detailed storage statistics
func (r *PostgresRepository) GetDetailedStats() (*domainChatStorage.DetailedStats, error) {
	stats := &domainChatStorage.DetailedStats{}

	var err error
	stats.TotalChats, err = r.GetTotalChatCount()
	if err != nil {
		return nil, err
	}

	stats.TotalMessages, err = r.GetTotalMessageCount()
	if err != nil {
		return nil, err
	}

	// Get database size (PostgreSQL specific)
	err = r.db.QueryRow("SELECT pg_database_size(current_database())").Scan(&stats.DatabaseSizeBytes)
	if err != nil {
		logrus.Warnf("Failed to get database size: %v", err)
	}

	stats.OldestMessage, stats.NewestMessage, _ = r.GetMessageDateRange()

	stats.EmptyChats, _ = r.CountEmptyChats()

	_ = r.db.QueryRow("SELECT COUNT(*) FROM messages WHERE device_id = $1 AND media_type != ''", r.deviceID).Scan(&stats.MediaMessages)
	_ = r.db.QueryRow("SELECT COUNT(*) FROM messages WHERE device_id = $1 AND media_type = ''", r.deviceID).Scan(&stats.TextMessages)

	return stats, nil
}

// VacuumDatabase runs PostgreSQL VACUUM ANALYZE
func (r *PostgresRepository) VacuumDatabase() error {
	_, err := r.db.Exec("VACUUM ANALYZE chats")
	if err != nil {
		return err
	}
	_, err = r.db.Exec("VACUUM ANALYZE messages")
	return err
}

// GetDatabaseSize returns the database size in bytes
func (r *PostgresRepository) GetDatabaseSize() (int64, error) {
	var size int64
	err := r.db.QueryRow("SELECT pg_database_size(current_database())").Scan(&size)
	return size, err
}

// GetMessageDateRange returns the oldest and newest message timestamps
func (r *PostgresRepository) GetMessageDateRange() (oldest, newest *time.Time, err error) {
	var oldestTime, newestTime sql.NullTime

	err = r.db.QueryRow("SELECT MIN(timestamp), MAX(timestamp) FROM messages WHERE device_id = $1", r.deviceID).Scan(&oldestTime, &newestTime)
	if err != nil {
		return nil, nil, err
	}

	if oldestTime.Valid {
		oldest = &oldestTime.Time
	}
	if newestTime.Valid {
		newest = &newestTime.Time
	}

	return oldest, newest, nil
}

// CountEmptyChats returns the number of chats with no messages
func (r *PostgresRepository) CountEmptyChats() (int64, error) {
	var count int64
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM chats c
		WHERE c.device_id = $1 AND NOT EXISTS (
			SELECT 1 FROM messages m WHERE m.device_id = c.device_id AND m.chat_jid = c.jid
		)
	`, r.deviceID).Scan(&count)
	return count, err
}

// DeleteEmptyChats removes chats with no messages
func (r *PostgresRepository) DeleteEmptyChats() (deleted int64, err error) {
	result, err := r.db.Exec(`
		DELETE FROM chats c
		WHERE c.device_id = $1 AND NOT EXISTS (
			SELECT 1 FROM messages m WHERE m.device_id = c.device_id AND m.chat_jid = c.jid
		)
	`, r.deviceID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteMessagesOlderThan removes messages older than the specified time
func (r *PostgresRepository) DeleteMessagesOlderThan(before time.Time) (deleted int64, err error) {
	result, err := r.db.Exec("DELETE FROM messages WHERE device_id = $1 AND timestamp < $2", r.deviceID, before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
