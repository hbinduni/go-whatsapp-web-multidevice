package chatstorage

import (
	"fmt"
	"strings"
	"time"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types"
)

// StoreChat creates or updates a chat
func (r *SQLiteRepository) StoreChat(chat *domainChatStorage.Chat) error {
	now := time.Now()
	chat.UpdatedAt = now

	query := `
		INSERT INTO chats (jid, name, last_message_time, ephemeral_expiration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = excluded.name,
			last_message_time = excluded.last_message_time,
			ephemeral_expiration = excluded.ephemeral_expiration,
			updated_at = excluded.updated_at
	`

	_, err := r.db.Exec(query, chat.JID, chat.Name, chat.LastMessageTime, chat.EphemeralExpiration, now, chat.UpdatedAt)
	return err
}

// StoreChatsBatch inserts multiple chats in a single transaction
// If skipExisting is true, uses ON CONFLICT DO NOTHING (for merge imports)
// If skipExisting is false, uses ON CONFLICT DO UPDATE (upsert behavior)
func (r *SQLiteRepository) StoreChatsBatch(chats []*domainChatStorage.Chat, skipExisting bool) (imported, skipped int64, err error) {
	if len(chats) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Choose query based on skipExisting mode
	var query string
	if skipExisting {
		// For merge mode: skip existing records
		query = `
			INSERT INTO chats (jid, name, last_message_time, ephemeral_expiration, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(jid) DO NOTHING
		`
	} else {
		// For overwrite mode: update existing records
		query = `
			INSERT INTO chats (jid, name, last_message_time, ephemeral_expiration, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(jid) DO UPDATE SET
				name = excluded.name,
				last_message_time = excluded.last_message_time,
				ephemeral_expiration = excluded.ephemeral_expiration,
				updated_at = excluded.updated_at
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
			chat.JID, chat.Name, chat.LastMessageTime,
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
func (r *SQLiteRepository) GetChat(jid string) (*domainChatStorage.Chat, error) {
	query := `
		SELECT jid, name, last_message_time, ephemeral_expiration, created_at, updated_at
		FROM chats
		WHERE jid = ?
	`

	chat, err := r.scanChat(r.db.QueryRow(query, jid))
	if err == ErrNoRows {
		return nil, nil
	}

	return chat, err
}

// GetChats retrieves chats with filtering and includes last message preview
func (r *SQLiteRepository) GetChats(filter *domainChatStorage.ChatFilter) ([]*domainChatStorage.Chat, error) {
	var conditions []string
	var args []any

	// Query with LEFT JOIN to get last message for each chat using a subquery
	// This efficiently fetches the most recent message per chat
	query := `
		SELECT
			c.jid, c.name, c.last_message_time, c.ephemeral_expiration, c.created_at, c.updated_at,
			lm.content AS last_message,
			lm.is_from_me AS last_message_from_me,
			lm.media_type AS last_message_type
		FROM chats c
		LEFT JOIN (
			SELECT m1.chat_jid, m1.content, m1.is_from_me, m1.media_type
			FROM messages m1
			INNER JOIN (
				SELECT chat_jid, MAX(timestamp) as max_ts
				FROM messages
				GROUP BY chat_jid
			) m2 ON m1.chat_jid = m2.chat_jid AND m1.timestamp = m2.max_ts
		) lm ON c.jid = lm.chat_jid
	`

	if filter.SearchName != "" {
		conditions = append(conditions, "c.name LIKE ?")
		args = append(args, "%"+filter.SearchName+"%")
	}

	if filter.HasMedia {
		query = `
			SELECT
				c.jid, c.name, c.last_message_time, c.ephemeral_expiration, c.created_at, c.updated_at,
				lm.content AS last_message,
				lm.is_from_me AS last_message_from_me,
				lm.media_type AS last_message_type
			FROM chats c
			INNER JOIN messages m ON c.jid = m.chat_jid
			LEFT JOIN (
				SELECT m1.chat_jid, m1.content, m1.is_from_me, m1.media_type
				FROM messages m1
				INNER JOIN (
					SELECT chat_jid, MAX(timestamp) as max_ts
					FROM messages
					GROUP BY chat_jid
				) m2 ON m1.chat_jid = m2.chat_jid AND m1.timestamp = m2.max_ts
			) lm ON c.jid = lm.chat_jid
		`
		conditions = append(conditions, "m.media_type != ''")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY c.last_message_time DESC"

	// Safely add LIMIT and OFFSET using parameterized values
	if filter.Limit > 0 {
		// Validate limit to prevent abuse
		if filter.Limit > 1000 {
			filter.Limit = 1000
		}
		query += " LIMIT ?"
		args = append(args, filter.Limit)

		if filter.Offset > 0 {
			query += " OFFSET ?"
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
func (r *SQLiteRepository) DeleteChat(jid string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Delete messages first (foreign key constraint)
	_, err = tx.Exec("DELETE FROM messages WHERE chat_jid = ?", jid)
	if err != nil {
		return err
	}

	// Delete chat
	_, err = tx.Exec("DELETE FROM chats WHERE jid = ?", jid)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetTotalChatCount returns the total number of chats
func (r *SQLiteRepository) GetTotalChatCount() (int64, error) {
	return r.getCount("SELECT COUNT(*) FROM chats")
}

// TruncateAllChats deletes all chats from the database
// Note: Due to foreign key constraints, messages must be deleted first
func (r *SQLiteRepository) TruncateAllChats() error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete reactions first
	_, err = tx.Exec("DELETE FROM message_reactions")
	if err != nil {
		// Table might not exist in older schemas, ignore error
		logrus.Debugf("Failed to delete message_reactions (table may not exist): %v", err)
	}

	// Delete messages (foreign key constraint)
	_, err = tx.Exec("DELETE FROM messages")
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	// Delete chats
	_, err = tx.Exec("DELETE FROM chats")
	if err != nil {
		return fmt.Errorf("failed to delete chats: %w", err)
	}

	return tx.Commit()
}

// GetChatNameWithPushName determines the appropriate name for a chat with pushname support
func (r *SQLiteRepository) GetChatNameWithPushName(jid types.JID, chatJID string, senderUser string, pushName string) string {
	// First, check if chat already exists with a name
	existingChat, err := r.GetChat(chatJID)
	if err == nil && existingChat != nil && existingChat.Name != "" {
		// If we have a pushname and the existing name is just a phone number/JID user, update it
		if pushName != "" && (existingChat.Name == jid.User || existingChat.Name == senderUser) {
			return pushName
		}
		return existingChat.Name
	}

	// Determine chat type and name
	var name string

	switch jid.Server {
	case "g.us":
		// This is a group chat
		// For now, use a generic name - this can be enhanced later with group info
		name = fmt.Sprintf("Group %s", jid.User)
	case "newsletter":
		// This is a newsletter/channel
		name = fmt.Sprintf("Newsletter %s", jid.User)
	default:
		// This is an individual contact
		// Priority: pushName > senderUser > JID user
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
func (r *SQLiteRepository) TruncateAllDataWithLogging(logPrefix string) error {
	// Get statistics before truncation
	chatCount, messageCount, err := r.GetStorageStatistics()
	if err != nil {
		logrus.Warnf("[%s] Failed to get storage statistics before truncation: %v", logPrefix, err)
	} else {
		logrus.Infof("[%s] Storage before truncation: %d chats, %d messages", logPrefix, chatCount, messageCount)
	}

	// Perform truncation
	if err := r.TruncateAllChats(); err != nil {
		return fmt.Errorf("failed to truncate chatstorage data: %w", err)
	}

	// Verify truncation
	chatCountAfter, messageCountAfter, err := r.GetStorageStatistics()
	if err != nil {
		logrus.Warnf("[%s] Failed to get storage statistics after truncation: %v", logPrefix, err)
	} else {
		logrus.Infof("[%s] Storage after truncation: %d chats, %d messages", logPrefix, chatCountAfter, messageCountAfter)
		if chatCountAfter == 0 && messageCountAfter == 0 {
			logrus.Infof("[%s] ✅ Chatstorage truncation completed successfully", logPrefix)
		} else {
			logrus.Warnf("[%s] ⚠️ Truncation may not have completed fully", logPrefix)
		}
	}

	return nil
}
