package whatsapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

// CleanupDatabase deletes all devices from the PostgreSQL database
func CleanupDatabase() error {
	globalStateMu.RLock()
	currentDB := db
	currentKeysDB := keysDB
	globalStateMu.RUnlock()

	if currentDB == nil {
		return nil
	}

	ctx := context.Background()

	// Get and delete all devices
	devices, err := currentDB.GetAllDevices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get devices: %v", err)
	}

	for _, device := range devices {
		if err := currentDB.DeleteDevice(ctx, device); err != nil {
			return fmt.Errorf("failed to delete device %s: %v", device.ID, err)
		}
	}

	// Also clean up keysDB if it exists and is separate
	if currentKeysDB != nil && currentKeysDB != currentDB {
		keysDevices, err := currentKeysDB.GetAllDevices(ctx)
		if err != nil {
			return fmt.Errorf("failed to get devices from keysDB: %v", err)
		}

		for _, device := range keysDevices {
			if err := currentKeysDB.DeleteDevice(ctx, device); err != nil {
				return fmt.Errorf("failed to delete device %s from keysDB: %v", device.ID, err)
			}
		}
	}

	return nil
}

// CleanupTemporaryFiles removes history files, QR images, and send items
func CleanupTemporaryFiles() error {
	// Clean up history files
	if files, err := filepath.Glob(fmt.Sprintf("./%s/history-*", config.PathStorages)); err == nil {
		for _, f := range files {
			_ = os.Remove(f)
		}
	}

	// Clean up QR images
	if qrImages, err := filepath.Glob(fmt.Sprintf("./%s/scan-*", config.PathQrCode)); err == nil {
		for _, f := range qrImages {
			_ = os.Remove(f)
		}
	}

	// Clean up send items
	if qrItems, err := filepath.Glob(fmt.Sprintf("./%s/*", config.PathSendItems)); err == nil {
		for _, f := range qrItems {
			if !strings.Contains(f, ".gitignore") {
				_ = os.Remove(f)
			}
		}
	}

	return nil
}

// ReinitializeWhatsAppComponents reinitializes database and client components
func ReinitializeWhatsAppComponents(ctx context.Context, chatStorageRepo domainChatStorage.IChatStorageRepository) (*sqlstore.Container, *whatsmeow.Client, error) {
	newDB := InitWaDB(ctx, config.DBURI)
	var newKeysDB *sqlstore.Container
	if config.DBKeysURI != "" {
		newKeysDB = InitWaDB(ctx, config.DBKeysURI)
	}
	newCli := InitWaCLI(ctx, newDB, newKeysDB, chatStorageRepo)

	return newDB, newCli, nil
}

// PerformCompleteCleanup performs all cleanup operations in the correct order
func PerformCompleteCleanup(ctx context.Context, logPrefix string, chatStorageRepo domainChatStorage.IChatStorageRepository) (*sqlstore.Container, *whatsmeow.Client, error) {
	logrus.Debugf("[%s] Starting cleanup...", logPrefix)

	// Disconnect current client if it exists
	if current := GetClient(); current != nil {
		current.Disconnect()
		// Wait for background goroutines to finish before cleanup
		time.Sleep(2 * time.Second)
	}

	// Truncate all chatstorage data before other cleanup
	if chatStorageRepo != nil {
		if err := chatStorageRepo.TruncateAllDataWithLogging(logPrefix); err != nil {
			logrus.Errorf("[%s] Failed to truncate chatstorage: %v", logPrefix, err)
		}
	}

	// Clean up database
	if err := CleanupDatabase(); err != nil {
		return nil, nil, fmt.Errorf("database cleanup failed: %v", err)
	}

	// Reinitialize components with retry logic for file lock issues
	var newDB *sqlstore.Container
	var newCli *whatsmeow.Client
	var err error

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		newDB, newCli, err = ReinitializeWhatsAppComponents(ctx, chatStorageRepo)
		if err == nil {
			break
		}
		logrus.Warnf("[%s] Reinitialization attempt %d/%d failed: %v", logPrefix, i+1, maxRetries, err)
		if i < maxRetries-1 {
			time.Sleep(500 * time.Millisecond) // Wait before retry
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reinitialization failed after %d attempts: %v", maxRetries, err)
	}

	// Clean up temporary files (non-critical)
	_ = CleanupTemporaryFiles()

	logrus.Debugf("[%s] Cleanup completed, ready for new login", logPrefix)

	return newDB, newCli, nil
}

// PerformCleanupAndUpdateGlobals is a convenience function that performs cleanup
// and ensures global client synchronization
func PerformCleanupAndUpdateGlobals(ctx context.Context, logPrefix string, chatStorageRepo domainChatStorage.IChatStorageRepository) (*sqlstore.Container, *whatsmeow.Client, error) {
	newDB, newCli, err := PerformCompleteCleanup(ctx, logPrefix, chatStorageRepo)
	if err != nil {
		return nil, nil, err
	}

	// Ensure global client is properly synchronized
	UpdateGlobalClient(newCli, newDB)

	return newDB, newCli, nil
}

// handleRemoteLogout performs cleanup when user logs out from their phone
func handleRemoteLogout(ctx context.Context, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	logrus.Warn("[REMOTE_LOGOUT] User logged out from phone")

	// Perform complete cleanup with global client synchronization
	_, _, err := PerformCleanupAndUpdateGlobals(ctx, "REMOTE_LOGOUT", chatStorageRepo)
	if err != nil {
		logrus.Errorf("[REMOTE_LOGOUT] Cleanup failed: %v", err)
	}
}
