package whatsapp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/sse"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/websocket"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// handler is the main event handler for WhatsApp events
func handler(ctx context.Context, rawEvt any, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	switch evt := rawEvt.(type) {
	case *events.DeleteForMe:
		handleDeleteForMe(ctx, evt, chatStorageRepo)
	case *events.AppStateSyncComplete:
		handleAppStateSyncComplete(ctx, evt)
	case *events.PairSuccess:
		handlePairSuccess(ctx, evt)
	case *events.LoggedOut:
		handleLoggedOut(ctx, chatStorageRepo)
	case *events.Connected, *events.PushNameSetting:
		handleConnectionEvents(ctx)
	case *events.StreamReplaced:
		handleStreamReplaced(ctx)
	case *events.Message:
		handleMessage(ctx, evt, chatStorageRepo)
	case *events.Receipt:
		handleReceipt(ctx, evt, chatStorageRepo)
	case *events.Presence:
		handlePresence(ctx, evt)
	case *events.PrivacySettings:
		handlePrivacySettings(ctx, evt)
	case *events.HistorySync:
		handleHistorySync(ctx, evt, chatStorageRepo)
	case *events.AppState:
		handleAppState(ctx, evt)
	case *events.GroupInfo:
		handleGroupInfo(ctx, evt)
	}
}

func handleDeleteForMe(ctx context.Context, evt *events.DeleteForMe, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	log.Debugf("DeleteForMe event: message %s for %s", evt.MessageID, evt.SenderJID.String())

	// Find the message to get its chat JID
	message, err := chatStorageRepo.GetMessageByID(evt.MessageID)
	if err != nil {
		log.Errorf("Failed to find message %s for deletion: %v", evt.MessageID, err)
		return
	}

	if message == nil {
		log.Warnf("Message %s not found in database, skipping deletion", evt.MessageID)
		return
	}

	// Delete the message from database
	if err := chatStorageRepo.DeleteMessage(evt.MessageID, message.ChatJID); err != nil {
		log.Errorf("Failed to delete message %s from database: %v", evt.MessageID, err)
	}

	// Send webhook notification for delete event
	if len(config.WhatsappWebhook) > 0 {
		go func() {
			if err := forwardDeleteToWebhook(ctx, evt, message); err != nil {
				log.Errorf("Failed to forward delete event to webhook: %v", err)
			}
		}()
	}
}

func handleAppStateSyncComplete(_ context.Context, evt *events.AppStateSyncComplete) {
	client := GetClient()
	if client == nil {
		return
	}
	if len(client.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
		sendInitialPresence(client)
	}
}

// sendInitialPresence registers the device presence after connect. We always
// send presence so the server has our push name (otherwise contacts see "-"),
// but default to PresenceUnavailable: marking the device PresenceAvailable sets
// whatsmeow's sendActiveReceipts flag, makes this the "active" device, and
// causes WhatsApp to suppress new-message notifications on the linked phone.
// Set WHATSAPP_PRESENCE_AVAILABLE=true to opt into the online/available behavior.
func sendInitialPresence(client *whatsmeow.Client) {
	state := types.PresenceUnavailable
	if config.WhatsappPresenceAvailable {
		state = types.PresenceAvailable
	}
	if err := client.SendPresence(context.Background(), state); err != nil {
		log.Warnf("Failed to send %s presence: %v", state, err)
	}
}

func handlePairSuccess(ctx context.Context, evt *events.PairSuccess) {
	websocket.Broadcast <- websocket.BroadcastMessage{
		Code:    "LOGIN_SUCCESS",
		Message: fmt.Sprintf("Successfully pair with %s", evt.ID.String()),
	}

	// Broadcast via SSE
	sse.BroadcastMessage(sse.EventLoginSuccess, "LOGIN_SUCCESS",
		fmt.Sprintf("Successfully paired with %s", evt.ID.String()),
		map[string]any{
			"device_id": evt.ID.String(),
		})

	primaryDB, secondaryDB := getStoreContainers()
	syncKeysDevice(ctx, primaryDB, secondaryDB)
}

func handleLoggedOut(ctx context.Context, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	// Forward disconnect event to webhook BEFORE cleanup
	if err := forwardDisconnectToWebhook(ctx, "logged_out"); err != nil {
		logrus.Warnf("Failed to forward disconnect event to webhook: %v", err)
	}

	// Perform comprehensive cleanup
	handleRemoteLogout(ctx, chatStorageRepo)

	// Broadcast final notification
	websocket.Broadcast <- websocket.BroadcastMessage{
		Code:    "LOGOUT_COMPLETE",
		Message: "Remote logout cleanup completed - ready for new login",
		Result:  nil,
	}

	// Broadcast via SSE
	sse.BroadcastMessage(sse.EventLogoutComplete, "LOGOUT_COMPLETE",
		"Remote logout cleanup completed - ready for new login", nil)
}

