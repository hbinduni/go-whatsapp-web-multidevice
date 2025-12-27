package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChat "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chat"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/storage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/appstate"

	_ "github.com/mattn/go-sqlite3"
)

type serviceChat struct {
	chatStorageRepo domainChatStorage.IChatStorageRepository
}

func NewChatService(chatStorageRepo domainChatStorage.IChatStorageRepository) domainChat.IChatUsecase {
	return &serviceChat{
		chatStorageRepo: chatStorageRepo,
	}
}

func (service serviceChat) ListChats(ctx context.Context, request domainChat.ListChatsRequest) (response domainChat.ListChatsResponse, err error) {
	if err = validations.ValidateListChats(ctx, &request); err != nil {
		return response, err
	}

	// Create filter from request
	filter := &domainChatStorage.ChatFilter{
		Limit:      request.Limit,
		Offset:     request.Offset,
		SearchName: request.Search,
		HasMedia:   request.HasMedia,
	}

	// Get chats from storage
	chats, err := service.chatStorageRepo.GetChats(filter)
	if err != nil {
		logrus.WithError(err).Error("Failed to get chats from storage")
		return response, err
	}

	// Get total count for pagination
	totalCount, err := service.chatStorageRepo.GetTotalChatCount()
	if err != nil {
		logrus.WithError(err).Error("Failed to get total chat count")
		// Continue with partial data
		totalCount = 0
	}

	// Convert entities to domain objects with JID normalization and deduplication
	// Use a map to deduplicate chats after normalization (same contact may have @lid and @s.whatsapp.net entries)
	seenJIDs := make(map[string]bool)
	chatInfos := make([]domainChat.ChatInfo, 0, len(chats))
	for _, chat := range chats {
		// Normalize JID from @lid to @s.whatsapp.net if possible
		normalizedJID := whatsapp.NormalizeJIDString(ctx, chat.JID)

		// Skip duplicates (keep the first one which has most recent last_message_time due to ORDER BY)
		if seenJIDs[normalizedJID] {
			logrus.WithFields(logrus.Fields{
				"original_jid":   chat.JID,
				"normalized_jid": normalizedJID,
			}).Debug("Skipping duplicate chat after JID normalization")
			continue
		}
		seenJIDs[normalizedJID] = true

		chatInfo := domainChat.ChatInfo{
			JID:                 normalizedJID,
			Name:                chat.Name,
			LastMessageTime:     chat.LastMessageTime.Format(time.RFC3339),
			EphemeralExpiration: chat.EphemeralExpiration,
			CreatedAt:           chat.CreatedAt.Format(time.RFC3339),
			UpdatedAt:           chat.UpdatedAt.Format(time.RFC3339),
			// Last message preview fields
			LastMessage:       chat.LastMessage,
			LastMessageFromMe: chat.LastMessageFromMe,
			LastMessageType:   chat.LastMessageType,
			UnreadCount:       chat.UnreadCount,
		}
		chatInfos = append(chatInfos, chatInfo)
	}

	// Create pagination response
	pagination := domainChat.PaginationResponse{
		Limit:  request.Limit,
		Offset: request.Offset,
		Total:  int(totalCount),
	}

	response.Data = chatInfos
	response.Pagination = pagination

	logrus.WithFields(logrus.Fields{
		"total_chats": len(chatInfos),
		"limit":       request.Limit,
		"offset":      request.Offset,
	}).Info("Listed chats successfully")

	return response, nil
}

