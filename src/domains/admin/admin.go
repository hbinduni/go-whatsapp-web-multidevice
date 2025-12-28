package admin

// StorageStatsResponse contains detailed storage statistics
type StorageStatsResponse struct {
	TotalChats        int64   `json:"total_chats"`
	TotalMessages     int64   `json:"total_messages"`
	DatabaseSize      string  `json:"database_size"`
	DatabaseSizeBytes int64   `json:"database_size_bytes"`
	OldestMessage     *string `json:"oldest_message,omitempty"`
	NewestMessage     *string `json:"newest_message,omitempty"`
	EmptyChats        int64   `json:"empty_chats"`
	MediaMessages     int64   `json:"media_messages"`
	TextMessages      int64   `json:"text_messages"`
}

// CleanupRequest specifies what to clean up
type CleanupRequest struct {
	Pattern    string `json:"pattern"`     // SQL LIKE pattern (e.g., "%@broadcast")
	OlderThan  string `json:"older_than"`  // RFC3339 date - delete messages older than this
	EmptyChats bool   `json:"empty_chats"` // Delete chats with no messages
	DryRun     bool   `json:"dry_run"`     // Preview only, don't delete
}

// CleanupResponse contains the results of a cleanup operation
type CleanupResponse struct {
	ChatsDeleted    int64 `json:"chats_deleted"`
	MessagesDeleted int64 `json:"messages_deleted"`
	DryRun          bool  `json:"dry_run"`
}

// DeleteChatsRequest specifies which chats to delete
type DeleteChatsRequest struct {
	JIDs    []string `json:"jids"`    // Specific JIDs to delete
	Pattern string   `json:"pattern"` // SQL LIKE pattern
	DryRun  bool     `json:"dry_run"` // Preview only, don't delete
}

// DeleteChatsResponse contains the results of a delete operation
type DeleteChatsResponse struct {
	ChatsDeleted    int64    `json:"chats_deleted"`
	MessagesDeleted int64    `json:"messages_deleted"`
	DeletedJIDs     []string `json:"deleted_jids,omitempty"`
	DryRun          bool     `json:"dry_run"`
}

// VacuumResponse contains the results of a vacuum operation
type VacuumResponse struct {
	SizeBefore      string `json:"size_before"`
	SizeAfter       string `json:"size_after"`
	SizeBeforeBytes int64  `json:"size_before_bytes"`
	SizeAfterBytes  int64  `json:"size_after_bytes"`
	Reclaimed       string `json:"reclaimed"`
	ReclaimedBytes  int64  `json:"reclaimed_bytes"`
}
