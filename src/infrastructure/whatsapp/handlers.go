package whatsapp

import (
	"context"
	"fmt"
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

// hasTypedText checks if a message contains typed text content
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

func handleWebhookForward(ctx context.Context, evt *events.Message, client *whatsmeow.Client) {
	if protocolMessage := evt.Message.GetProtocolMessage(); protocolMessage != nil {
		if protocolMessage.GetType().String() == "EPHEMERAL_SYNC_RESPONSE" {
			log.Debugf("Skipping webhook for EPHEMERAL_SYNC_RESPONSE message")
			return
		}
	}

	if len(config.WhatsappWebhook) > 0 &&
		!strings.Contains(evt.Info.SourceString(), "broadcast") {
		go func(evt *events.Message, c *whatsmeow.Client) {
			if err := forwardMessageToWebhook(ctx, evt, c); err != nil {
				logrus.Error("Failed forward to webhook: ", err)
			}
		}(evt, client)
	}
}

func handleAppState(_ context.Context, evt *events.AppState) {
	log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
}

func handleGroupInfo(ctx context.Context, evt *events.GroupInfo, mc *ManagedClient) {
	hasChanges := len(evt.Join) > 0 || len(evt.Leave) > 0 || len(evt.Promote) > 0 || len(evt.Demote) > 0 ||
		evt.Name != nil || evt.Topic != nil || evt.Locked != nil || evt.Announce != nil

	if !hasChanges {
		return
	}

	if len(evt.Join) > 0 {
		log.Debugf("[%s] Group %s: %d users joined at %s", mc.Phone, evt.JID, len(evt.Join), evt.Timestamp)
	}
	if len(evt.Leave) > 0 {
		log.Debugf("[%s] Group %s: %d users left at %s", mc.Phone, evt.JID, len(evt.Leave), evt.Timestamp)
	}
	if len(evt.Promote) > 0 {
		log.Debugf("[%s] Group %s: %d users promoted at %s", mc.Phone, evt.JID, len(evt.Promote), evt.Timestamp)
	}
	if len(evt.Demote) > 0 {
		log.Debugf("[%s] Group %s: %d users demoted at %s", mc.Phone, evt.JID, len(evt.Demote), evt.Timestamp)
	}

	if len(config.WhatsappWebhook) > 0 {
		go func(e *events.GroupInfo, client *ManagedClient) {
			if err := forwardGroupInfoToWebhook(ctx, e, client.Client); err != nil {
				logrus.Errorf("[%s] Failed to forward group info event to webhook: %v", client.Phone, err)
			}
		}(evt, mc)
	}
}

// handler is the main event handler for WhatsApp events
// It receives the ManagedClient directly for multi-client architecture
func handler(ctx context.Context, rawEvt any, mc *ManagedClient) {
	if mc == nil {
		return
	}

	chatStorageRepo := mc.ChatStorageRepo

	switch evt := rawEvt.(type) {
	case *events.DeleteForMe:
		handleDeleteForMe(ctx, evt, mc)
	case *events.AppStateSyncComplete:
		handleAppStateSyncComplete(ctx, evt, mc)
	case *events.PairSuccess:
		handlePairSuccess(ctx, evt, mc)
	case *events.LoggedOut:
		handleLoggedOut(ctx, mc)
	case *events.Connected, *events.PushNameSetting:
		handleConnectionEvents(ctx, mc)
	case *events.Disconnected:
		handleDisconnected(ctx, evt, mc)
	case *events.StreamReplaced:
		handleStreamReplaced(ctx, mc)
	case *events.Message:
		handleMessage(ctx, evt, mc)
	case *events.Receipt:
		handleReceipt(ctx, evt, mc)
	case *events.Presence:
		handlePresence(ctx, evt, mc)
	case *events.HistorySync:
		handleHistorySync(ctx, evt, chatStorageRepo, mc.Client)
	case *events.AppState:
		handleAppState(ctx, evt) // Reuse existing
	case *events.GroupInfo:
		handleGroupInfo(ctx, evt, mc)
	}
}

func handleDeleteForMe(ctx context.Context, evt *events.DeleteForMe, mc *ManagedClient) {
	log.Debugf("[%s] DeleteForMe event: message %s for %s", mc.Phone, evt.MessageID, evt.SenderJID.String())

	if mc.ChatStorageRepo == nil {
		return
	}

	message, err := mc.ChatStorageRepo.GetMessageByID(evt.MessageID)
	if err != nil {
		log.Errorf("[%s] Failed to find message %s for deletion: %v", mc.Phone, evt.MessageID, err)
		return
	}

	if message == nil {
		log.Warnf("[%s] Message %s not found in database, skipping deletion", mc.Phone, evt.MessageID)
		return
	}

	if err := mc.ChatStorageRepo.DeleteMessage(evt.MessageID, message.ChatJID); err != nil {
		log.Errorf("[%s] Failed to delete message %s from database: %v", mc.Phone, evt.MessageID, err)
	}

	if len(config.WhatsappWebhook) > 0 {
		go func(client *whatsmeow.Client) {
			if err := forwardDeleteToWebhook(ctx, evt, message, client); err != nil {
				log.Errorf("[%s] Failed to forward delete event to webhook: %v", mc.Phone, err)
			}
		}(mc.Client)
	}
}

func handleAppStateSyncComplete(_ context.Context, evt *events.AppStateSyncComplete, mc *ManagedClient) {
	if mc.Client == nil {
		return
	}
	if len(mc.Client.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
		if err := mc.Client.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
			log.Warnf("[%s] Failed to send available presence: %v", mc.Phone, err)
		}
	}
}

