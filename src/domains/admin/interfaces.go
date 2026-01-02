package admin

import "context"

// IAdminUsecase defines the interface for admin operations
type IAdminUsecase interface {
	// GetStorageStats returns detailed storage statistics
	GetStorageStats(ctx context.Context) (StorageStatsResponse, error)

	// CleanupStorage performs cleanup operations based on the request
	CleanupStorage(ctx context.Context, request CleanupRequest) (CleanupResponse, error)

	// VacuumDatabase runs VACUUM to optimize and reclaim space
	VacuumDatabase(ctx context.Context) (VacuumResponse, error)

	// DeleteChats deletes chats by pattern or specific JIDs
	DeleteChats(ctx context.Context, request DeleteChatsRequest) (DeleteChatsResponse, error)

	// AddClient dynamically adds a new WhatsApp client
	AddClient(ctx context.Context, request AddClientRequest) (AddClientResponse, error)

	// RemoveClient removes a WhatsApp client (keeps chat history)
	RemoveClient(ctx context.Context, phone string) (RemoveClientResponse, error)
}
