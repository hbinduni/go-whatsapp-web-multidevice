package chatstorage

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/sirupsen/logrus"
)

// DeleteChatsByPattern deletes all chats matching a SQL LIKE pattern
func (r *SQLiteRepository) DeleteChatsByPattern(pattern string) (chatsDeleted, messagesDeleted int64, deletedJIDs []string, err error) {
	if pattern == "" {
		return 0, 0, nil, fmt.Errorf("pattern cannot be empty")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// First, get the JIDs that will be deleted
	rows, err := tx.Query("SELECT jid FROM chats WHERE jid LIKE ?", pattern)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to query chats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var jid string
		if err := rows.Scan(&jid); err != nil {
			return 0, 0, nil, fmt.Errorf("failed to scan jid: %w", err)
		}
		deletedJIDs = append(deletedJIDs, jid)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if len(deletedJIDs) == 0 {
		return 0, 0, nil, nil // No chats matched
	}

	// Delete reactions for messages in matching chats
	_, _ = tx.Exec("DELETE FROM message_reactions WHERE chat_jid LIKE ?", pattern)

	// Count and delete messages
	msgResult, err := tx.Exec("DELETE FROM messages WHERE chat_jid LIKE ?", pattern)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to delete messages: %w", err)
	}
	messagesDeleted, _ = msgResult.RowsAffected()

	// Delete chats
	chatResult, err := tx.Exec("DELETE FROM chats WHERE jid LIKE ?", pattern)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to delete chats: %w", err)
	}
	chatsDeleted, _ = chatResult.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, 0, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logrus.Infof("[Admin] Deleted %d chats and %d messages matching pattern '%s'", chatsDeleted, messagesDeleted, pattern)
	return chatsDeleted, messagesDeleted, deletedJIDs, nil
}

// DeleteChatsByJIDs deletes specific chats by their JIDs
func (r *SQLiteRepository) DeleteChatsByJIDs(jids []string) (chatsDeleted, messagesDeleted int64, err error) {
	if len(jids) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, jid := range jids {
		// Delete reactions
		_, _ = tx.Exec("DELETE FROM message_reactions WHERE chat_jid = ?", jid)

		// Delete messages
		msgResult, err := tx.Exec("DELETE FROM messages WHERE chat_jid = ?", jid)
		if err != nil {
			logrus.Warnf("Failed to delete messages for chat %s: %v", jid, err)
			continue
		}
		affected, _ := msgResult.RowsAffected()
		messagesDeleted += affected

		// Delete chat
		chatResult, err := tx.Exec("DELETE FROM chats WHERE jid = ?", jid)
		if err != nil {
			logrus.Warnf("Failed to delete chat %s: %v", jid, err)
			continue
		}
		affected, _ = chatResult.RowsAffected()
		chatsDeleted += affected
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logrus.Infof("[Admin] Deleted %d chats and %d messages for %d JIDs", chatsDeleted, messagesDeleted, len(jids))
	return chatsDeleted, messagesDeleted, nil
}

// GetDetailedStats returns comprehensive storage statistics
func (r *SQLiteRepository) GetDetailedStats() (*domainChatStorage.DetailedStats, error) {
	stats := &domainChatStorage.DetailedStats{}

	// Get total counts
	var err error
	stats.TotalChats, err = r.GetTotalChatCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get chat count: %w", err)
	}

	stats.TotalMessages, err = r.GetTotalMessageCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get message count: %w", err)
	}

	// Get database size
	stats.DatabaseSizeBytes, err = r.GetDatabaseSize()
	if err != nil {
		logrus.Warnf("Failed to get database size: %v", err)
	}

	// Get message date range
	oldest, newest, err := r.GetMessageDateRange()
	if err != nil {
		logrus.Warnf("Failed to get message date range: %v", err)
	}
	stats.OldestMessage = oldest
	stats.NewestMessage = newest

	// Count empty chats
	stats.EmptyChats, err = r.CountEmptyChats()
	if err != nil {
		logrus.Warnf("Failed to count empty chats: %v", err)
	}

	// Count media vs text messages
	stats.MediaMessages, _ = r.getCount("SELECT COUNT(*) FROM messages WHERE media_type != ''")
	stats.TextMessages, _ = r.getCount("SELECT COUNT(*) FROM messages WHERE media_type = '' AND content != ''")

	return stats, nil
}

// VacuumDatabase runs SQLite VACUUM to optimize and reclaim space
func (r *SQLiteRepository) VacuumDatabase() error {
	logrus.Info("[Admin] Starting database VACUUM operation")
	_, err := r.db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	logrus.Info("[Admin] Database VACUUM completed successfully")
	return nil
}

// GetDatabaseSize returns the database file size in bytes
func (r *SQLiteRepository) GetDatabaseSize() (int64, error) {
	dbPath := config.ChatStorageURI

	// Handle file: prefix in URI
	if strings.HasPrefix(dbPath, "file:") {
		dbPath = strings.TrimPrefix(dbPath, "file:")
		// Remove any query parameters
		if idx := strings.Index(dbPath, "?"); idx != -1 {
			dbPath = dbPath[:idx]
		}
	}

	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat database file: %w", err)
	}

	// Also check for WAL and SHM files
	totalSize := fileInfo.Size()

	if walInfo, err := os.Stat(dbPath + "-wal"); err == nil {
		totalSize += walInfo.Size()
	}
	if shmInfo, err := os.Stat(dbPath + "-shm"); err == nil {
		totalSize += shmInfo.Size()
	}

	return totalSize, nil
}

// GetMessageDateRange returns the oldest and newest message timestamps
func (r *SQLiteRepository) GetMessageDateRange() (oldest, newest *time.Time, err error) {
	var oldestTime, newestTime time.Time

	err = r.db.QueryRow("SELECT MIN(timestamp) FROM messages").Scan(&oldestTime)
	if err == nil && !oldestTime.IsZero() {
		oldest = &oldestTime
	}

	err = r.db.QueryRow("SELECT MAX(timestamp) FROM messages").Scan(&newestTime)
	if err == nil && !newestTime.IsZero() {
		newest = &newestTime
	}

	return oldest, newest, nil
}

// CountEmptyChats counts chats that have no messages
func (r *SQLiteRepository) CountEmptyChats() (int64, error) {
	return r.getCount(`
		SELECT COUNT(*) FROM chats c
		WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.chat_jid = c.jid)
	`)
}

// DeleteEmptyChats deletes chats that have no messages
func (r *SQLiteRepository) DeleteEmptyChats() (deleted int64, err error) {
	result, err := r.db.Exec(`
		DELETE FROM chats WHERE jid IN (
			SELECT c.jid FROM chats c
			WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.chat_jid = c.jid)
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to delete empty chats: %w", err)
	}
	deleted, _ = result.RowsAffected()
	logrus.Infof("[Admin] Deleted %d empty chats", deleted)
	return deleted, nil
}

// DeleteMessagesOlderThan deletes messages older than the specified time
func (r *SQLiteRepository) DeleteMessagesOlderThan(before time.Time) (deleted int64, err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete reactions for old messages first
	_, _ = tx.Exec("DELETE FROM message_reactions WHERE message_id IN (SELECT id FROM messages WHERE timestamp < ?)", before)

	// Delete old messages
	result, err := tx.Exec("DELETE FROM messages WHERE timestamp < ?", before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old messages: %w", err)
	}
	deleted, _ = result.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logrus.Infof("[Admin] Deleted %d messages older than %s", deleted, before.Format(time.RFC3339))
	return deleted, nil
}