func handlePairSuccess(ctx context.Context, evt *events.PairSuccess, mc *ManagedClient) {
	// Check if the scanned phone matches the configured client ID
	scannedPhone := evt.ID.User
	configuredPhone := normalizePhone(mc.Phone)

	if normalizePhone(scannedPhone) != configuredPhone {
		// Phone mismatch - reject the login
		logrus.Errorf("[%s] Phone mismatch! Configured: %s, Scanned: %s. Will disconnect in 5 seconds.",
			mc.Phone, configuredPhone, scannedPhone)

		// Broadcast error to clients immediately
		errorMsg := fmt.Sprintf("Phone mismatch: configured client %s but scanned with %s. "+
			"Disconnecting in 5 seconds. Please update WHATSAPP_CLIENTS configuration.",
			mc.Phone, scannedPhone)

		websocket.Broadcast <- websocket.BroadcastMessage{
			Code:    "LOGIN_REJECTED",
			Message: errorMsg,
			Result: map[string]any{
				"configured_phone": mc.Phone,
				"scanned_phone":    scannedPhone,
				"reason":           "phone_mismatch",
				"disconnect_in":    5,
			},
		}

		sse.BroadcastMessage(sse.EventLoginFailed, "LOGIN_REJECTED", errorMsg,
			map[string]any{
				"configured_phone": mc.Phone,
				"scanned_phone":    scannedPhone,
				"reason":           "phone_mismatch",
				"disconnect_in":    5,
			})

		// Wait 5 seconds before disconnecting to allow WhatsApp to complete its sync
		// This prevents incomplete session cleanup on the phone
		go func() {
			time.Sleep(5 * time.Second)

			logrus.Infof("[%s] Disconnecting mismatched device now", mc.Phone)

			// Logout the mismatched device
			if err := mc.Client.Logout(context.Background()); err != nil {
				logrus.Warnf("[%s] Failed to logout mismatched device: %v", mc.Phone, err)
			}
			mc.SetStatus(StatusLoggedOut)

			// Delete the device from database to clean up
			if mc.Client.Store != nil {
				if err := mc.Client.Store.Delete(context.Background()); err != nil {
					logrus.Warnf("[%s] Failed to delete mismatched device: %v", mc.Phone, err)
				}
			}

			// Notify that disconnect is complete
			websocket.Broadcast <- websocket.BroadcastMessage{
				Code:    "LOGIN_REJECTED_COMPLETE",
				Message: fmt.Sprintf("Mismatched device %s has been disconnected", scannedPhone),
				Result: map[string]any{
					"configured_phone": mc.Phone,
					"scanned_phone":    scannedPhone,
				},
			}

			sse.BroadcastMessage(sse.EventLoginFailed, "LOGIN_REJECTED_COMPLETE",
				fmt.Sprintf("Mismatched device %s has been disconnected", scannedPhone),
				map[string]any{
					"configured_phone": mc.Phone,
					"scanned_phone":    scannedPhone,
				})
		}()

		return
	}

	// Phone matches - proceed with successful login
	mc.SetStatus(StatusLoggedIn)
	mc.DeviceID = evt.ID.String()

	websocket.Broadcast <- websocket.BroadcastMessage{
		Code:    "LOGIN_SUCCESS",
		Message: fmt.Sprintf("[%s] Successfully paired with %s", mc.Phone, evt.ID.String()),
	}

	sse.BroadcastMessage(sse.EventLoginSuccess, "LOGIN_SUCCESS",
		fmt.Sprintf("[%s] Successfully paired with %s", mc.Phone, evt.ID.String()),
		map[string]any{
			"device_id": evt.ID.String(),
			"phone":     mc.Phone,
		})

	// Sync keys if available
	if mc.DB != nil && mc.KeysDB != nil {
		syncKeysDevice(ctx, mc.DB, mc.KeysDB)
	}
}