func handleConnectionEvents(_ context.Context) {
	client := GetClient()
	if client == nil {
		return
	}
	if len(client.Store.PushName) == 0 {
		return
	}

	sendInitialPresence(client)
}

func handleStreamReplaced(ctx context.Context) {
	// Forward disconnect event to webhook before exit
	if err := forwardDisconnectToWebhook(ctx, "stream_replaced"); err != nil {
		logrus.Warnf("Failed to forward disconnect event to webhook: %v", err)
	}
	os.Exit(0)
}

func handleMessage(ctx context.Context, evt *events.Message, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	log.Debugf("Received message %s from %s", evt.Info.ID, evt.Info.SourceString())

	if err := chatStorageRepo.CreateMessage(ctx, evt); err != nil {
		log.Errorf("Failed to store incoming message %s: %v", evt.Info.ID, err)
	}

	// Normalize JIDs
	client := GetClient()
	normalizedChatJID := NormalizeJIDFromLID(ctx, evt.Info.Chat, client)
	normalizedSenderJID := NormalizeJIDFromLID(ctx, evt.Info.Sender, client)

	messageContent := utils.ExtractMessageTextFromEvent(evt)
	mediaType, _, _, _, _, _, _ := utils.ExtractMediaInfo(evt.Message)

	// Check if this is a reaction message
	if reactionMessage := evt.Message.GetReactionMessage(); reactionMessage != nil {
		handleReactionMessage(ctx, evt, reactionMessage, normalizedChatJID, normalizedSenderJID, chatStorageRepo)
		return
	}

	// Download media if configured
	mediaPath, mediaMimeType, mediaFilename, mediaFileSize := downloadMediaIfEnabled(ctx, evt, client, normalizedChatJID)

	// Broadcast via SSE
	sse.BroadcastMessageReceived(
		evt.Info.ID,
		normalizedChatJID.String(),
		normalizedSenderJID.String(),
		messageContent,
		evt.Info.Timestamp,
		evt.Info.IsFromMe,
		mediaType,
		mediaPath,
		mediaMimeType,
		mediaFilename,
		mediaFileSize,
	)

	handleAutoMarkRead(ctx, evt)
	handleAutoReply(ctx, evt, chatStorageRepo)
	handleWebhookForward(ctx, evt)
}

func handleReactionMessage(ctx context.Context, evt *events.Message, reactionMessage *waE2E.ReactionMessage, normalizedChatJID, normalizedSenderJID types.JID, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	reactionEmoji := reactionMessage.GetText()
	targetMessageID := reactionMessage.GetKey().GetID()

	// Store reaction in database
	reaction := &domainChatStorage.MessageReaction{
		MessageID: targetMessageID,
		ChatJID:   normalizedChatJID.String(),
		SenderJID: normalizedSenderJID.String(),
		Emoji:     reactionEmoji,
		Timestamp: evt.Info.Timestamp,
	}
	if err := chatStorageRepo.StoreReaction(ctx, reaction); err != nil {
		log.Errorf("Failed to store reaction: %v", err)
	} else {
		if reactionEmoji == "" {
			log.Debugf("📍 Reaction removed from message %s", targetMessageID)
		} else {
			log.Debugf("📍 Reaction '%s' stored for message %s", reactionEmoji, targetMessageID)
		}
	}

	// Broadcast via SSE
	sse.BroadcastReactionReceived(
		evt.Info.ID,
		normalizedChatJID.String(),
		normalizedSenderJID.String(),
		reactionEmoji,
		targetMessageID,
		evt.Info.Timestamp,
		evt.Info.IsFromMe,
	)

	handleWebhookForward(ctx, evt)
}

