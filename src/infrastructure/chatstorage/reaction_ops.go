package chatstorage

import (
	"context"
	"fmt"
	"strings"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
)

// StoreReaction stores or updates a reaction (empty emoji = delete the reaction)
func (r *SQLiteRepository) StoreReaction(ctx context.Context, reaction *domainChatStorage.MessageReaction) error {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// If emoji is empty, delete the reaction
	if reaction.Emoji == "" {
		_, err := r.db.ExecContext(ctx,
			"DELETE FROM message_reactions WHERE message_id = ? AND chat_jid = ? AND sender_jid = ?",
			reaction.MessageID, reaction.ChatJID, reaction.SenderJID,
		)
		return err
	}

	// Upsert the reaction
	query := `
		INSERT INTO message_reactions (message_id, chat_jid, sender_jid, emoji, timestamp)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(message_id, chat_jid, sender_jid) DO UPDATE SET
			emoji = excluded.emoji,
			timestamp = excluded.timestamp
	`
	_, err := r.db.ExecContext(ctx, query,
		reaction.MessageID, reaction.ChatJID, reaction.SenderJID, reaction.Emoji, reaction.Timestamp,
	)
	return err
}

// GetReactionsForMessage returns all reactions for a specific message
func (r *SQLiteRepository) GetReactionsForMessage(messageID, chatJID string) ([]domainChatStorage.MessageReaction, error) {
	query := `
		SELECT message_id, chat_jid, sender_jid, emoji, timestamp
		FROM message_reactions
		WHERE message_id = ? AND chat_jid = ?
		ORDER BY timestamp ASC
	`
	rows, err := r.db.Query(query, messageID, chatJID)
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

// GetReactionsForMessages returns reactions for multiple messages (batch query)
func (r *SQLiteRepository) GetReactionsForMessages(messageIDs []string, chatJID string) (map[string][]domainChatStorage.MessageReaction, error) {
	if len(messageIDs) == 0 {
		return make(map[string][]domainChatStorage.MessageReaction), nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(messageIDs))
	args := make([]any, len(messageIDs)+1)
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	args[len(messageIDs)] = chatJID

	query := fmt.Sprintf(`
		SELECT message_id, chat_jid, sender_jid, emoji, timestamp
		FROM message_reactions
		WHERE message_id IN (%s) AND chat_jid = ?
		ORDER BY timestamp ASC
	`, strings.Join(placeholders, ","))

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
