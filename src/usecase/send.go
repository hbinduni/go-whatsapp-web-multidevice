// Package usecase contains business logic for WhatsApp message sending.
// The send functionality is split across multiple files:
//   - send.go: Service struct, constructor, shared helpers, SendText
//   - send_media.go: Image, Video, Audio, File, Sticker sending
//   - send_misc.go: Contact, Link, Location, Poll, Presence sending
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type serviceSend struct {
	appService      app.IAppUsecase
	chatStorageRepo domainChatStorage.IChatStorageRepository
}

func NewSendService(appService app.IAppUsecase, chatStorageRepo domainChatStorage.IChatStorageRepository) domainSend.ISendUsecase {
	return &serviceSend{
		appService:      appService,
		chatStorageRepo: chatStorageRepo,
	}
}

// wrapSendMessage wraps the message sending process with message ID saving
func (service serviceSend) wrapSendMessage(ctx context.Context, recipient types.JID, msg *waE2E.Message, content string) (whatsmeow.SendResponse, error) {
	ts, err := whatsapp.GetClientFromContext(ctx).SendMessage(ctx, recipient, msg)
	if err != nil {
		return whatsmeow.SendResponse{}, err
	}

	// Store the sent message using chatstorage
	senderJID := ""
	if whatsapp.GetClientFromContext(ctx).Store.ID != nil {
		senderJID = whatsapp.GetClientFromContext(ctx).Store.ID.String()
	}

	// Store message asynchronously with timeout
	go func() {
		storeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := service.chatStorageRepo.StoreSentMessageWithContext(storeCtx, ts.ID, senderJID, recipient.String(), content, ts.Timestamp); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				logrus.Warn("Timeout storing sent message")
			} else {
				logrus.Warnf("Failed to store sent message: %v", err)
			}
		}
	}()

	return ts, nil
}

func (service serviceSend) SendText(ctx context.Context, request domainSend.MessageRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendMessage(ctx, request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClientFromContext(ctx), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	// Create base message
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        proto.String(request.Message),
			ContextInfo: &waE2E.ContextInfo{},
		},
	}

	// Add forwarding context if IsForwarded is true
	if request.BaseRequest.IsForwarded {
		msg.ExtendedTextMessage.ContextInfo.IsForwarded = proto.Bool(true)
		msg.ExtendedTextMessage.ContextInfo.ForwardingScore = proto.Uint32(100)
	}

	// Set disappearing message duration if provided
	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		msg.ExtendedTextMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	} else {
		msg.ExtendedTextMessage.ContextInfo.Expiration = proto.Uint32(service.getDefaultEphemeralExpiration(request.BaseRequest.Phone))
	}

	parsedMentions := service.getMentionFromText(ctx, request.Message)
	if len(parsedMentions) > 0 {
		msg.ExtendedTextMessage.ContextInfo.MentionedJID = parsedMentions
	}

	// Reply message
	if request.ReplyMessageID != nil && *request.ReplyMessageID != "" {
		message, err := service.chatStorageRepo.GetMessageByID(*request.ReplyMessageID)
		if err != nil {
			logrus.Warnf("Error retrieving reply message ID %s: %v, continuing without reply context", *request.ReplyMessageID, err)
		} else if message != nil {
			participantJID := message.Sender

			ctxInfo := &waE2E.ContextInfo{
				StanzaID:    request.ReplyMessageID,
				Participant: proto.String(participantJID),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String(message.Content),
				},
			}

			if request.BaseRequest.IsForwarded {
				ctxInfo.IsForwarded = proto.Bool(true)
				ctxInfo.ForwardingScore = proto.Uint32(100)
			}

			if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
				ctxInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
			} else {
				ctxInfo.Expiration = proto.Uint32(service.getDefaultEphemeralExpiration(participantJID))
			}

			if len(parsedMentions) > 0 {
				ctxInfo.MentionedJID = parsedMentions
			}

			msg.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
				Text:        proto.String(request.Message),
				ContextInfo: ctxInfo,
			}
		} else {
			logrus.Warnf("Reply message ID %s not found in storage, continuing without reply context", *request.ReplyMessageID)
		}
	}

	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, request.Message)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Message sent to %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) getMentionFromText(ctx context.Context, messages string) (result []string) {
	mentions := utils.ContainsMention(messages)
	for _, mention := range mentions {
		if dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClientFromContext(ctx), mention); err == nil {
			result = append(result, dataWaRecipient.String())
		}
	}
	return result
}

func (service serviceSend) uploadMedia(ctx context.Context, mediaType whatsmeow.MediaType, media []byte, recipient types.JID) (uploaded whatsmeow.UploadResponse, err error) {
	if recipient.Server == types.NewsletterServer {
		uploaded, err = whatsapp.GetClientFromContext(ctx).UploadNewsletter(ctx, media, mediaType)
	} else {
		uploaded, err = whatsapp.GetClientFromContext(ctx).Upload(ctx, media, mediaType)
	}
	return uploaded, err
}

func (service serviceSend) getDefaultEphemeralExpiration(jid string) (expiration uint32) {
	expiration = 0
	if jid == "" {
		return expiration
	}

	chat, err := service.chatStorageRepo.GetChat(jid)
	if err != nil {
		return expiration
	}

	if chat != nil && chat.EphemeralExpiration != 0 {
		expiration = chat.EphemeralExpiration
	}

	return expiration
}