func (service serviceChat) GetChatMessages(ctx context.Context, request domainChat.GetChatMessagesRequest) (response domainChat.GetChatMessagesResponse, err error) {
	if err = validations.ValidateGetChatMessages(ctx, &request); err != nil {
		return response, err
	}

	// Normalize JID from @lid to @s.whatsapp.net if needed
	// This ensures queries match messages stored with normalized JIDs
	normalizedJID := whatsapp.NormalizeJIDString(ctx, request.ChatJID)
	if normalizedJID != request.ChatJID {
		logrus.WithFields(logrus.Fields{
			"original_jid":   request.ChatJID,
			"normalized_jid": normalizedJID,
		}).Debug("Normalized LID to phone number JID for chat query")
		request.ChatJID = normalizedJID
	}

	// Get chat info first
	chat, err := service.chatStorageRepo.GetChat(request.ChatJID)
	if err != nil {
		logrus.WithError(err).WithField("chat_jid", request.ChatJID).Error("Failed to get chat info")
		return response, err
	}
	if chat == nil {
		return response, fmt.Errorf("chat with JID %s not found", request.ChatJID)
	}

	// Create message filter from request
	filter := &domainChatStorage.MessageFilter{
		ChatJID:   request.ChatJID,
		Limit:     request.Limit,
		Offset:    request.Offset,
		MediaOnly: request.MediaOnly,
		IsFromMe:  request.IsFromMe,
	}

	// Parse time filters if provided
	if request.StartTime != nil && *request.StartTime != "" {
		startTime, err := time.Parse(time.RFC3339, *request.StartTime)
		if err != nil {
			return response, fmt.Errorf("invalid start_time format: %v", err)
		}
		filter.StartTime = &startTime
	}

	if request.EndTime != nil && *request.EndTime != "" {
		endTime, err := time.Parse(time.RFC3339, *request.EndTime)
		if err != nil {
			return response, fmt.Errorf("invalid end_time format: %v", err)
		}
		filter.EndTime = &endTime
	}

	// Get messages from storage
	var messages []*domainChatStorage.Message
	if request.Search != "" {
		// Use search functionality if search query is provided
		messages, err = service.chatStorageRepo.SearchMessages(request.ChatJID, request.Search, request.Limit)
		if err != nil {
			logrus.WithError(err).WithField("chat_jid", request.ChatJID).Error("Failed to search messages")
			return response, err
		}
	} else {
		// Use regular filter
		messages, err = service.chatStorageRepo.GetMessages(filter)
		if err != nil {
			logrus.WithError(err).WithField("chat_jid", request.ChatJID).Error("Failed to get messages")
			return response, err
		}
	}

	// Get total message count for pagination
	totalCount, err := service.chatStorageRepo.GetChatMessageCount(request.ChatJID)
	if err != nil {
		logrus.WithError(err).WithField("chat_jid", request.ChatJID).Error("Failed to get message count")
		// Continue with partial data
		totalCount = 0
	}

	// Get device ID for constructing S3 media URLs
	var deviceID string
	if client := whatsapp.GetClient(); client != nil && client.Store != nil && client.Store.ID != nil {
		deviceID = client.Store.ID.User
	}

	// Convert entities to domain objects
	messageInfos := make([]domainChat.MessageInfo, 0, len(messages))
	for _, message := range messages {
		// Determine URL: use constructed S3 URL for media messages if S3 storage is enabled
		// and auto-download is enabled (media should exist in S3 only if auto-download was on)
		url := message.URL
		if message.MediaType != "" && storage.IsS3StorageEnabled() && config.WhatsappAutoDownloadMedia && deviceID != "" {
			if s3URL := storage.ConstructMediaURL(deviceID, message.ChatJID, message.ID, message.MediaType); s3URL != "" {
				url = s3URL
			}
		}

		messageInfo := domainChat.MessageInfo{
			ID:         message.ID,
			ChatJID:    message.ChatJID,
			SenderJID:  message.Sender,
			Content:    message.Content,
			Timestamp:  message.Timestamp.Format(time.RFC3339),
			IsFromMe:   message.IsFromMe,
			MediaType:  message.MediaType,
			Filename:   message.Filename,
			URL:        url,
			FileLength: message.FileLength,
			Status:     message.Status,
			CreatedAt:  message.CreatedAt.Format(time.RFC3339),
			UpdatedAt:  message.UpdatedAt.Format(time.RFC3339),
		}

		// Format status timestamps if available
		if message.DeliveredAt != nil {
			deliveredAt := message.DeliveredAt.Format(time.RFC3339)
			messageInfo.DeliveredAt = &deliveredAt
		}
		if message.ReadAt != nil {
			readAt := message.ReadAt.Format(time.RFC3339)
			messageInfo.ReadAt = &readAt
		}
		if message.PlayedAt != nil {
			playedAt := message.PlayedAt.Format(time.RFC3339)
			messageInfo.PlayedAt = &playedAt
		}

		messageInfos = append(messageInfos, messageInfo)
	}

	// Create chat info for response
	chatInfo := domainChat.ChatInfo{
		JID:                 chat.JID,
		Name:                chat.Name,
		LastMessageTime:     chat.LastMessageTime.Format(time.RFC3339),
		EphemeralExpiration: chat.EphemeralExpiration,
		CreatedAt:           chat.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           chat.UpdatedAt.Format(time.RFC3339),
	}

	// Create pagination response
	pagination := domainChat.PaginationResponse{
		Limit:  request.Limit,
		Offset: request.Offset,
		Total:  int(totalCount),
	}

	response.Data = messageInfos
	response.Pagination = pagination
	response.ChatInfo = chatInfo

	logrus.WithFields(logrus.Fields{
		"chat_jid":       request.ChatJID,
		"total_messages": len(messageInfos),
		"limit":          request.Limit,
		"offset":         request.Offset,
	}).Info("Retrieved chat messages successfully")

	return response, nil
}