func downloadMediaIfEnabled(ctx context.Context, evt *events.Message, client *whatsmeow.Client, normalizedChatJID types.JID) (mediaPath, mediaMimeType, mediaFilename string, mediaFileSize int64) {
	if !config.WhatsappAutoDownloadMedia || client == nil {
		return
	}

	deviceID := client.Store.ID.User
	chatJID := normalizedChatJID.String()
	messageID := evt.Info.ID

	var mediaFile whatsmeow.DownloadableMessage
	var mediaLabel string

	if img := evt.Message.GetImageMessage(); img != nil {
		mediaFile = img
		mediaLabel = "image"
	} else if vid := evt.Message.GetVideoMessage(); vid != nil {
		mediaFile = vid
		mediaLabel = "video"
	} else if aud := evt.Message.GetAudioMessage(); aud != nil {
		mediaFile = aud
		mediaLabel = "audio"
	} else if doc := evt.Message.GetDocumentMessage(); doc != nil {
		mediaFile = doc
		mediaLabel = "document"
	} else if sticker := evt.Message.GetStickerMessage(); sticker != nil {
		mediaFile = sticker
		mediaLabel = "sticker"
	} else if ptv := evt.Message.GetPtvMessage(); ptv != nil {
		mediaFile = ptv
		mediaLabel = "video_note"
	}

	if mediaFile != nil {
		if extractedMedia, err := utils.ExtractMediaWithInfo(ctx, client, mediaFile, chatJID, messageID, deviceID); err != nil {
			log.Errorf("Failed to download %s: %v", mediaLabel, err)
		} else {
			mediaPath = extractedMedia.MediaPath
			mediaMimeType = extractedMedia.MimeType
			mediaFilename = extractedMedia.Filename
			mediaFileSize = extractedMedia.FileSize
			log.Debugf("📸 Media (%s) downloaded for SSE broadcast: %s", mediaLabel, mediaPath)
		}
	}

	return
}

func handleAutoMarkRead(_ context.Context, evt *events.Message) {
	if !config.WhatsappAutoMarkRead || evt.Info.IsFromMe {
		return
	}

	client := GetClient()
	if client == nil {
		return
	}

	messageIDs := []types.MessageID{evt.Info.ID}
	timestamp := time.Now()

	if err := client.MarkRead(context.Background(), messageIDs, timestamp, evt.Info.Chat, evt.Info.Sender); err != nil {
		log.Warnf("Failed to mark message %s as read: %v", evt.Info.ID, err)
	} else {
		log.Debugf("Marked message %s as read", evt.Info.ID)
	}
}

func handleAutoReply(ctx context.Context, evt *events.Message, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	if config.WhatsappAutoReplyMessage == "" {
		return
	}

	client := GetClient()
	if client == nil {
		return
	}

	// Skip groups, broadcasts, and self messages
	if utils.IsGroupJID(evt.Info.Chat.String()) || evt.Info.IsIncomingBroadcast() || evt.Info.IsFromMe {
		return
	}

	if evt.Info.Chat.Server != types.DefaultUserServer {
		return
	}

	// Skip broadcast/status contexts
	source := evt.Info.SourceString()
	if strings.Contains(source, "broadcast") ||
		strings.HasSuffix(evt.Info.Chat.String(), "@broadcast") ||
		strings.HasPrefix(evt.Info.Chat.String(), "status@") {
		return
	}

	// Require actual typed text
	if !hasTypedText(evt.Message) {
		return
	}

	recipientJID := utils.FormatJID(evt.Info.Sender.String())

	response, err := client.SendMessage(
		ctx,
		recipientJID,
		&waE2E.Message{Conversation: proto.String(config.WhatsappAutoReplyMessage)},
	)

	if err != nil {
		log.Errorf("Failed to send auto-reply message: %v", err)
		return
	}

	// Store the auto-reply message
	if chatStorageRepo != nil {
		senderJID := ""
		if client.Store.ID != nil {
			senderJID = client.Store.ID.String()
		}

		if err := chatStorageRepo.StoreSentMessageWithContext(
			ctx,
			response.ID,
			senderJID,
			recipientJID.String(),
			config.WhatsappAutoReplyMessage,
			response.Timestamp,
		); err != nil {
			log.Errorf("Failed to store auto-reply message in chat storage: %v", err)
		} else {
			log.Debugf("Auto-reply message %s stored successfully in chat storage", response.ID)
		}
	}
}

func hasTypedText(msg *waE2E.Message) bool {
	innerMsg := msg
	for i := 0; i < 3; i++ {
		if vm := innerMsg.GetViewOnceMessage(); vm != nil && vm.GetMessage() != nil {
			innerMsg = vm.GetMessage()
			continue
		}
		if em := innerMsg.GetEphemeralMessage(); em != nil && em.GetMessage() != nil {
			innerMsg = em.GetMessage()
			continue
		}
		if vm2 := innerMsg.GetViewOnceMessageV2(); vm2 != nil && vm2.GetMessage() != nil {
			innerMsg = vm2.GetMessage()
			continue
		}
		if vm2e := innerMsg.GetViewOnceMessageV2Extension(); vm2e != nil && vm2e.GetMessage() != nil {
			innerMsg = vm2e.GetMessage()
			continue
		}
		break
	}

	if conv := innerMsg.GetConversation(); conv != "" {
		return true
	}
	if ext := innerMsg.GetExtendedTextMessage(); ext != nil && ext.GetText() != "" {
		return true
	}
	if protoMsg := innerMsg.GetProtocolMessage(); protoMsg != nil {
		if edited := protoMsg.GetEditedMessage(); edited != nil {
			if ext := edited.GetExtendedTextMessage(); ext != nil && ext.GetText() != "" {
				return true
			}
			if conv := edited.GetConversation(); conv != "" {
				return true
			}
		}
	}
	return false
}

