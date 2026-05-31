package chatstorage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/storage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// StoreMessage creates or updates a message
func (r *SQLiteRepository) StoreMessage(message *domainChatStorage.Message) error {
	now := time.Now()
	message.CreatedAt = now
	message.UpdatedAt = now

	// Set default status if not already set
	if message.Status == "" {
		message.Status = "sent"
	}

	// Skip empty messages
	if message.Content == "" && message.MediaType == "" {
		// This is not an error, just skip storing empty messages
		return nil
	}

	query := `
		INSERT INTO messages (
			id, chat_jid, sender, content, timestamp, is_from_me,
			media_type, filename, url, media_key, file_sha256,
			file_enc_sha256, file_length, status, delivered_at, read_at,
			played_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, chat_jid) DO UPDATE SET
			sender = excluded.sender,
			content = excluded.content,
			timestamp = excluded.timestamp,
			is_from_me = excluded.is_from_me,
			media_type = excluded.media_type,
			filename = excluded.filename,
			url = excluded.url,
			media_key = excluded.media_key,
			file_sha256 = excluded.file_sha256,
			file_enc_sha256 = excluded.file_enc_sha256,
			file_length = excluded.file_length,
			updated_at = excluded.updated_at
	`

	_, err := r.db.Exec(query,
		message.ID, message.ChatJID, message.Sender, message.Content,
		message.Timestamp, message.IsFromMe, message.MediaType, message.Filename,
		message.URL, message.MediaKey, message.FileSHA256, message.FileEncSHA256,
		message.FileLength, message.Status, message.DeliveredAt, message.ReadAt,
		message.PlayedAt, message.CreatedAt, message.UpdatedAt,
	)

	return err
}