func handleLoggedOut(ctx context.Context, mc *ManagedClient) {
	mc.SetStatus(StatusLoggedOut)

	if err := forwardDisconnectToWebhook(ctx, fmt.Sprintf("logged_out:%s", mc.Phone), mc.Client); err != nil {
		logrus.Warnf("[%s] Failed to forward disconnect event to webhook: %v", mc.Phone, err)
	}

	websocket.Broadcast <- websocket.BroadcastMessage{
		Code:    "LOGOUT_COMPLETE",
		Message: fmt.Sprintf("[%s] Remote logout - client disconnected", mc.Phone),
		Result:  map[string]any{"phone": mc.Phone},
	}

	sse.BroadcastMessage(sse.EventLogoutComplete, "LOGOUT_COMPLETE",
		fmt.Sprintf("[%s] Remote logout completed", mc.Phone),
		map[string]any{"phone": mc.Phone})
}

func handleConnectionEvents(_ context.Context, mc *ManagedClient) {
	if mc.Client == nil {
		return
	}

	// Log connection event
	if mc.Client.IsLoggedIn() {
		mc.SetStatus(StatusLoggedIn)
		logrus.Infof("[%s] Client connected and logged in", mc.Phone)
	} else {
		mc.SetStatus(StatusConnected)
		logrus.Infof("[%s] Client connected (not logged in yet)", mc.Phone)
	}

	if len(mc.Client.Store.PushName) == 0 {
		return
	}

	if err := mc.Client.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
		log.Warnf("[%s] Failed to send available presence: %v", mc.Phone, err)
	}
}

func handleStreamReplaced(ctx context.Context, mc *ManagedClient) {
	mc.SetStatus(StatusDisconnected)

	if err := forwardDisconnectToWebhook(ctx, fmt.Sprintf("stream_replaced:%s", mc.Phone), mc.Client); err != nil {
		logrus.Warnf("[%s] Failed to forward disconnect event to webhook: %v", mc.Phone, err)
	}

	log.Warnf("[%s] Stream replaced, client disconnected", mc.Phone)
}