func handleWebhookForward(ctx context.Context, evt *events.Message) {
	if protocolMessage := evt.Message.GetProtocolMessage(); protocolMessage != nil {
		if protocolMessage.GetType().String() == "EPHEMERAL_SYNC_RESPONSE" {
			log.Debugf("Skipping webhook for EPHEMERAL_SYNC_RESPONSE message")
			return
		}
	}

	if len(config.WhatsappWebhook) > 0 &&
		!strings.Contains(evt.Info.SourceString(), "broadcast") {
		go func(evt *events.Message) {
			if err := forwardMessageToWebhook(ctx, evt); err != nil {
				logrus.Error("Failed forward to webhook: ", err)
			}
		}(evt)
	}
}

func handleReceipt(ctx context.Context, evt *events.Receipt, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	sendReceipt := false
	var status string

	switch evt.Type {
	case types.ReceiptTypeRead, types.ReceiptTypeReadSelf:
		sendReceipt = true
		status = "read"
	case types.ReceiptTypeDelivered:
		sendReceipt = true
		status = "delivered"
	case types.ReceiptTypePlayed:
		sendReceipt = true
		status = "played"
	}

	// Update message status in database
	if status != "" && chatStorageRepo != nil {
		for _, messageID := range evt.MessageIDs {
			if err := chatStorageRepo.UpdateMessageStatus(ctx, messageID, status, evt.Timestamp); err != nil {
				logrus.Errorf("Failed to update status for message %s to %s: %v", messageID, status, err)
			} else {
				logrus.Debugf("Updated message %s status to %s", messageID, status)
			}
		}

		client := GetClient()
		normalizedChatJID := NormalizeJIDFromLID(ctx, evt.Chat, client)
		sse.BroadcastReceipt(evt.MessageIDs, normalizedChatJID.String(), status, evt.Timestamp)
	}

	// Forward to webhook
	if len(config.WhatsappWebhook) > 0 && sendReceipt {
		go func(e *events.Receipt) {
			if err := forwardReceiptToWebhook(ctx, e); err != nil {
				logrus.Errorf("Failed to forward ack event to webhook: %v", err)
			}
		}(evt)
	}
}

func handlePresence(ctx context.Context, evt *events.Presence) {
	client := GetClient()
	normalizedJID := NormalizeJIDFromLID(ctx, evt.From, client)
	sse.BroadcastPresenceUpdate(normalizedJID.String(), !evt.Unavailable, evt.LastSeen)
}

func handlePrivacySettings(_ context.Context, evt *events.PrivacySettings) {
	settings := evt.NewSettings
	sse.BroadcastPrivacyUpdate(map[string]any{
		"group_add":     string(settings.GroupAdd),
		"last_seen":     string(settings.LastSeen),
		"status":        string(settings.Status),
		"profile":       string(settings.Profile),
		"read_receipts": string(settings.ReadReceipts),
		"call_add":      string(settings.CallAdd),
		"online":        string(settings.Online),
		"messages":      string(settings.Messages),
		"defense":       string(settings.Defense),
		"stickers":      string(settings.Stickers),
	})
}

func handleAppState(_ context.Context, evt *events.AppState) {
	log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
}

func handleGroupInfo(ctx context.Context, evt *events.GroupInfo) {
	hasChanges := len(evt.Join) > 0 || len(evt.Leave) > 0 || len(evt.Promote) > 0 || len(evt.Demote) > 0 ||
		evt.Name != nil || evt.Topic != nil || evt.Locked != nil || evt.Announce != nil

	if !hasChanges {
		return
	}

	if len(evt.Join) > 0 {
		log.Debugf("Group %s: %d users joined at %s", evt.JID, len(evt.Join), evt.Timestamp)
	}
	if len(evt.Leave) > 0 {
		log.Debugf("Group %s: %d users left at %s", evt.JID, len(evt.Leave), evt.Timestamp)
	}
	if len(evt.Promote) > 0 {
		log.Debugf("Group %s: %d users promoted at %s", evt.JID, len(evt.Promote), evt.Timestamp)
	}
	if len(evt.Demote) > 0 {
		log.Debugf("Group %s: %d users demoted at %s", evt.JID, len(evt.Demote), evt.Timestamp)
	}

	if len(config.WhatsappWebhook) > 0 {
		go func(e *events.GroupInfo) {
			if err := forwardGroupInfoToWebhook(ctx, e); err != nil {
				logrus.Errorf("Failed to forward group info event to webhook: %v", err)
			}
		}(evt)
	}
}
