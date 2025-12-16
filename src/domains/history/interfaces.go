package history

import "context"

// IHistoryUsecase defines the interface for history sync operations
type IHistoryUsecase interface {
	// RequestHistorySync triggers a manual history sync request
	RequestHistorySync(ctx context.Context, request HistorySyncRequest) (HistorySyncResponse, error)

	// GetHistorySyncStatus returns the current history sync configuration
	GetHistorySyncStatus(ctx context.Context) (HistorySyncStatusResponse, error)
}
