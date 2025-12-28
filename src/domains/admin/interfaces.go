package admin

import "context"

// IAdminUsecase defines the interface for admin operations
type IAdminUsecase interface {
	// GetStorageStats returns detailed storage statistics
	GetStorageStats(ctx context.Context) (StorageStatsResponse, error)

	// CleanupStorage performs cleanup operations based on the request
	CleanupStorage(ctx context.Context, request CleanupRequest) (CleanupResponse, error)

	// VacuumDatabase runs SQLite VACUUM to optimize and reclaim space
	VacuumDatabase(ctx context.Context) (VacuumResponse, error)

	// DeleteChats deletes chats by pattern or specific JIDs
	DeleteChats(ctx context.Context, request DeleteChatsRequest) (DeleteChatsResponse, error)
}