func handleDisconnected(_ context.Context, _ *events.Disconnected, mc *ManagedClient) {
	mc.SetStatus(StatusDisconnected)
	logrus.Warnf("[%s] Client disconnected from WhatsApp server, waiting for auto-reconnect...", mc.Phone)
}

func handleMessage(ctx context.Context, evt *events.Message, mc *ManagedClient) {
	log.Debugf("[%s] Received message %s from %s", mc.Phone, evt.Info.ID, evt.Info.SourceString())

	if mc.ChatStorageRepo != nil {
		if err := mc.ChatStorageRepo.CreateMessage(ctx, evt); err != nil {
			log.Errorf("[%s] Failed to store incoming message %s: %v", mc.Phone, evt.Info.ID, err)
		}
	}

	normalizedChatJID := NormalizeJIDFromLID(ctx, evt.Info.Chat, mc.Client)
	normalizedSenderJID := NormalizeJIDFromLID(ctx, evt.Info.Sender, mc.Client)

	messageContent := utils.ExtractMessageTextFromEvent(evt)
	mediaType, _, _, _, _, _, _ := utils.ExtractMediaInfo(evt.Message)

	if reactionMessage := evt.Message.GetReactionMessage(); reactionMessage != nil {
		handleReactionMessage(ctx, evt, reactionMessage, normalizedChatJID, normalizedSenderJID, mc)
		return
	}

	mediaPath, mediaMimeType, mediaFilename, mediaFileSize := downloadMediaIfEnabled(ctx, evt, mc, normalizedChatJID)

	// Broadcast via SSE with device info
	sse.BroadcastMessageReceivedWithDevice(
		mc.Phone,
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

	handleAutoMarkRead(ctx, evt, mc)
	handleAutoReply(ctx, evt, mc)
	handleWebhookForward(ctx, evt, mc.Client)
}

func handleReactionMessage(ctx context.Context, evt *events.Message, reactionMessage *waE2E.ReactionMessage, normalizedChatJID, normalizedSenderJID types.JID, mc *ManagedClient) {
	reactionEmoji := reactionMessage.GetText()
	targetMessageID := reactionMessage.GetKey().GetID()

	if mc.ChatStorageRepo != nil {
		reaction := &domainChatStorage.MessageReaction{
			MessageID: targetMessageID,
			ChatJID:   normalizedChatJID.String(),
			SenderJID: normalizedSenderJID.String(),
			Emoji:     reactionEmoji,
			Timestamp: evt.Info.Timestamp,
		}
		if err := mc.ChatStorageRepo.StoreReaction(ctx, reaction); err != nil {
			log.Errorf("[%s] Failed to store reaction: %v", mc.Phone, err)
		}
	}

	sse.BroadcastReactionReceivedWithDevice(
		mc.Phone,
		evt.Info.ID,
		normalizedChatJID.String(),
		normalizedSenderJID.String(),
		reactionEmoji,
		targetMessageID,
		evt.Info.Timestamp,
		evt.Info.IsFromMe,
	)

	handleWebhookForward(ctx, evt, mc.Client)
}

func downloadMediaIfEnabled(ctx context.Context, evt *events.Message, mc *ManagedClient, normalizedChatJID types.JID) (mediaPath, mediaMimeType, mediaFilename string, mediaFileSize int64) {
	if !config.WhatsappAutoDownloadMedia || mc.Client == nil {
		return
	}

	deviceID := mc.Phone
	if mc.Client.Store.ID != nil {
		deviceID = mc.Client.Store.ID.User
	}
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
		if extractedMedia, err := utils.ExtractMediaWithInfo(ctx, mc.Client, mediaFile, chatJID, messageID, deviceID); err != nil {
			log.Errorf("[%s] Failed to download %s: %v", mc.Phone, mediaLabel, err)
		} else {
			mediaPath = extractedMedia.MediaPath
			mediaMimeType = extractedMedia.MimeType
			mediaFilename = extractedMedia.Filename
			mediaFileSize = extractedMedia.FileSize
		}
	}

	return
}

