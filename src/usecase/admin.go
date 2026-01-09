package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainAdmin "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/admin"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
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

// =============================================================================
// WhatsApp Store Operations (device/keys database)
// =============================================================================

// GetWhatsAppStoreStats returns statistics for WhatsApp store databases
func (s *serviceAdmin) GetWhatsAppStoreStats(ctx context.Context) (response domainAdmin.WhatsAppStoreStatsResponse, err error) {
	// Get main DB path
	mainDBPath, err := getDBPathFromURI(config.DBURI)
	if err != nil {
		return response, fmt.Errorf("failed to parse main DB URI: %w", err)
	}

	// Get main DB size
	mainDBSize, err := getFileSizeBytes(mainDBPath)
	if err != nil {
		logrus.WithError(err).Warn("[Admin] Failed to get main DB size")
	}
	response.DatabaseSizeBytes = mainDBSize
	response.DatabaseSize = formatFileSize(mainDBSize)

	// Count devices in main DB
	deviceCount, err := countDevicesInDB(mainDBPath)
	if err != nil {
		logrus.WithError(err).Warn("[Admin] Failed to count devices")
	}
	response.DeviceCount = deviceCount

	// Check connection status
	isConnected, isLoggedIn, deviceJID := whatsapp.GetConnectionStatus()
	response.IsConnected = isConnected
	response.IsLoggedIn = isLoggedIn
	response.DeviceJID = deviceJID

	// Check keys DB if configured
	if config.DBKeysURI != "" {
		response.HasKeysDB = true
		keysDBPath, err := getDBPathFromURI(config.DBKeysURI)
		if err == nil {
			keysDBSize, err := getFileSizeBytes(keysDBPath)
			if err == nil {
				response.KeysDBSizeBytes = keysDBSize
				response.KeysDBSize = formatFileSize(keysDBSize)
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"database_size": response.DatabaseSize,
		"device_count":  response.DeviceCount,
		"is_connected":  response.IsConnected,
		"has_keys_db":   response.HasKeysDB,
	}).Info("[Admin] Retrieved WhatsApp store stats")

	return response, nil
}

// ExportWhatsAppStore prepares WhatsApp store databases for export
func (s *serviceAdmin) ExportWhatsAppStore(ctx context.Context) (response domainAdmin.WhatsAppStoreExportResponse, err error) {
	// Check if using PostgreSQL (not supported for file export)
	if strings.HasPrefix(config.DBURI, "postgres") {
		return response, fmt.Errorf("export not supported for PostgreSQL databases - use pg_dump instead")
	}

	// Get main DB path
	mainDBPath, err := getDBPathFromURI(config.DBURI)
	if err != nil {
		return response, fmt.Errorf("failed to parse main DB URI: %w", err)
	}

	// Verify file exists
	if _, err := os.Stat(mainDBPath); os.IsNotExist(err) {
		return response, fmt.Errorf("WhatsApp store database not found at %s", mainDBPath)
	}

	// Checkpoint WAL to ensure all data is in main file
	if err := checkpointWAL(mainDBPath); err != nil {
		logrus.WithError(err).Warn("[Admin] Failed to checkpoint WAL for main DB")
	}

	response.MainDBPath = mainDBPath

	// Check connection status for warning
	isConnected, _, _ := whatsapp.GetConnectionStatus()
	response.IsConnected = isConnected
	if isConnected {
		response.Warning = "Client is connected. For a consistent backup, consider disconnecting first."
	}

	// Handle keys DB if configured
	if config.DBKeysURI != "" && !strings.HasPrefix(config.DBKeysURI, "postgres") {
		keysDBPath, err := getDBPathFromURI(config.DBKeysURI)
		if err == nil {
			if _, err := os.Stat(keysDBPath); err == nil {
				if err := checkpointWAL(keysDBPath); err != nil {
					logrus.WithError(err).Warn("[Admin] Failed to checkpoint WAL for keys DB")
				}
				response.KeysDBPath = keysDBPath
				response.HasKeysDB = true
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"main_db_path": response.MainDBPath,
		"has_keys_db":  response.HasKeysDB,
		"is_connected": response.IsConnected,
	}).Info("[Admin] WhatsApp store export prepared")

	return response, nil
}

// ImportWhatsAppStore imports WhatsApp store from backup files
func (s *serviceAdmin) ImportWhatsAppStore(ctx context.Context, mainDBPath string, keysDBPath string) (response domainAdmin.WhatsAppStoreImportResponse, err error) {
	// Check if using PostgreSQL (not supported for file import)
	if strings.HasPrefix(config.DBURI, "postgres") {
		return response, fmt.Errorf("import not supported for PostgreSQL databases - use pg_restore instead")
	}

	// Verify client is disconnected
	isConnected, _, _ := whatsapp.GetConnectionStatus()
	if isConnected {
		return response, fmt.Errorf("client must be disconnected before importing. Call /app/logout first")
	}

	// Validate main backup file exists
	if _, err := os.Stat(mainDBPath); os.IsNotExist(err) {
		return response, fmt.Errorf("backup file not found: %s", mainDBPath)
	}

	// Validate it's a valid WhatsApp store database
	valid, err := isValidWhatsAppStoreDB(mainDBPath)
	if err != nil || !valid {
		return response, fmt.Errorf("invalid WhatsApp store backup file: missing expected tables")
	}

	// Get target path
	targetPath, err := getDBPathFromURI(config.DBURI)
	if err != nil {
		return response, fmt.Errorf("failed to parse target DB URI: %w", err)
	}

	// Create backup of existing database
	if _, err := os.Stat(targetPath); err == nil {
		backupPath := targetPath + ".backup." + time.Now().Format("20060102-150405")
		if err := copyFile(mainDBPath, backupPath); err != nil {
			logrus.WithError(err).Warn("[Admin] Failed to create backup of existing database")
		} else {
			logrus.WithField("backup_path", backupPath).Info("[Admin] Created backup of existing database")
		}
	}

	// Copy backup to target location
	if err := copyFile(mainDBPath, targetPath); err != nil {
		return response, fmt.Errorf("failed to copy backup to target location: %w", err)
	}
	response.MainDBImported = true

	// Handle keys DB import if provided
	if keysDBPath != "" && config.DBKeysURI != "" {
		if _, err := os.Stat(keysDBPath); err == nil {
			keysTargetPath, err := getDBPathFromURI(config.DBKeysURI)
			if err == nil {
				if err := copyFile(keysDBPath, keysTargetPath); err != nil {
					logrus.WithError(err).Warn("[Admin] Failed to import keys database")
				} else {
					response.KeysDBImported = true
				}
			}
		}
	}

	response.Status = "success"
	response.Message = "WhatsApp store imported successfully. Please restart the application for changes to take effect."
	response.RequiresRestart = true

	logrus.WithFields(logrus.Fields{
		"main_db_imported": response.MainDBImported,
		"keys_db_imported": response.KeysDBImported,
	}).Info("[Admin] WhatsApp store import completed")

	return response, nil
}

// VacuumWhatsAppStore runs VACUUM on WhatsApp store databases
func (s *serviceAdmin) VacuumWhatsAppStore(ctx context.Context) (response domainAdmin.WhatsAppStoreVacuumResponse, err error) {
	// Check if using PostgreSQL
	if strings.HasPrefix(config.DBURI, "postgres") {
		return response, fmt.Errorf("vacuum for PostgreSQL should be done using VACUUM ANALYZE command directly")
	}

	// Vacuum main DB
	mainDBPath, err := getDBPathFromURI(config.DBURI)
	if err != nil {
		return response, fmt.Errorf("failed to parse main DB URI: %w", err)
	}

	mainVacuum, err := vacuumSQLiteDB(mainDBPath)
	if err != nil {
		return response, fmt.Errorf("failed to vacuum main database: %w", err)
	}
	response.MainDB = mainVacuum

	// Check connection status for warning
	isConnected, _, _ := whatsapp.GetConnectionStatus()
	if isConnected {
		response.Warning = "Client is connected. Vacuum completed but may have limited effect on active database."
	}

	// Vacuum keys DB if configured
	if config.DBKeysURI != "" && !strings.HasPrefix(config.DBKeysURI, "postgres") {
		keysDBPath, err := getDBPathFromURI(config.DBKeysURI)
		if err == nil {
			keysVacuum, err := vacuumSQLiteDB(keysDBPath)
			if err != nil {
				logrus.WithError(err).Warn("[Admin] Failed to vacuum keys database")
			} else {
				response.KeysDB = &keysVacuum
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"main_db_reclaimed": response.MainDB.Reclaimed,
		"has_keys_db":       response.KeysDB != nil,
	}).Info("[Admin] WhatsApp store vacuum completed")

	return response, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// getDBPathFromURI extracts file path from database URI
func getDBPathFromURI(dbURI string) (string, error) {
	if strings.HasPrefix(dbURI, "file:") {
		path := strings.TrimPrefix(dbURI, "file:")
		// Remove query parameters
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		return path, nil
	}
	if strings.HasPrefix(dbURI, "postgres") {
		return "", fmt.Errorf("PostgreSQL URI cannot be converted to file path")
	}
	return dbURI, nil
}

// getFileSizeBytes returns file size in bytes
func getFileSizeBytes(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// countDevicesInDB counts devices in WhatsApp store database
func countDevicesInDB(dbPath string) (int64, error) {
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var count int64
	// Try whatsmeow table name
	err = db.QueryRow("SELECT COUNT(*) FROM whatsmeow_device").Scan(&count)
	if err != nil {
		// Try alternative table name
		err = db.QueryRow("SELECT COUNT(*) FROM whatsapp_keys_devices").Scan(&count)
		if err != nil {
			return 0, err
		}
	}
	return count, nil
}

// checkpointWAL runs SQLite WAL checkpoint
func checkpointWAL(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return err
	}
	logrus.WithField("path", dbPath).Debug("[Admin] WAL checkpoint completed")
	return nil
}

// isValidWhatsAppStoreDB checks if database has expected WhatsApp store tables
func isValidWhatsAppStoreDB(dbPath string) (bool, error) {
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return false, err
	}
	defer db.Close()

	// Check for whatsmeow device table
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='whatsmeow_device'").Scan(&tableName)
	if err == nil {
		return true, nil
	}

	// Try alternative table name
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='whatsapp_keys_devices'").Scan(&tableName)
	if err == nil {
		return true, nil
	}

	return false, fmt.Errorf("no WhatsApp store tables found")
}

// vacuumSQLiteDB runs VACUUM on a SQLite database and returns size stats
func vacuumSQLiteDB(dbPath string) (domainAdmin.VacuumResponse, error) {
	var response domainAdmin.VacuumResponse

	// Get size before
	sizeBefore, err := getFileSizeBytes(dbPath)
	if err != nil {
		return response, fmt.Errorf("failed to get size before vacuum: %w", err)
	}
	response.SizeBeforeBytes = sizeBefore
	response.SizeBefore = formatFileSize(sizeBefore)

	// Open database and run vacuum
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return response, err
	}
	defer db.Close()

	if _, err := db.Exec("VACUUM"); err != nil {
		return response, fmt.Errorf("vacuum failed: %w", err)
	}

	// Get size after
	sizeAfter, err := getFileSizeBytes(dbPath)
	if err != nil {
		return response, fmt.Errorf("failed to get size after vacuum: %w", err)
	}
	response.SizeAfterBytes = sizeAfter
	response.SizeAfter = formatFileSize(sizeAfter)

	// Calculate reclaimed
	response.ReclaimedBytes = sizeBefore - sizeAfter
	if response.ReclaimedBytes < 0 {
		response.ReclaimedBytes = 0
	}
	response.Reclaimed = formatFileSize(response.ReclaimedBytes)

	return response, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	buf := make([]byte, 1024*1024) // 1MB buffer
	for {
		n, err := sourceFile.Read(buf)
		if n > 0 {
			if _, writeErr := destFile.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
	}

	return destFile.Sync()
}
