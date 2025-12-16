package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainHistory "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/history"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/sirupsen/logrus"
)

type serviceHistory struct{}

func NewHistoryService() domainHistory.IHistoryUsecase {
	return &serviceHistory{}
}

// RequestHistorySync implements history.IHistoryUsecase.
func (service serviceHistory) RequestHistorySync(ctx context.Context, request domainHistory.HistorySyncRequest) (response domainHistory.HistorySyncResponse, err error) {
	client := whatsapp.GetClient()
	if client == nil {
		return response, fmt.Errorf("WhatsApp client not initialized")
	}

	if !client.IsLoggedIn() {
		return response, fmt.Errorf("WhatsApp client not logged in")
	}

	// Check if history sync is enabled
	if !config.HistorySyncEnabled {
		return response, fmt.Errorf("history sync is disabled in configuration")
	}

	// Set default message count if not provided
	messageCount := request.MessageCount
	if messageCount <= 0 {
		messageCount = 50 // WhatsApp recommended default
	}

	// Use configured max days if not overridden in request
	maxDays := config.HistorySyncMaxDays
	if request.MaxDays > 0 {
		maxDays = request.MaxDays
	}

	response.RequestedAt = time.Now()
	response.MaxDays = maxDays
	response.MessageCount = messageCount

	// Calculate cutoff time
	if maxDays > 0 {
		response.EstimatedCutoff = time.Now().AddDate(0, 0, -int(maxDays))
	}

	// If chat JID is provided, request on-demand history for that chat
	if request.ChatJID != "" {
		jid, err := utils.ValidateJidWithLogin(client, request.ChatJID)
		if err != nil {
			return response, fmt.Errorf("invalid chat JID: %v", err)
		}

		logrus.Infof("Requesting on-demand history sync for chat %s (last %d messages)", jid.String(), messageCount)

		// Note: BuildHistorySyncRequest requires a reference message to sync before
		// For now, we'll document this as a limitation
		// Users need to call this API with a specific message ID as reference
		response.Status = "pending"
		response.Message = "On-demand history sync requires a reference message ID. This will be implemented in a future update. For now, history is automatically synced on login."
		response.ChatJID = jid.String()
		response.SyncType = "on_demand"

		return response, nil
	}

	// If no specific chat, return info about automatic sync
	response.Status = "info"
	response.Message = fmt.Sprintf("History sync is configured to process messages from the last %d days. History is automatically synced when you connect to WhatsApp.", maxDays)
	response.SyncType = "automatic"

	logrus.Infof("History sync status requested - enabled: %v, max_days: %d", config.HistorySyncEnabled, maxDays)

	return response, nil
}

// GetHistorySyncStatus implements history.IHistoryUsecase.
func (service serviceHistory) GetHistorySyncStatus(ctx context.Context) (response domainHistory.HistorySyncStatusResponse, err error) {
	response.Enabled = config.HistorySyncEnabled
	response.OnLogin = config.HistorySyncOnLogin
	response.MaxDays = config.HistorySyncMaxDays

	// Create user-friendly label for max days
	switch config.HistorySyncMaxDays {
	case -1:
		response.MaxDaysLabel = "All available (no limit)"
	case 90:
		response.MaxDaysLabel = "3 months"
	case 365:
		response.MaxDaysLabel = "1 year"
	case 730:
		response.MaxDaysLabel = "2 years"
	case 1095:
		response.MaxDaysLabel = "3 years"
	default:
		response.MaxDaysLabel = fmt.Sprintf("%d days", config.HistorySyncMaxDays)
	}

	return response, nil
}
