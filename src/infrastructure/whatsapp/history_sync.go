package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// historySyncID tracks history sync file numbering
var historySyncID int32

func handleHistorySync(ctx context.Context, evt *events.HistorySync, chatStorageRepo domainChatStorage.IChatStorageRepository, client *whatsmeow.Client) {
	if !config.HistorySyncEnabled {
		log.Debugf("History sync is disabled, skipping sync type: %s", evt.Data.SyncType.String())
		return
	}

	if client == nil || client.Store == nil || client.Store.ID == nil {
		log.Warnf("Skipping history sync handling: WhatsApp client not initialized")
		return
	}
	id := atomic.AddInt32(&historySyncID, 1)
	fileName := fmt.Sprintf("%s/history-%d-%s-%d-%s.json",
		config.PathStorages,
		startupTime,
		client.Store.ID.String(),
		id,
		evt.Data.SyncType.String(),
	)

	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Errorf("Failed to open file to write history sync: %v", err)
		return
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err = enc.Encode(evt.Data); err != nil {
		log.Errorf("Failed to write history sync: %v", err)
		return
	}

	log.Infof("Wrote history sync to %s (type: %s)", fileName, evt.Data.SyncType.String())

	// Process history sync data to database
	if chatStorageRepo != nil {
		if err := processHistorySync(ctx, evt.Data, chatStorageRepo, client); err != nil {
			log.Errorf("Failed to process history sync to database: %v", err)
		}
	}
}

// processHistorySync processes history sync data and stores messages in the database
func processHistorySync(ctx context.Context, data *waHistorySync.HistorySync, chatStorageRepo domainChatStorage.IChatStorageRepository, client *whatsmeow.Client) error {
	if data == nil {
		return nil
	}

	syncType := data.GetSyncType()
	log.Debugf("Processing history sync type: %s", syncType.String())

	switch syncType {
	case waHistorySync.HistorySync_INITIAL_BOOTSTRAP, waHistorySync.HistorySync_RECENT:
		return processConversationMessages(ctx, data, chatStorageRepo, client)
	case waHistorySync.HistorySync_PUSH_NAME:
		return processPushNames(ctx, data, chatStorageRepo, client)
	default:
		log.Debugf("Skipping history sync type: %s", syncType.String())
		return nil
	}
}

