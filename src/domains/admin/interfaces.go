package admin

import "context"

// IAdminUsecase defines the interface for admin operations
type IAdminUsecase interface {
	// ==========================================================================
	// Chat Storage Operations (chatstorage.db)
	// ==========================================================================

	// GetStorageStats returns detailed storage statistics for chat storage
	GetStorageStats(ctx context.Context) (StorageStatsResponse, error)

	// CleanupStorage performs cleanup operations based on the request
	CleanupStorage(ctx context.Context, request CleanupRequest) (CleanupResponse, error)

	// VacuumDatabase runs SQLite VACUUM on chat storage to optimize and reclaim space
	VacuumDatabase(ctx context.Context) (VacuumResponse, error)

	// DeleteChats deletes chats by pattern or specific JIDs
	DeleteChats(ctx context.Context, request DeleteChatsRequest) (DeleteChatsResponse, error)

	// ==========================================================================
	// WhatsApp Store Operations (whatsapp.db - device/keys database)
	// ==========================================================================

	// GetWhatsAppStoreStats returns statistics for WhatsApp store databases
	GetWhatsAppStoreStats(ctx context.Context) (WhatsAppStoreStatsResponse, error)

	// ExportWhatsAppStore prepares WhatsApp store databases for export
	ExportWhatsAppStore(ctx context.Context) (WhatsAppStoreExportResponse, error)

	// ImportWhatsAppStore imports WhatsApp store from backup files
	// Note: Requires client disconnection and app restart after import
	ImportWhatsAppStore(ctx context.Context, mainDBPath string, keysDBPath string) (WhatsAppStoreImportResponse, error)

	// VacuumWhatsAppStore runs VACUUM on WhatsApp store databases
	VacuumWhatsAppStore(ctx context.Context) (WhatsAppStoreVacuumResponse, error)
}
