package usecase

import (
	"context"
	"fmt"
	"time"

	domainAdmin "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/admin"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/sirupsen/logrus"
)

type serviceAdmin struct {
	chatStorageRepo domainChatStorage.IChatStorageRepository
}

// NewAdminService creates a new admin service
func NewAdminService(chatStorageRepo domainChatStorage.IChatStorageRepository) domainAdmin.IAdminUsecase {
	return &serviceAdmin{
		chatStorageRepo: chatStorageRepo,
	}
}

// GetStorageStats returns detailed storage statistics
func (s *serviceAdmin) GetStorageStats(ctx context.Context) (response domainAdmin.StorageStatsResponse, err error) {
	stats, err := s.chatStorageRepo.GetDetailedStats()
	if err != nil {
		return response, fmt.Errorf("failed to get storage stats: %w", err)
	}

	response.TotalChats = stats.TotalChats
	response.TotalMessages = stats.TotalMessages
	response.DatabaseSizeBytes = stats.DatabaseSizeBytes
	response.DatabaseSize = formatFileSize(stats.DatabaseSizeBytes)
	response.EmptyChats = stats.EmptyChats
	response.MediaMessages = stats.MediaMessages
	response.TextMessages = stats.TextMessages

	if stats.OldestMessage != nil {
		formatted := stats.OldestMessage.Format(time.RFC3339)
		response.OldestMessage = &formatted
	}
	if stats.NewestMessage != nil {
		formatted := stats.NewestMessage.Format(time.RFC3339)
		response.NewestMessage = &formatted
	}

	logrus.WithFields(logrus.Fields{
		"total_chats":    response.TotalChats,
		"total_messages": response.TotalMessages,
		"database_size":  response.DatabaseSize,
	}).Info("[Admin] Retrieved storage stats")

	return response, nil
}

// CleanupStorage performs cleanup operations based on the request
func (s *serviceAdmin) CleanupStorage(ctx context.Context, request domainAdmin.CleanupRequest) (response domainAdmin.CleanupResponse, err error) {
	response.DryRun = request.DryRun

	// Handle pattern-based cleanup
	if request.Pattern != "" {
		if request.DryRun {
			// For dry run, just count what would be deleted
			chatsCount, err := s.chatStorageRepo.GetTotalChatCount()
			if err != nil {
				return response, err
			}
			// We can't easily count pattern matches without implementing a separate method
			// So for dry run with pattern, we'll return 0 and let the user know to run without dry run
			logrus.Infof("[Admin] Dry run: would delete chats matching pattern '%s' (count not available in dry run)", request.Pattern)
			_ = chatsCount // suppress unused warning
		} else {
			chatsDeleted, messagesDeleted, _, err := s.chatStorageRepo.DeleteChatsByPattern(request.Pattern)
			if err != nil {
				return response, fmt.Errorf("failed to delete chats by pattern: %w", err)
			}
			response.ChatsDeleted += chatsDeleted
			response.MessagesDeleted += messagesDeleted
		}
	}

	// Handle older_than cleanup
	if request.OlderThan != "" {
		olderThan, err := time.Parse(time.RFC3339, request.OlderThan)
		if err != nil {
			return response, fmt.Errorf("invalid older_than format, expected RFC3339: %w", err)
		}

		if request.DryRun {
			logrus.Infof("[Admin] Dry run: would delete messages older than %s", olderThan.Format(time.RFC3339))
		} else {
			messagesDeleted, err := s.chatStorageRepo.DeleteMessagesOlderThan(olderThan)
			if err != nil {
				return response, fmt.Errorf("failed to delete old messages: %w", err)
			}
			response.MessagesDeleted += messagesDeleted
		}
	}

	// Handle empty chats cleanup
	if request.EmptyChats {
		if request.DryRun {
			count, _ := s.chatStorageRepo.CountEmptyChats()
			logrus.Infof("[Admin] Dry run: would delete %d empty chats", count)
			response.ChatsDeleted += count
		} else {
			deleted, err := s.chatStorageRepo.DeleteEmptyChats()
			if err != nil {
				return response, fmt.Errorf("failed to delete empty chats: %w", err)
			}
			response.ChatsDeleted += deleted
		}
	}

	return response, nil
}

// VacuumDatabase runs SQLite VACUUM to optimize and reclaim space
func (s *serviceAdmin) VacuumDatabase(ctx context.Context) (response domainAdmin.VacuumResponse, err error) {
	// Get size before vacuum
	sizeBefore, err := s.chatStorageRepo.GetDatabaseSize()
	if err != nil {
		return response, fmt.Errorf("failed to get database size before vacuum: %w", err)
	}
	response.SizeBeforeBytes = sizeBefore
	response.SizeBefore = formatFileSize(sizeBefore)

	// Run vacuum
	if err := s.chatStorageRepo.VacuumDatabase(); err != nil {
		return response, fmt.Errorf("vacuum failed: %w", err)
	}

	// Get size after vacuum
	sizeAfter, err := s.chatStorageRepo.GetDatabaseSize()
	if err != nil {
		return response, fmt.Errorf("failed to get database size after vacuum: %w", err)
	}
	response.SizeAfterBytes = sizeAfter
	response.SizeAfter = formatFileSize(sizeAfter)

	// Calculate reclaimed space
	response.ReclaimedBytes = sizeBefore - sizeAfter
	if response.ReclaimedBytes < 0 {
		response.ReclaimedBytes = 0
	}
	response.Reclaimed = formatFileSize(response.ReclaimedBytes)

	logrus.WithFields(logrus.Fields{
		"size_before": response.SizeBefore,
		"size_after":  response.SizeAfter,
		"reclaimed":   response.Reclaimed,
	}).Info("[Admin] Database vacuum completed")

	return response, nil
}

// DeleteChats deletes chats by pattern or specific JIDs
func (s *serviceAdmin) DeleteChats(ctx context.Context, request domainAdmin.DeleteChatsRequest) (response domainAdmin.DeleteChatsResponse, err error) {
	response.DryRun = request.DryRun

	// Validate request
	if len(request.JIDs) == 0 && request.Pattern == "" {
		return response, fmt.Errorf("either jids or pattern must be provided")
	}

	// Handle JID-based deletion
	if len(request.JIDs) > 0 {
		if request.DryRun {
			response.ChatsDeleted = int64(len(request.JIDs))
			response.DeletedJIDs = request.JIDs
			logrus.Infof("[Admin] Dry run: would delete %d chats by JID", len(request.JIDs))
		} else {
			chatsDeleted, messagesDeleted, err := s.chatStorageRepo.DeleteChatsByJIDs(request.JIDs)
			if err != nil {
				return response, fmt.Errorf("failed to delete chats by JIDs: %w", err)
			}
			response.ChatsDeleted = chatsDeleted
			response.MessagesDeleted = messagesDeleted
			response.DeletedJIDs = request.JIDs
		}
	}

	// Handle pattern-based deletion
	if request.Pattern != "" {
		if request.DryRun {
			logrus.Infof("[Admin] Dry run: would delete chats matching pattern '%s'", request.Pattern)
		} else {
			chatsDeleted, messagesDeleted, deletedJIDs, err := s.chatStorageRepo.DeleteChatsByPattern(request.Pattern)
			if err != nil {
				return response, fmt.Errorf("failed to delete chats by pattern: %w", err)
			}
			response.ChatsDeleted += chatsDeleted
			response.MessagesDeleted += messagesDeleted
			response.DeletedJIDs = append(response.DeletedJIDs, deletedJIDs...)
		}
	}

	return response, nil
}
