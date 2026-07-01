package usecase

import (
	"context"
	"fmt"

	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func (service serviceSend) SendContact(ctx context.Context, request domainSend.ContactRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendContact(ctx, request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	msgVCard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%v;;;\nFN:%v\nTEL;type=CELL;waid=%v:+%v\nEND:VCARD",
		request.ContactName, request.ContactName, request.ContactPhone, request.ContactPhone)
	msg := &waE2E.Message{ContactMessage: &waE2E.ContactMessage{
		DisplayName: proto.String(request.ContactName),
		Vcard:       proto.String(msgVCard),
	}}

	if request.BaseRequest.IsForwarded {
		msg.ContactMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		if msg.ContactMessage.ContextInfo == nil {
			msg.ContactMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.ContactMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	}

	content := "👤 " + request.ContactName

	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Contact sent to %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendLink(ctx context.Context, request domainSend.LinkRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendLink(ctx, request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	metadata, err := utils.GetMetaDataFromURL(request.Link)
	if err != nil {
		return response, err
	}

	if metadata.Width != nil && metadata.Height != nil {
		logrus.Debugf("Image dimensions: %dx%d", *metadata.Width, *metadata.Height)
	} else {
		logrus.Debugf("Image dimensions: Square image or dimensions not available")
	}

	msg := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text:          proto.String(fmt.Sprintf("%s\n%s", request.Caption, request.Link)),
		Title:         proto.String(metadata.Title),
		MatchedText:   proto.String(request.Link),
		Description:   proto.String(metadata.Description),
		JPEGThumbnail: metadata.ImageThumb,
	}}

	if request.BaseRequest.IsForwarded {
		msg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		if msg.ExtendedTextMessage.ContextInfo == nil {
			msg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.ExtendedTextMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	}

	if len(metadata.ImageThumb) > 0 && metadata.Height != nil && metadata.Width != nil {
		uploadedThumb, err := service.uploadMedia(ctx, whatsmeow.MediaLinkThumbnail, metadata.ImageThumb, dataWaRecipient)
		if err == nil {
			msg.ExtendedTextMessage.ThumbnailDirectPath = proto.String(uploadedThumb.DirectPath)
			msg.ExtendedTextMessage.ThumbnailSHA256 = uploadedThumb.FileSHA256
			msg.ExtendedTextMessage.ThumbnailEncSHA256 = uploadedThumb.FileEncSHA256
			msg.ExtendedTextMessage.MediaKey = uploadedThumb.MediaKey
			msg.ExtendedTextMessage.ThumbnailHeight = metadata.Height
			msg.ExtendedTextMessage.ThumbnailWidth = metadata.Width
		} else {
			logrus.Warnf("Failed to upload thumbnail: %v, continue without uploaded thumbnail", err)
		}
	}

	content := "🔗 " + request.Link
	if request.Caption != "" {
		content = "🔗 " + request.Caption
	}
	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Link sent to %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendLocation(ctx context.Context, request domainSend.LocationRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendLocation(ctx, request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	msg := &waE2E.Message{
		LocationMessage: &waE2E.LocationMessage{
			DegreesLatitude:  proto.Float64(utils.StrToFloat64(request.Latitude)),
			DegreesLongitude: proto.Float64(utils.StrToFloat64(request.Longitude)),
		},
	}

	if request.BaseRequest.IsForwarded {
		msg.LocationMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		if msg.LocationMessage.ContextInfo == nil {
			msg.LocationMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.LocationMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	}

	content := "📍 " + request.Latitude + ", " + request.Longitude

	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Send location success %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendPoll(ctx context.Context, request domainSend.PollRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendPoll(ctx, request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	content := "📊 " + request.Question

	msg := whatsapp.GetClient().BuildPollCreation(request.Question, request.Options, request.MaxAnswer)

	if request.BaseRequest.Duration != nil && *request.BaseRequest.Duration > 0 {
		if msg.PollCreationMessage.ContextInfo == nil {
			msg.PollCreationMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.PollCreationMessage.ContextInfo.Expiration = proto.Uint32(uint32(*request.BaseRequest.Duration))
	}

	ts, err := service.wrapSendMessage(ctx, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Send poll success %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendPollVote(ctx context.Context, request domainSend.PollVoteRequest) (response domainSend.GenericResponse, err error) {
	if err = validations.ValidateSendPollVote(ctx, request); err != nil {
		return response, err
	}
	recipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	// Reconstruct the original poll's MessageInfo from chat storage.
	stored, err := service.chatStorageRepo.GetMessageByID(request.PollMessageID)
	if err != nil {
		logrus.Errorf("Failed to look up poll message %s: %v", request.PollMessageID, err)
		return response, fmt.Errorf("failed to look up poll message %s", request.PollMessageID)
	}
	if stored == nil {
		return response, pkgError.ValidationError("poll message not found in storage; cannot vote on an unknown poll")
	}
	chatJID, err := types.ParseJID(stored.ChatJID)
	if err != nil {
		return response, pkgError.ValidationError("stored poll has an invalid chat JID")
	}
	senderJID, err := types.ParseJID(stored.Sender)
	if err != nil {
		return response, pkgError.ValidationError("stored poll has an invalid sender JID")
	}
	if recipient.String() != chatJID.String() {
		return response, pkgError.ValidationError("phone does not match the poll's chat; the vote must target the chat where the poll was created")
	}
	pollInfo := &types.MessageInfo{
		ID: request.PollMessageID,
		MessageSource: types.MessageSource{
			Chat:     chatJID,
			Sender:   senderJID,
			IsFromMe: stored.IsFromMe,
			IsGroup:  utils.IsGroupJID(stored.ChatJID),
		},
		Timestamp: stored.Timestamp,
	}

	msg, err := whatsapp.GetClient().BuildPollVote(ctx, pollInfo, request.OptionNames)
	if err != nil {
		return response, err
	}

	ts, err := service.wrapSendMessage(ctx, recipient, msg, "🗳️ poll vote")
	if err != nil {
		return response, err
	}
	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Send poll vote success %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendPresence(ctx context.Context, request domainSend.PresenceRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendPresence(ctx, request)
	if err != nil {
		return response, err
	}

	err = whatsapp.GetClient().SendPresence(ctx, types.Presence(request.Type))
	if err != nil {
		return response, err
	}

	response.MessageID = "presence"
	response.Status = fmt.Sprintf("Send presence success %s", request.Type)
	return response, nil
}

func (service serviceSend) SendChatPresence(ctx context.Context, request domainSend.ChatPresenceRequest) (response domainSend.GenericResponse, err error) {
	err = validations.ValidateSendChatPresence(ctx, request)
	if err != nil {
		return response, err
	}

	userJid, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.Phone)
	if err != nil {
		return response, err
	}

	var presenceType types.ChatPresence
	var messageID string
	var statusMessage string

	switch request.Action {
	case "start":
		presenceType = types.ChatPresenceComposing
		messageID = "chat-presence-start"
		statusMessage = fmt.Sprintf("Send chat presence start typing success %s", request.Phone)
	case "stop":
		presenceType = types.ChatPresencePaused
		messageID = "chat-presence-stop"
		statusMessage = fmt.Sprintf("Send chat presence stop typing success %s", request.Phone)
	default:
		return response, fmt.Errorf("invalid action: %s. Must be 'start' or 'stop'", request.Action)
	}

	err = whatsapp.GetClient().SendChatPresence(ctx, userJid, presenceType, types.ChatPresenceMedia(""))
	if err != nil {
		return response, err
	}

	response.MessageID = messageID
	response.Status = statusMessage
	return response, nil
}
