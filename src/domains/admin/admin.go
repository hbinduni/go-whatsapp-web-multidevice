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

// =============================================================================
// WhatsApp Store Management (Device/Keys Database)
// =============================================================================

// WhatsAppStoreStatsResponse contains statistics for WhatsApp store databases
type WhatsAppStoreStatsResponse struct {
	DatabaseSize      string `json:"database_size"`
	DatabaseSizeBytes int64  `json:"database_size_bytes"`
	DeviceCount       int64  `json:"device_count"`
	HasKeysDB         bool   `json:"has_keys_db"`
	KeysDBSize        string `json:"keys_db_size,omitempty"`
	KeysDBSizeBytes   int64  `json:"keys_db_size_bytes,omitempty"`
	IsConnected       bool   `json:"is_connected"`
	IsLoggedIn        bool   `json:"is_logged_in"`
	DeviceJID         string `json:"device_jid,omitempty"`
}

// WhatsAppStoreExportResponse contains export result paths
type WhatsAppStoreExportResponse struct {
	MainDBPath  string `json:"main_db_path"`
	KeysDBPath  string `json:"keys_db_path,omitempty"`
	HasKeysDB   bool   `json:"has_keys_db"`
	IsConnected bool   `json:"is_connected"`
	Warning     string `json:"warning,omitempty"`
}

// WhatsAppStoreImportResponse contains import result
type WhatsAppStoreImportResponse struct {
	Status          string `json:"status"`
	Message         string `json:"message"`
	MainDBImported  bool   `json:"main_db_imported"`
	KeysDBImported  bool   `json:"keys_db_imported"`
	RequiresRestart bool   `json:"requires_restart"`
}

// WhatsAppStoreVacuumResponse contains vacuum results for both databases
type WhatsAppStoreVacuumResponse struct {
	MainDB  VacuumResponse  `json:"main_db"`
	KeysDB  *VacuumResponse `json:"keys_db,omitempty"`
	Warning string          `json:"warning,omitempty"`
}