// processConversationMessages processes and stores conversation messages from history sync
func processConversationMessages(ctx context.Context, data *waHistorySync.HistorySync, chatStorageRepo domainChatStorage.IChatStorageRepository, client *whatsmeow.Client) error {
	conversations := data.GetConversations()
	log.Infof("Processing %d conversations from history sync", len(conversations))

	// Calculate cutoff time based on configuration
	var cutoffTime time.Time
	if config.HistorySyncMaxDays > 0 {
		cutoffTime = time.Now().AddDate(0, 0, -int(config.HistorySyncMaxDays))
		log.Infof("History sync filtering: only messages after %s will be processed (max %d days)",
			cutoffTime.Format("2006-01-02"), config.HistorySyncMaxDays)
	} else if config.HistorySyncMaxDays == -1 {
		log.Infof("History sync filtering: processing all available messages (no time limit)")
	}

	for _, conv := range conversations {
		rawChatJID := conv.GetID()
		if rawChatJID == "" {
			continue
		}

		// Parse JID to get proper format
		jid, err := types.ParseJID(rawChatJID)
		if err != nil {
			log.Warnf("Failed to parse JID %s: %v", rawChatJID, err)
			continue
		}

		// Normalize JID
		jid = NormalizeJIDFromLID(ctx, jid, client)
		chatJID := jid.String()

		displayName := conv.GetDisplayName()
		chatName := chatStorageRepo.GetChatNameWithPushName(jid, chatJID, "", displayName)
		ephemeralExpiration := conv.GetEphemeralExpiration()

		// Process messages in the conversation
		messages := conv.GetMessages()
		log.Debugf("Processing %d messages for chat %s", len(messages), chatJID)

		// Collect messages for batch processing
		var messageBatch []*domainChatStorage.Message
		var latestTimestamp time.Time

		for _, histMsg := range messages {
			if histMsg == nil || histMsg.Message == nil {
				continue
			}

			msg := histMsg.Message
			msgKey := msg.GetKey()
			if msgKey == nil {
				continue
			}

			messageID := msgKey.GetID()
			if messageID == "" {
				continue
			}

			content := utils.ExtractMessageTextFromProto(msg.GetMessage())
			mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := utils.ExtractMediaInfo(msg.GetMessage())

			if content == "" && mediaType == "" {
				continue
			}

			// Determine sender
			sender := ""
			isFromMe := msgKey.GetFromMe()
			if isFromMe {
				if client != nil && client.Store.ID != nil {
					sender = client.Store.ID.String()
				} else {
					log.Warnf("Skipping self-message %s: client ID unavailable", messageID)
					continue
				}
			} else {
				participant := msgKey.GetParticipant()
				if participant != "" {
					if senderJID, err := types.ParseJID(participant); err == nil {
						senderJID = NormalizeJIDFromLID(ctx, senderJID, client)
						sender = senderJID.String()
					} else if participant != "" {
						sender = participant
					} else {
						log.Warnf("Skipping message %s: empty participant", messageID)
						continue
					}
				} else {
					sender = jid.String()
				}
			}

			timestamp := time.Unix(int64(msg.GetMessageTimestamp()), 0)

			if config.HistorySyncMaxDays > 0 && timestamp.Before(cutoffTime) {
				continue
			}

			if timestamp.After(latestTimestamp) {
				latestTimestamp = timestamp
			}

			message := &domainChatStorage.Message{
				ID:            messageID,
				ChatJID:       chatJID,
				Sender:        sender,
				Content:       content,
				Timestamp:     timestamp,
				IsFromMe:      isFromMe,
				MediaType:     mediaType,
				Filename:      filename,
				URL:           url,
				MediaKey:      mediaKey,
				FileSHA256:    fileSHA256,
				FileEncSHA256: fileEncSHA256,
				FileLength:    fileLength,
			}

			messageBatch = append(messageBatch, message)
		}

		// Store or update the chat with latest message time
		if len(messageBatch) > 0 {
			chat := &domainChatStorage.Chat{
				JID:                 chatJID,
				Name:                chatName,
				LastMessageTime:     latestTimestamp,
				EphemeralExpiration: ephemeralExpiration,
			}

			if err := chatStorageRepo.StoreChat(chat); err != nil {
				log.Warnf("Failed to store chat %s: %v", chatJID, err)
				continue
			}

			if err := chatStorageRepo.StoreMessagesBatch(messageBatch); err != nil {
				log.Warnf("Failed to store messages batch for chat %s: %v", chatJID, err)
			} else {
				log.Debugf("Stored %d messages for chat %s", len(messageBatch), chatJID)
			}
		}
	}

	return nil
}

// processPushNames processes push names from history sync to update chat names
func processPushNames(ctx context.Context, data *waHistorySync.HistorySync, chatStorageRepo domainChatStorage.IChatStorageRepository, client *whatsmeow.Client) error {
	pushnames := data.GetPushnames()
	log.Debugf("Processing %d push names from history sync", len(pushnames))

	for _, pushname := range pushnames {
		rawJIDStr := pushname.GetID()
		name := pushname.GetPushname()

		if rawJIDStr == "" || name == "" {
			continue
		}

		jid, err := types.ParseJID(rawJIDStr)
		if err != nil {
			log.Warnf("Failed to parse JID %s in push names: %v", rawJIDStr, err)
			continue
		}
		jid = NormalizeJIDFromLID(ctx, jid, client)
		jidStr := jid.String()

		existingChat, err := chatStorageRepo.GetChat(jidStr)
		if err != nil || existingChat == nil {
			continue
		}

		if existingChat.Name != name {
			existingChat.Name = name
			if err := chatStorageRepo.StoreChat(existingChat); err != nil {
				log.Warnf("Failed to update chat name for %s: %v", jidStr, err)
			} else {
				log.Debugf("Updated chat name for %s to %s", jidStr, name)
			}
		}
	}

	return nil
}