// StoreMessagesBatch creates or updates multiple messages in a single transaction
func (r *SQLiteRepository) StoreMessagesBatch(messages []*domainChatStorage.Message) error {
	if len(messages) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Prepare the statement once for better performance
	stmt, err := tx.Prepare(`
		INSERT INTO messages (
			id, chat_jid, sender, content, timestamp, is_from_me,
			media_type, filename, url, media_key, file_sha256,
			file_enc_sha256, file_length, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, chat_jid) DO UPDATE SET
			sender = excluded.sender,
			content = excluded.content,
			timestamp = excluded.timestamp,
			is_from_me = excluded.is_from_me,
			media_type = excluded.media_type,
			filename = excluded.filename,
			url = excluded.url,
			media_key = excluded.media_key,
			file_sha256 = excluded.file_sha256,
			file_enc_sha256 = excluded.file_enc_sha256,
			file_length = excluded.file_length,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, message := range messages {
		// Skip empty messages
		if message.Content == "" && message.MediaType == "" {
			continue
		}

		message.CreatedAt = now
		message.UpdatedAt = now

		_, err = stmt.Exec(
			message.ID, message.ChatJID, message.Sender, message.Content,
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

// ImportMessagesBatch inserts multiple messages in a single transaction for imports
// If skipExisting is true, uses ON CONFLICT DO NOTHING (for merge imports)
// If skipExisting is false, uses ON CONFLICT DO UPDATE (upsert behavior)
// Returns the count of imported and skipped messages
func (r *SQLiteRepository) ImportMessagesBatch(messages []*domainChatStorage.Message, skipExisting bool) (imported, skipped int64, err error) {
	if len(messages) == 0 {
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
			INSERT INTO messages (
				id, chat_jid, sender, content, timestamp, is_from_me,
				media_type, filename, url, file_length, status,
				delivered_at, read_at, played_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id, chat_jid) DO NOTHING
		`
	} else {
		// For overwrite mode: update existing records
		query = `
			INSERT INTO messages (
				id, chat_jid, sender, content, timestamp, is_from_me,
				media_type, filename, url, file_length, status,
				delivered_at, read_at, played_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id, chat_jid) DO UPDATE SET
				sender = excluded.sender,
				content = excluded.content,
				timestamp = excluded.timestamp,
				is_from_me = excluded.is_from_me,
				media_type = excluded.media_type,
				filename = excluded.filename,
				url = excluded.url,
				file_length = excluded.file_length,
				status = excluded.status,
				delivered_at = excluded.delivered_at,
				read_at = excluded.read_at,
				played_at = excluded.played_at,
				updated_at = excluded.updated_at
		`
	}

	stmt, err := tx.Prepare(query)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, msg := range messages {
		// Skip empty messages
		if msg.Content == "" && msg.MediaType == "" {
			skipped++
			continue
		}

		msg.CreatedAt = now
		msg.UpdatedAt = now

		// Set default status if not set
		if msg.Status == "" {
			msg.Status = "sent"
		}

		result, err := stmt.Exec(
			msg.ID, msg.ChatJID, msg.Sender, msg.Content,
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

// GetMessageByID retrieves a message by its ID from any chat
// This is more efficient than searching through all chats
func (r *SQLiteRepository) GetMessageByID(id string) (*domainChatStorage.Message, error) {
	query := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me,
			media_type, filename, url, media_key, file_sha256,
			file_enc_sha256, file_length, status, delivered_at, read_at,
			played_at, created_at, updated_at
		FROM messages
		WHERE id = ?
		LIMIT 1
	`

	message, err := r.scanMessage(r.db.QueryRow(query, id))
	if err == ErrNoRows {
		return nil, nil
	}

	return message, err
}

// GetMessages retrieves messages with filtering and includes reactions
func (r *SQLiteRepository) GetMessages(filter *domainChatStorage.MessageFilter) ([]*domainChatStorage.Message, error) {
	var conditions []string
	var args []any

	conditions = append(conditions, "chat_jid = ?")
	args = append(args, filter.ChatJID)

	if filter.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *filter.StartTime)
	}

	if filter.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, *filter.EndTime)
	}

	if filter.MediaOnly {
		conditions = append(conditions, "media_type != ''")
	}

	if filter.IsFromMe != nil {
		conditions = append(conditions, "is_from_me = ?")
		args = append(args, *filter.IsFromMe)
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

	// Fetch reactions for all messages in a single batch query
	if len(messageIDs) > 0 {
		reactionsMap, err := r.GetReactionsForMessages(messageIDs, filter.ChatJID)
		if err != nil {
			// Log error but don't fail - reactions are optional
			logrus.Warnf("Failed to fetch reactions for messages: %v", err)
		} else {
			// Attach reactions to messages
			for _, msg := range messages {
				if reactions, ok := reactionsMap[msg.ID]; ok {
					msg.Reactions = reactions
				}
			}
		}
	}

	return messages, nil
}

// SearchMessages performs database-level search for messages containing specific text
func (r *SQLiteRepository) SearchMessages(chatJID, searchText string, limit int) ([]*domainChatStorage.Message, error) {
	// Return empty results for empty search text
	if strings.TrimSpace(searchText) == "" {
		return []*domainChatStorage.Message{}, nil
	}

	var conditions []string
	var args []any

	// Always filter by chat JID
	conditions = append(conditions, "chat_jid = ?")
	args = append(args, chatJID)

	// Add search condition using LIKE operator for case-insensitive search
	conditions = append(conditions, "LOWER(content) LIKE ?")
	args = append(args, "%"+strings.ToLower(searchText)+"%")

	query := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me,
			media_type, filename, url, media_key, file_sha256,
			file_enc_sha256, file_length, status, delivered_at, read_at,
			played_at, created_at, updated_at
		FROM messages
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY timestamp DESC
	`

	// Add limit with validation
	if limit > 0 {
		// Validate limit to prevent abuse
		if limit > 1000 {
			limit = 1000
		}
		query += " LIMIT ?"
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// DeleteMessage deletes a specific message
func (r *SQLiteRepository) DeleteMessage(id, chatJID string) error {
	_, err := r.db.Exec("DELETE FROM messages WHERE id = ? AND chat_jid = ?", id, chatJID)
	return err
}

// GetChatMessageCount returns the number of messages in a chat
func (r *SQLiteRepository) GetChatMessageCount(chatJID string) (int64, error) {
	return r.getCount("SELECT COUNT(*) FROM messages WHERE chat_jid = ?", chatJID)
}

// GetTotalMessageCount returns the total number of messages
func (r *SQLiteRepository) GetTotalMessageCount() (int64, error) {
	return r.getCount("SELECT COUNT(*) FROM messages")
}

// CreateMessage creates a message from a WhatsApp event
func (r *SQLiteRepository) CreateMessage(ctx context.Context, evt *events.Message) error {
	if evt == nil || evt.Message == nil {
		return nil
	}

	// Get WhatsApp client for LID resolution
	client := whatsapp.GetClient()

	// Normalize chat and sender JIDs (convert @lid to @s.whatsapp.net)
	normalizedChatJID := whatsapp.NormalizeJIDFromLID(ctx, evt.Info.Chat, client)
	normalizedSender := whatsapp.NormalizeJIDFromLID(ctx, evt.Info.Sender, client)

	// Skip broadcast JIDs (status@broadcast) - these are WhatsApp Status/Stories
	// and should not appear as regular chats in the chat list
	if normalizedChatJID.Server == types.BroadcastServer {
		logrus.Debugf("Skipping broadcast message from %s (chat: %s)", normalizedSender.String(), normalizedChatJID.String())
		return nil
	}

	chatJID := normalizedChatJID.String()
	// Store the full sender JID (user@server) to ensure consistency between received and sent messages
	sender := normalizedSender.String()

	// Get appropriate chat name using pushname if available
	chatName := r.GetChatNameWithPushName(normalizedChatJID, chatJID, normalizedSender.User, evt.Info.PushName)

	// Get existing chat to preserve ephemeral_expiration if needed
	existingChat, err := r.GetChat(chatJID)
	if err != nil {
		return fmt.Errorf("failed to get existing chat: %w", err)
	}

	// Extract ephemeral expiration from incoming message
	ephemeralExpiration := utils.ExtractEphemeralExpiration(evt.Message)

	// Create or update chat
	chat := &domainChatStorage.Chat{
		JID:             chatJID,
		Name:            chatName,
		LastMessageTime: evt.Info.Timestamp,
	}

	// Set ephemeral expiration: use incoming message value if > 0, otherwise preserve existing
	if ephemeralExpiration > 0 {
		chat.EphemeralExpiration = ephemeralExpiration
	} else if existingChat != nil {
		// Preserve existing ephemeral_expiration if incoming message doesn't have one
		chat.EphemeralExpiration = existingChat.EphemeralExpiration
	}

	// Store or update the chat
	if err := r.StoreChat(chat); err != nil {
		return fmt.Errorf("failed to store chat: %w", err)
	}

	// Extract message content and media info
	content := utils.ExtractMessageTextFromProto(evt.Message)
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := utils.ExtractMediaInfo(evt.Message)

	// Skip if there's no content and no media
	if content == "" && mediaType == "" {
		logrus.Debugf("Skipping message %s - no content or media", evt.Info.ID)
		return nil
	}

	// Create message object
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

	// Store the message
	return r.StoreMessage(message)
}

// GetStorageStatistics returns current storage statistics for logging purposes
func (r *SQLiteRepository) GetStorageStatistics() (chatCount int64, messageCount int64, err error) {
	// Count all chats using efficient query
	chatCount, err = r.GetTotalChatCount()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get chat count: %w", err)
	}

	// Count all messages
	messageCount, err = r.GetTotalMessageCount()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get message count: %w", err)
	}

	return chatCount, messageCount, nil
}

// StoreSentMessageWithContext stores a message that was sent by the user with context cancellation support
func (r *SQLiteRepository) StoreSentMessageWithContext(ctx context.Context, messageID string, senderJID string, recipientJID string, content string, timestamp time.Time) error {
	// Check if context is already cancelled before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Ensure JID is properly formatted
	jid, err := types.ParseJID(recipientJID)
	if err != nil {
		return fmt.Errorf("invalid JID format: %w", err)
	}

	// Get WhatsApp client for LID resolution
	client := whatsapp.GetClient()

	// Normalize recipient JID (convert @lid to @s.whatsapp.net)
	normalizedJID := whatsapp.NormalizeJIDFromLID(ctx, jid, client)
	chatJID := normalizedJID.String()

	// Get chat name (no pushname available for sent messages)
	chatName := r.GetChatNameWithPushName(normalizedJID, chatJID, normalizedJID.User, "")

	// Check context again before database operations
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Get existing chat to preserve ephemeral_expiration
	existingChat, err := r.GetChat(chatJID)
	if err != nil {
		return fmt.Errorf("failed to get existing chat: %w", err)
	}

	// Store or update chat, preserving existing ephemeral_expiration
	chat := &domainChatStorage.Chat{
		JID:             chatJID,
		Name:            chatName,
		LastMessageTime: timestamp,
	}

	// Preserve existing ephemeral_expiration if chat exists
	if existingChat != nil {
		chat.EphemeralExpiration = existingChat.EphemeralExpiration
	}

	if err := r.StoreChat(chat); err != nil {
		return fmt.Errorf("failed to store chat: %w", err)
	}

	// Check context one more time before storing message
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Store the sent message
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

// StoreSentMediaMessageWithContext stores a media message sent by the user. It uploads the
// media bytes to S3 using the same object key scheme as received media (deviceID/chatJID/messageID)
// so the chat-messages endpoint can reconstruct the URL via ConstructMediaURL, and records the
// media metadata. If the upload fails (or storage/auto-download is disabled) it degrades to
// storing the message without media metadata - never a broken media reference.
func (r *SQLiteRepository) StoreSentMediaMessageWithContext(ctx context.Context, messageID string, senderJID string, recipientJID string, content string, mediaType string, filename string, mediaData []byte, timestamp time.Time) error {
	// Check if context is already cancelled before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Ensure JID is properly formatted
	jid, err := types.ParseJID(recipientJID)
	if err != nil {
		return fmt.Errorf("invalid JID format: %w", err)
	}

	// Get WhatsApp client for LID resolution
	client := whatsapp.GetClient()

	// Normalize recipient JID (convert @lid to @s.whatsapp.net). This MUST match the chatJID
	// used for the S3 key so ConstructMediaURL (which reads message.ChatJID) resolves correctly.
	normalizedJID := whatsapp.NormalizeJIDFromLID(ctx, jid, client)
	chatJID := normalizedJID.String()

	// Get chat name (no pushname available for sent messages)
	chatName := r.GetChatNameWithPushName(normalizedJID, chatJID, normalizedJID.User, "")

	// Get existing chat to preserve ephemeral_expiration
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

	// Upload media bytes to S3 with the same key scheme used for received media. Only set the
	// media metadata on the stored message when the upload succeeds, so a failed upload degrades
	// gracefully to the text indicator instead of a dead media link.
	storedMediaType := ""
	storedFilename := ""
	if config.WhatsappAutoDownloadMedia && storage.IsStorageInitialized() && len(mediaData) > 0 {
		var deviceID string
		if client != nil && client.Store != nil && client.Store.ID != nil {
			deviceID = client.Store.ID.User
		}
		key := storage.BuildMediaObjectKey(deviceID, chatJID, messageID)
		if key == "" {
			logrus.Warnf("Skipping sent media upload for %s: could not build object key", messageID)
		} else if mediaStorage := storage.GetStorage(); mediaStorage != nil {
			if _, saveErr := mediaStorage.Save(ctx, mediaData, key); saveErr != nil {
				logrus.Warnf("Failed to upload sent media to storage for %s: %v", messageID, saveErr)
			} else {
				storedMediaType = mediaType
				storedFilename = filename
			}
		}
	}

	// Store the sent message
	message := &domainChatStorage.Message{
		ID:        messageID,
		ChatJID:   chatJID,
		Sender:    senderJID,
		Content:   content,
		Timestamp: timestamp,
		IsFromMe:  true,
		MediaType: storedMediaType,
		Filename:  storedFilename,
	}

	return r.StoreMessage(message)
}

// UpdateMessageStatus updates the status of a message and records the timestamp
func (r *SQLiteRepository) UpdateMessageStatus(ctx context.Context, messageID string, status string, statusTime time.Time) error {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Determine which timestamp column to update based on status
	var query string
	switch status {
	case "delivered":
		query = `
			UPDATE messages
			SET status = ?, delivered_at = ?, updated_at = ?
			WHERE id = ? AND (status = 'sent' OR delivered_at IS NULL)
		`
	case "read":
		query = `
			UPDATE messages
			SET status = ?, read_at = ?, updated_at = ?
			WHERE id = ? AND (status IN ('sent', 'delivered') OR read_at IS NULL)
		`
	case "played":
		query = `
			UPDATE messages
			SET status = ?, played_at = ?, updated_at = ?
			WHERE id = ?
		`
	default:
		return fmt.Errorf("invalid status: %s (valid: delivered, read, played)", status)
	}

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, status, statusTime, now, messageID)
	if err != nil {
		return fmt.Errorf("failed to update message status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		logrus.Debugf("No rows updated for message %s with status %s (message may not exist or status already set)", messageID, status)
	}

	return nil
}
