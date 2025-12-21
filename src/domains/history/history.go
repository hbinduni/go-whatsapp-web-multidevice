package history

import "time"

// HistorySyncRequest represents a request to sync historical data
type HistorySyncRequest struct {
	ChatJID      string `json:"chat_jid" form:"chat_jid"`
	MaxDays      int32  `json:"max_days" form:"max_days"`           // Optional: override default max days
	MessageCount int    `json:"message_count" form:"message_count"` // Number of messages to fetch per request (default 50)
}

// HistorySyncResponse represents the response from a history sync operation
type HistorySyncResponse struct {
	Status          string    `json:"status"`
	Message         string    `json:"message"`
	ChatJID         string    `json:"chat_jid,omitempty"`
	RequestedAt     time.Time `json:"requested_at"`
	MaxDays         int32     `json:"max_days"`
	MessageCount    int       `json:"message_count,omitempty"`
	SyncType        string    `json:"sync_type,omitempty"`
	EstimatedCutoff time.Time `json:"estimated_cutoff,omitempty"`
}

// HistorySyncStatusResponse represents the status of history sync configuration
type HistorySyncStatusResponse struct {
	Enabled      bool   `json:"enabled"`
	OnLogin      bool   `json:"on_login"`
	MaxDays      int32  `json:"max_days"`
	MaxDaysLabel string `json:"max_days_label"`
}