func handleAutoMarkRead(_ context.Context, evt *events.Message, mc *ManagedClient) {
	if !config.WhatsappAutoMarkRead || evt.Info.IsFromMe {
		return
	}

	if mc.Client == nil {
		return
	}

	messageIDs := []types.MessageID{evt.Info.ID}
	timestamp := time.Now()

	if err := mc.Client.MarkRead(context.Background(), messageIDs, timestamp, evt.Info.Chat, evt.Info.Sender); err != nil {
		log.Warnf("[%s] Failed to mark message %s as read: %v", mc.Phone, evt.Info.ID, err)
	}
}

func handleAutoReply(ctx context.Context, evt *events.Message, mc *ManagedClient) {
	if config.WhatsappAutoReplyMessage == "" || mc.Client == nil {
		return
	}

	if utils.IsGroupJID(evt.Info.Chat.String()) || evt.Info.IsIncomingBroadcast() || evt.Info.IsFromMe {
		return
	}

	if evt.Info.Chat.Server != types.DefaultUserServer {
		return
	}

	source := evt.Info.SourceString()
	if strings.Contains(source, "broadcast") ||
		strings.HasSuffix(evt.Info.Chat.String(), "@broadcast") ||
		strings.HasPrefix(evt.Info.Chat.String(), "status@") {
		return
	}

	if !hasTypedText(evt.Message) {
		return
	}

	recipientJID := utils.FormatJID(evt.Info.Sender.String())

	response, err := mc.Client.SendMessage(
		ctx,
		recipientJID,
		&waE2E.Message{Conversation: proto.String(config.WhatsappAutoReplyMessage)},
	)

	if err != nil {
		log.Errorf("[%s] Failed to send auto-reply message: %v", mc.Phone, err)
		return
	}

	if mc.ChatStorageRepo != nil {
		senderJID := ""
		if mc.Client.Store.ID != nil {
			senderJID = mc.Client.Store.ID.String()
		}

		if err := mc.ChatStorageRepo.StoreSentMessageWithContext(
			ctx,
			response.ID,
			senderJID,
			recipientJID.String(),
			config.WhatsappAutoReplyMessage,
			response.Timestamp,
		); err != nil {
			log.Errorf("[%s] Failed to store auto-reply message: %v", mc.Phone, err)
		}
	}
}

func handleReceipt(ctx context.Context, evt *events.Receipt, mc *ManagedClient) {
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

	if status != "" && mc.ChatStorageRepo != nil {
		for _, messageID := range evt.MessageIDs {
			if err := mc.ChatStorageRepo.UpdateMessageStatus(ctx, messageID, status, evt.Timestamp); err != nil {
				logrus.Errorf("[%s] Failed to update status for message %s to %s: %v", mc.Phone, messageID, status, err)
			}
		}

		normalizedChatJID := NormalizeJIDFromLID(ctx, evt.Chat, mc.Client)
		sse.BroadcastReceiptWithDevice(mc.Phone, evt.MessageIDs, normalizedChatJID.String(), status, evt.Timestamp)
	}

	if len(config.WhatsappWebhook) > 0 && sendReceipt {
		go func(e *events.Receipt, client *whatsmeow.Client) {
			if err := forwardReceiptToWebhook(ctx, e, client); err != nil {
				logrus.Errorf("[%s] Failed to forward ack event to webhook: %v", mc.Phone, err)
			}
		}(evt, mc.Client)
	}
}

func handlePresence(ctx context.Context, evt *events.Presence, mc *ManagedClient) {
	normalizedJID := NormalizeJIDFromLID(ctx, evt.From, mc.Client)
	sse.BroadcastPresenceUpdateWithDevice(mc.Phone, normalizedJID.String(), !evt.Unavailable, evt.LastSeen)
}