func (service serviceChat) PinChat(ctx context.Context, request domainChat.PinChatRequest) (response domainChat.PinChatResponse, err error) {
	if err = validations.ValidatePinChat(ctx, &request); err != nil {
		return response, err
	}

	// Validate JID and ensure connection
	targetJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.ChatJID)
	if err != nil {
		return response, err
	}

	// Build pin patch using whatsmeow's BuildPin
	patchInfo := appstate.BuildPin(targetJID, request.Pinned)

	// Send app state update
	if err = whatsapp.GetClient().SendAppState(ctx, patchInfo); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"chat_jid": request.ChatJID,
			"pinned":   request.Pinned,
		}).Error("Failed to send pin chat app state")
		return response, err
	}

	// Build response
	response.Status = "success"
	response.ChatJID = request.ChatJID
	response.Pinned = request.Pinned

	if request.Pinned {
		response.Message = "Chat pinned successfully"
	} else {
		response.Message = "Chat unpinned successfully"
	}

	logrus.WithFields(logrus.Fields{
		"chat_jid": request.ChatJID,
		"pinned":   request.Pinned,
	}).Info("Chat pin operation completed successfully")

	return response, nil
}

func (service serviceChat) SetDisappearingTimer(ctx context.Context, request domainChat.SetDisappearingTimerRequest) (response domainChat.SetDisappearingTimerResponse, err error) {
	if err = validations.ValidateSetDisappearingTimer(ctx, &request); err != nil {
		return response, err
	}

	// Validate JID and ensure connection
	targetJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.ChatJID)
	if err != nil {
		return response, err
	}

	// Set disappearing timer using whatsmeow
	if err = whatsapp.GetClient().SetDisappearingTimer(ctx, targetJID, time.Duration(request.TimerSeconds)*time.Second, time.Now()); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"chat_jid":      request.ChatJID,
			"timer_seconds": request.TimerSeconds,
		}).Error("Failed to set disappearing timer")
		return response, err
	}

	// Update local storage immediately for consistency
	if existingChat, err := service.chatStorageRepo.GetChat(request.ChatJID); err == nil && existingChat != nil {
		existingChat.EphemeralExpiration = request.TimerSeconds
		if storeErr := service.chatStorageRepo.StoreChat(existingChat); storeErr != nil {
			logrus.Warnf("failed to update local chat storage: %v", storeErr)
		}
	}

	// Build response
	response.Status = "success"
	response.ChatJID = request.ChatJID
	response.TimerSeconds = request.TimerSeconds

	if request.TimerSeconds == 0 {
		response.Message = "Disappearing messages disabled"
	} else {
		response.Message = fmt.Sprintf("Disappearing messages set to %d seconds", request.TimerSeconds)
	}

	logrus.WithFields(logrus.Fields{
		"chat_jid":      request.ChatJID,
		"timer_seconds": request.TimerSeconds,
	}).Info("Disappearing timer set successfully")

	return response, nil
}

