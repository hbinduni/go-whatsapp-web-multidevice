package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	domainAdmin "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/admin"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/sirupsen/logrus"
)

type serviceAdmin struct {
	chatStorageRepo domainChatStorage.IChatStorageRepository
	db              *sql.DB // Shared database for creating per-client repositories
}

// NewAdminService creates a new admin service
func NewAdminService(chatStorageRepo domainChatStorage.IChatStorageRepository, db *sql.DB) domainAdmin.IAdminUsecase {
	return &serviceAdmin{
		chatStorageRepo: chatStorageRepo,
		db:              db,
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
			count, err := s.chatStorageRepo.CountEmptyChats()
			if err != nil {
				logrus.Warnf("[Admin] Dry run: failed to count empty chats: %v", err)
			} else {
				logrus.Infof("[Admin] Dry run: would delete %d empty chats", count)
				response.ChatsDeleted += count
			}
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

// VacuumDatabase runs VACUUM to optimize and reclaim space
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

// formatFileSize converts bytes to a human-readable format
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

// AddClient dynamically adds a new WhatsApp client
func (s *serviceAdmin) AddClient(ctx context.Context, request domainAdmin.AddClientRequest) (response domainAdmin.AddClientResponse, err error) {
	response.Phone = request.Phone
	response.DisplayName = request.DisplayName

	// Validate phone number format
	normalizedPhone, err := whatsapp.ValidatePhone(request.Phone)
	if err != nil {
		return response, fmt.Errorf("invalid phone number: %w", err)
	}
	response.Phone = normalizedPhone

	// Get the registry
	registry := whatsapp.GetRegistry()
	if registry == nil {
		return response, fmt.Errorf("client registry not initialized")
	}

	// Check MAX_CLIENTS limit
	if !registry.CanAddClient() {
		currentCount := registry.GetClientCount()
		maxClients := registry.GetMaxClients()
		return response, fmt.Errorf("maximum client limit reached (%d/%d). Remove a client before adding new ones", currentCount, maxClients)
	}

	// Check if client already exists in registry
	if _, err := registry.GetClient(normalizedPhone); err == nil {
		response.Status = "already_registered"
		response.Message = "Client is already registered"
		return response, nil
	}

	// Check if client is already registered in database (might be inactive)
	isRegistered, err := s.chatStorageRepo.IsClientRegistered(normalizedPhone)
	if err != nil {
		logrus.Warnf("[Admin] Failed to check client registration: %v", err)
	}
	if isRegistered {
		// Client exists in DB but not in registry - might be from a previous session
		logrus.Infof("[Admin] Reactivating previously registered client: %s", normalizedPhone)
	}

	// Add to registered_clients table
	if err := s.chatStorageRepo.AddRegisteredClient(normalizedPhone, request.DisplayName); err != nil {
		return response, fmt.Errorf("failed to persist client registration: %w", err)
	}

	// Create chat storage repository for new client
	clientChatStorageRepo := chatstorage.NewPostgresRepository(s.db, normalizedPhone)

	// Initialize schema for this client
	if err := clientChatStorageRepo.InitializeSchema(); err != nil {
		// Rollback: remove from registered_clients
		_ = s.chatStorageRepo.RemoveRegisteredClient(normalizedPhone)
		return response, fmt.Errorf("failed to initialize chat storage schema: %w", err)
	}

	// Register client in registry
	_, err = registry.RegisterClient(ctx, normalizedPhone, clientChatStorageRepo)
	if err != nil {
		// Rollback: remove from registered_clients
		_ = s.chatStorageRepo.RemoveRegisteredClient(normalizedPhone)
		return response, fmt.Errorf("failed to register client: %w", err)
	}

	// Connect the client (it will be in disconnected state, awaiting QR login)
	if err := registry.ConnectClient(normalizedPhone); err != nil {
		logrus.Warnf("[Admin] Client registered but failed to connect: %v", err)
		response.Status = "registered_not_connected"
		response.Message = fmt.Sprintf("Client registered but connection failed: %v. Use /api/%s/app/login to retry", err, normalizedPhone)
		return response, nil
	}

	response.Status = "awaiting_login"
	response.Message = fmt.Sprintf("Client registered successfully. Use /api/%s/app/login to get QR code", normalizedPhone)

	logrus.WithFields(logrus.Fields{
		"phone":        normalizedPhone,
		"display_name": request.DisplayName,
	}).Info("[Admin] Added new client")

	return response, nil
}

// RemoveClient removes a WhatsApp client (keeps chat history)
func (s *serviceAdmin) RemoveClient(ctx context.Context, phone string) (response domainAdmin.RemoveClientResponse, err error) {
	response.Phone = phone

	// Validate phone number format
	normalizedPhone, err := whatsapp.ValidatePhone(phone)
	if err != nil {
		return response, fmt.Errorf("invalid phone number: %w", err)
	}
	response.Phone = normalizedPhone

	// Get the registry
	registry := whatsapp.GetRegistry()
	if registry == nil {
		return response, fmt.Errorf("client registry not initialized")
	}

	// Get the client from registry
	managedClient, err := registry.GetClient(normalizedPhone)
	if err != nil {
		// Client not in registry - check if it's in database
		isRegistered, dbErr := s.chatStorageRepo.IsClientRegistered(normalizedPhone)
		if dbErr != nil || !isRegistered {
			return response, fmt.Errorf("client not found: %s", normalizedPhone)
		}
		// Client is in DB but not in registry - just mark as inactive
		if err := s.chatStorageRepo.RemoveRegisteredClient(normalizedPhone); err != nil {
			return response, fmt.Errorf("failed to remove client registration: %w", err)
		}
		response.Message = "Client registration removed (was not active)"
		return response, nil
	}

	// Logout from WhatsApp (this will invalidate the session)
	if managedClient.Client != nil && managedClient.Client.IsLoggedIn() {
		if err := managedClient.Client.Logout(ctx); err != nil {
			logrus.Warnf("[Admin] Failed to logout client %s: %v", normalizedPhone, err)
			// Continue with removal even if logout fails
		}
	}

	// Disconnect client
	if err := registry.DisconnectClient(normalizedPhone); err != nil {
		logrus.Warnf("[Admin] Failed to disconnect client %s: %v", normalizedPhone, err)
	}

	// Delete device from whatsmeow store (removes session/keys)
	if managedClient.Client != nil && managedClient.Client.Store != nil && managedClient.Client.Store.ID != nil {
		db := registry.GetDB()
		if db != nil {
			if err := db.DeleteDevice(ctx, managedClient.Client.Store); err != nil {
				logrus.Warnf("[Admin] Failed to delete device for %s: %v", normalizedPhone, err)
			}
		}
	}

	// Unregister from registry
	if err := registry.UnregisterClient(normalizedPhone); err != nil {
		logrus.Warnf("[Admin] Failed to unregister client %s: %v", normalizedPhone, err)
	}

	// Mark as inactive in registered_clients (soft delete - preserves audit trail)
	if err := s.chatStorageRepo.RemoveRegisteredClient(normalizedPhone); err != nil {
		logrus.Warnf("[Admin] Failed to remove client registration: %v", err)
	}

	// NOTE: Chat history is intentionally NOT deleted per user requirement
	response.Message = "Client disconnected and removed. Chat history preserved."

	logrus.WithFields(logrus.Fields{
		"phone": normalizedPhone,
	}).Info("[Admin] Removed client (chat history preserved)")

	return response, nil
}