func (service serviceChat) ExportStorage(ctx context.Context) (filePath string, err error) {
	// Get the database path from config
	dbPath := config.ChatStorageURI

	// Handle file: prefix in URI
	if strings.HasPrefix(dbPath, "file:") {
		dbPath = strings.TrimPrefix(dbPath, "file:")
		// Remove any query parameters (like ?_foreign_keys=on)
		if idx := strings.Index(dbPath, "?"); idx != -1 {
			dbPath = dbPath[:idx]
		}
	}

	// Check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return "", fmt.Errorf("chat storage database not found at %s", dbPath)
	}

	logrus.WithField("path", dbPath).Info("Exporting chat storage database")
	return dbPath, nil
}

func (service serviceChat) ImportStorage(ctx context.Context, backupFilePath string) (response domainChat.ImportStorageResponse, err error) {
	startTime := time.Now()

	// Validate backup file exists
	if _, err := os.Stat(backupFilePath); os.IsNotExist(err) {
		return response, fmt.Errorf("backup file not found: %s", backupFilePath)
	}

	// Open backup database
	backupDB, err := sql.Open("sqlite3", backupFilePath+"?mode=ro")
	if err != nil {
		return response, fmt.Errorf("failed to open backup database: %w", err)
	}
	defer backupDB.Close()

	// Verify it's a valid chat storage database by checking for expected tables
	var tableName string
	err = backupDB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='chats'").Scan(&tableName)
	if err != nil {
		return response, fmt.Errorf("invalid backup file: missing 'chats' table")
	}

	// Import chats
	chatsImported, chatsSkipped, err := service.importChats(ctx, backupDB)
	if err != nil {
		logrus.WithError(err).Error("Failed to import chats")
		return response, fmt.Errorf("failed to import chats: %w", err)
	}

	// Import messages
	messagesImported, messagesSkipped, err := service.importMessages(ctx, backupDB)
	if err != nil {
		logrus.WithError(err).Error("Failed to import messages")
		return response, fmt.Errorf("failed to import messages: %w", err)
	}

	// Build response
	response.Status = "success"
	response.Message = "Import completed successfully"
	response.Imported = domainChat.ImportStorageStats{
		Chats:    chatsImported,
		Messages: messagesImported,
	}
	response.Skipped = domainChat.ImportStorageStats{
		Chats:    chatsSkipped,
		Messages: messagesSkipped,
	}
	response.Duration = time.Since(startTime).String()

	logrus.WithFields(logrus.Fields{
		"chats_imported":    chatsImported,
		"chats_skipped":     chatsSkipped,
		"messages_imported": messagesImported,
		"messages_skipped":  messagesSkipped,
		"duration":          response.Duration,
	}).Info("Chat storage import completed")

	return response, nil
}

func (service serviceChat) importChats(ctx context.Context, backupDB *sql.DB) (imported, skipped int64, err error) {
	// Query only the actual columns in the chats table
	// Note: last_message, last_message_from_me, last_message_type are computed via JOIN, not stored
	rows, err := backupDB.Query(`
		SELECT jid, name, last_message_time, ephemeral_expiration, created_at, updated_at
		FROM chats
	`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var chat domainChatStorage.Chat
		var lastMessageTime, createdAt, updatedAt string

		err := rows.Scan(
			&chat.JID,
			&chat.Name,
			&lastMessageTime,
			&chat.EphemeralExpiration,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			logrus.WithError(err).Warn("Failed to scan chat row, skipping")
			skipped++
			continue
		}

		// Parse timestamps
		chat.LastMessageTime, _ = time.Parse(time.RFC3339, lastMessageTime)
		chat.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		chat.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		// Check if chat already exists
		existing, _ := service.chatStorageRepo.GetChat(chat.JID)
		if existing != nil {
			// Chat exists, skip (merge behavior - keep existing)
			skipped++
			continue
		}

		// Store chat (will use ON CONFLICT handling)
		if err := service.chatStorageRepo.StoreChat(&chat); err != nil {
			logrus.WithError(err).WithField("jid", chat.JID).Warn("Failed to store chat, skipping")
			skipped++
			continue
		}
		imported++
	}

	return imported, skipped, rows.Err()
}

func (service serviceChat) importMessages(ctx context.Context, backupDB *sql.DB) (imported, skipped int64, err error) {
	rows, err := backupDB.Query(`
		SELECT id, chat_jid, sender, content, timestamp, is_from_me,
		       media_type, filename, url, file_length, status,
		       delivered_at, read_at, played_at, created_at, updated_at
		FROM messages
	`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var msg domainChatStorage.Message
		var timestamp, createdAt, updatedAt string
		var deliveredAt, readAt, playedAt sql.NullString

		err := rows.Scan(
			&msg.ID,
			&msg.ChatJID,
			&msg.Sender,
			&msg.Content,
			&timestamp,
			&msg.IsFromMe,
			&msg.MediaType,
			&msg.Filename,
			&msg.URL,
			&msg.FileLength,
			&msg.Status,
			&deliveredAt,
			&readAt,
			&playedAt,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			logrus.WithError(err).Warn("Failed to scan message row, skipping")
			skipped++
			continue
		}

		// Parse timestamps
		msg.Timestamp, _ = time.Parse(time.RFC3339, timestamp)
		msg.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		msg.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		// Handle nullable timestamp fields
		if deliveredAt.Valid {
			t, _ := time.Parse(time.RFC3339, deliveredAt.String)
			msg.DeliveredAt = &t
		}
		if readAt.Valid {
			t, _ := time.Parse(time.RFC3339, readAt.String)
			msg.ReadAt = &t
		}
		if playedAt.Valid {
			t, _ := time.Parse(time.RFC3339, playedAt.String)
			msg.PlayedAt = &t
		}

		// Check if message already exists
		existing, _ := service.chatStorageRepo.GetMessageByID(msg.ID)
		if existing != nil {
			// Message exists, skip (merge behavior - keep existing)
			skipped++
			continue
		}

		// Store message (will use ON CONFLICT handling)
		if err := service.chatStorageRepo.StoreMessage(&msg); err != nil {
			logrus.WithError(err).WithField("id", msg.ID).Warn("Failed to store message, skipping")
			skipped++
			continue
		}
		imported++
	}

	return imported, skipped, rows.Err()
}

func (service serviceChat) AnalyzeStorage(ctx context.Context, filePath string) (response domainChat.AnalyzeStorageResponse, err error) {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return response, fmt.Errorf("failed to stat file: %w", err)
	}

	response.Filename = fileInfo.Name()
	response.SizeBytes = fileInfo.Size()
	response.Size = formatFileSize(fileInfo.Size())

	// Open the database
	db, err := sql.Open("sqlite3", filePath+"?mode=ro")
	if err != nil {
		return response, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get all tables
	tableRows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return response, fmt.Errorf("failed to query tables: %w", err)
	}
	defer tableRows.Close()

	var tables []domainChat.TableInfo
	for tableRows.Next() {
		var tableName string
		if err := tableRows.Scan(&tableName); err != nil {
			continue
		}

		// Get row count for this table
		var rowCount int64
		countRow := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName))
		if err := countRow.Scan(&rowCount); err != nil {
			rowCount = 0
		}

		tables = append(tables, domainChat.TableInfo{
			Name:     tableName,
			RowCount: rowCount,
		})
	}
	response.Tables = tables

	// Add schema descriptions
	response.Schema = []domainChat.SchemaDescription{
		{
			Table:       "chats",
			Description: "Stores chat/conversation info (JID, name, last message time, ephemeral settings)",
		},
		{
			Table:       "messages",
			Description: "Stores message content, sender, timestamps, media info, delivery status",
		},
		{
			Table:       "schema_info",
			Description: "Database version tracking",
		},
	}

	logrus.WithFields(logrus.Fields{
		"filename":    response.Filename,
		"size":        response.Size,
		"table_count": len(tables),
	}).Info("Analyzed storage database")

	return response, nil
}

func formatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GiB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MiB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KiB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
