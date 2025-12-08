package utils

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/storage"
	"go.mau.fi/whatsmeow"
)

var knownDocumentMIMEByExtension = map[string]string{
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

var knownDocumentExtensionByMIME map[string]string

// pathSegmentSanitizer is precompiled regex for sanitizing path segments
// Only allows alphanumeric, underscore, and hyphen characters
var pathSegmentSanitizer = regexp.MustCompile(`[^A-Za-z0-9_\-]`)

func init() {
	knownDocumentExtensionByMIME = make(map[string]string, len(knownDocumentMIMEByExtension))
	for ext, mimeType := range knownDocumentMIMEByExtension {
		knownDocumentExtensionByMIME[strings.ToLower(mimeType)] = ext
	}
}

func resolveKnownDocumentMIME(ext string) (string, bool) {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	mimeType, ok := knownDocumentMIMEByExtension[ext]
	return mimeType, ok
}

func resolveKnownDocumentExtension(mimeType string) (string, bool) {
	ext, ok := knownDocumentExtensionByMIME[strings.ToLower(mimeType)]
	return ext, ok
}

// KnownDocumentMIMEByExtension returns a known MIME type for a given Office document extension.
func KnownDocumentMIMEByExtension(ext string) (string, bool) {
	return resolveKnownDocumentMIME(ext)
}

// KnownDocumentExtensionByMIME returns a known Office document extension for a given MIME type.
func KnownDocumentExtensionByMIME(mimeType string) (string, bool) {
	return resolveKnownDocumentExtension(mimeType)
}

func determineMediaExtension(originalFilename, mimeType string) string {
	if originalFilename != "" {
		if ext := filepath.Ext(originalFilename); ext != "" {
			return ext
		}
	}

	if ext, ok := resolveKnownDocumentExtension(mimeType); ok {
		return ext
	}

	if ext, err := mime.ExtensionsByType(mimeType); err == nil && len(ext) > 0 {
		return ext[0]
	}

	if parts := strings.Split(mimeType, "/"); len(parts) > 1 {
		return "." + parts[len(parts)-1]
	}

	return ""
}

// ExtractMessageTextFromProto extracts text content from a WhatsApp proto message
func ExtractMessageTextFromProto(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}

	// Check for regular text message
	if text := msg.GetConversation(); text != "" {
		return text
	}

	// Check for extended text message (with link preview, etc.)
	if extendedText := msg.GetExtendedTextMessage(); extendedText != nil {
		return extendedText.GetText()
	}

	// Check for image with caption
	if img := msg.GetImageMessage(); img != nil && img.GetCaption() != "" {
		return img.GetCaption()
	}

	// Check for video with caption
	if vid := msg.GetVideoMessage(); vid != nil && vid.GetCaption() != "" {
		return vid.GetCaption()
	}

	// Check for document with caption
	if doc := msg.GetDocumentMessage(); doc != nil && doc.GetCaption() != "" {
		return doc.GetCaption()
	}

	// Check for buttons response message
	if buttonsResponse := msg.GetButtonsResponseMessage(); buttonsResponse != nil {
		return buttonsResponse.GetSelectedDisplayText()
	}

	// Check for list response message
	if listResponse := msg.GetListResponseMessage(); listResponse != nil {
		return listResponse.GetTitle()
	}

	// Check for template button reply
	if templateButtonReply := msg.GetTemplateButtonReplyMessage(); templateButtonReply != nil {
		return templateButtonReply.GetSelectedDisplayText()
	}

	return ""
}

// ExtractMessageTextFromEvent extracts text content from a WhatsApp event message with emojis
func ExtractMessageTextFromEvent(evt *events.Message) string {
	messageText := evt.Message.GetConversation()
	if extendedText := evt.Message.GetExtendedTextMessage(); extendedText != nil {
		messageText = extendedText.GetText()
	} else if protocolMessage := evt.Message.GetProtocolMessage(); protocolMessage != nil {
		if editedMessage := protocolMessage.GetEditedMessage(); editedMessage != nil {
			if extendedText := editedMessage.GetExtendedTextMessage(); extendedText != nil {
				messageText = extendedText.GetText()
			}
		}
	} else if imageMessage := evt.Message.GetImageMessage(); imageMessage != nil {
		messageText = imageMessage.GetCaption()
		if messageText == "" {
			messageText = "🖼️ Image"
		} else {
			messageText = "🖼️ " + messageText
		}
	} else if documentMessage := evt.Message.GetDocumentMessage(); documentMessage != nil {
		messageText = documentMessage.GetCaption()
		if messageText == "" {
			messageText = "📄 Document"
		} else {
			messageText = "📄 " + messageText
		}
	} else if videoMessage := evt.Message.GetVideoMessage(); videoMessage != nil {
		messageText = videoMessage.GetCaption()
		if messageText == "" {
			messageText = "🎥 Video"
		} else {
			messageText = "🎥 " + messageText
		}
	} else if liveLocationMessage := evt.Message.GetLiveLocationMessage(); liveLocationMessage != nil {
		messageText = liveLocationMessage.GetCaption()
		if messageText == "" {
			messageText = "📍 Live Location"
		} else {
			messageText = "📍 " + messageText
		}
	} else if locationMessage := evt.Message.GetLocationMessage(); locationMessage != nil {
		messageText = locationMessage.GetName()
		if messageText == "" {
			messageText = "📍 Location"
		} else {
			messageText = "📍 " + messageText
		}
	} else if stickerMessage := evt.Message.GetStickerMessage(); stickerMessage != nil {
		messageText = "🎨 Sticker"
		if stickerMessage.GetIsAnimated() {
			messageText = "✨ Animated Sticker"
		}
		if stickerMessage.GetAccessibilityLabel() != "" {
			messageText += " - " + stickerMessage.GetAccessibilityLabel()
		}
	} else if contactMessage := evt.Message.GetContactMessage(); contactMessage != nil {
		messageText = contactMessage.GetDisplayName()
		if messageText == "" {
			messageText = "👤 Contact"
		} else {
			messageText = "👤 " + messageText
		}
	} else if listMessage := evt.Message.GetListMessage(); listMessage != nil {
		messageText = listMessage.GetTitle()
		if messageText == "" {
			messageText = "📝 List"
		} else {
			messageText = "📝 " + messageText
		}
	} else if orderMessage := evt.Message.GetOrderMessage(); orderMessage != nil {
		messageText = orderMessage.GetOrderTitle()
		if messageText == "" {
			messageText = "🛍️ Order"
		} else {
			messageText = "🛍️ " + messageText
		}
	} else if paymentMessage := evt.Message.GetPaymentInviteMessage(); paymentMessage != nil {
		messageText = paymentMessage.GetServiceType().String()
		if messageText == "" {
			messageText = "💳 Payment"
		} else {
			messageText = "💳 " + messageText
		}
	} else if audioMessage := evt.Message.GetAudioMessage(); audioMessage != nil {
		messageText = "🎧 Audio"
		if audioMessage.GetPTT() {
			messageText = "🎤 Voice Message"
		}
	} else if pollMessageV3 := evt.Message.GetPollCreationMessageV3(); pollMessageV3 != nil {
		messageText = pollMessageV3.GetName()
		if messageText == "" {
			messageText = "📊 Poll"
		} else {
			messageText = "📊 " + messageText
		}
	} else if pollMessageV4 := evt.Message.GetPollCreationMessageV4(); pollMessageV4 != nil {
		if pollMessage := pollMessageV4.GetMessage(); pollMessage != nil {
			messageText = pollMessage.GetConversation()
		}
		if messageText == "" {
			messageText = "📊 Poll"
		} else {
			messageText = "📊 " + messageText
		}
	} else if pollMessageV5 := evt.Message.GetPollCreationMessageV5(); pollMessageV5 != nil {
		messageText = pollMessageV5.GetName()
		if messageText == "" {
			messageText = "📊 Poll"
		} else {
			messageText = "📊 " + messageText
		}
	}
	return messageText
}

// ExtractMediaInfo extracts media information from a WhatsApp message
func ExtractMediaInfo(msg *waE2E.Message) (mediaType string, filename string, url string, mediaKey []byte, fileSHA256 []byte, fileEncSHA256 []byte, fileLength uint64) {
	if msg == nil {
		return "", "", "", nil, nil, nil, 0
	}

	// Check for image message
	if img := msg.GetImageMessage(); img != nil {
		filename = GenerateMediaFilename("image", "jpg", img.GetCaption())
		return "image", filename,
			img.GetURL(), img.GetMediaKey(), img.GetFileSHA256(),
			img.GetFileEncSHA256(), img.GetFileLength()
	}

	// Check for video message
	if vid := msg.GetVideoMessage(); vid != nil {
		filename = GenerateMediaFilename("video", "mp4", vid.GetCaption())
		return "video", filename,
			vid.GetURL(), vid.GetMediaKey(), vid.GetFileSHA256(),
			vid.GetFileEncSHA256(), vid.GetFileLength()
	}

	// Check for PTV (video note) message - circular video messages
	if ptv := msg.GetPtvMessage(); ptv != nil {
		filename = GenerateMediaFilename("video_note", "mp4", ptv.GetCaption())
		return "video_note", filename,
			ptv.GetURL(), ptv.GetMediaKey(), ptv.GetFileSHA256(),
			ptv.GetFileEncSHA256(), ptv.GetFileLength()
	}

	// Check for audio message
	if aud := msg.GetAudioMessage(); aud != nil {
		extension := "ogg"
		if aud.GetPTT() {
			extension = "ogg" // Voice notes are typically ogg
		}
		filename = GenerateMediaFilename("audio", extension, "")
		return "audio", filename,
			aud.GetURL(), aud.GetMediaKey(), aud.GetFileSHA256(),
			aud.GetFileEncSHA256(), aud.GetFileLength()
	}

	// Check for document message
	if doc := msg.GetDocumentMessage(); doc != nil {
		filename = doc.GetFileName()
		if filename == "" {
			filename = GenerateMediaFilename("document", "", doc.GetTitle())
		}
		return "document", filename,
			doc.GetURL(), doc.GetMediaKey(), doc.GetFileSHA256(),
			doc.GetFileEncSHA256(), doc.GetFileLength()
	}

	// Check for sticker message
	if sticker := msg.GetStickerMessage(); sticker != nil {
		filename = GenerateMediaFilename("sticker", "webp", "")
		return "sticker", filename,
			sticker.GetURL(), sticker.GetMediaKey(), sticker.GetFileSHA256(),
			sticker.GetFileEncSHA256(), sticker.GetFileLength()
	}

	return "", "", "", nil, nil, nil, 0
}

// ExtractEphemeralExpiration extracts ephemeral expiration from a WhatsApp message
func ExtractEphemeralExpiration(msg *waE2E.Message) uint32 {
	if msg == nil {
		return 0
	}

	// Check extended text message
	if extendedText := msg.GetExtendedTextMessage(); extendedText != nil {
		if contextInfo := extendedText.GetContextInfo(); contextInfo != nil {
			return contextInfo.GetExpiration()
		}
	}

	// Check image message
	if img := msg.GetImageMessage(); img != nil {
		if contextInfo := img.GetContextInfo(); contextInfo != nil {
			return contextInfo.GetExpiration()
		}
	}

	// Check video message
	if vid := msg.GetVideoMessage(); vid != nil {
		if contextInfo := vid.GetContextInfo(); contextInfo != nil {
			return contextInfo.GetExpiration()
		}
	}

	// Check audio message
	if aud := msg.GetAudioMessage(); aud != nil {
		if contextInfo := aud.GetContextInfo(); contextInfo != nil {
			return contextInfo.GetExpiration()
		}
	}

	// Check document message
	if doc := msg.GetDocumentMessage(); doc != nil {
		if contextInfo := doc.GetContextInfo(); contextInfo != nil {
			return contextInfo.GetExpiration()
		}
	}

	// Check sticker message
	if sticker := msg.GetStickerMessage(); sticker != nil {
		if contextInfo := sticker.GetContextInfo(); contextInfo != nil {
			return contextInfo.GetExpiration()
		}
	}

	// Check protocol message
	if protocolMessage := msg.GetProtocolMessage(); protocolMessage != nil {
		if ephemeralExpiration := protocolMessage.GetEphemeralExpiration(); ephemeralExpiration != 0 {
			return ephemeralExpiration
		}
	}

	return 0
}

// GenerateMediaFilename creates a filename for media files
func GenerateMediaFilename(mediaType, extension, caption string) string {
	timestamp := time.Now().Format("20060102_150405")
	name := mediaType + "_" + timestamp

	if caption != "" {
		// Only keep alphanumeric, _, -
		re := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
		cleanCaption := re.ReplaceAllString(caption, "_")
		if len(cleanCaption) > 30 {
			cleanCaption = cleanCaption[:30]
		}
		name += "_" + cleanCaption
	}

	if extension != "" {
		name += "." + extension
	}
	return name
}

// ExtractPhoneNumber is a helper function to extract the phone number from a JID
func ExtractPhoneNumber(jid string) string {
	regex := regexp.MustCompile(`\d+`)
	// Find all matches of the pattern in the JID
	matches := regex.FindAllString(jid, -1)
	// The first match should be the phone number
	if len(matches) > 0 {
		return matches[0]
	}
	// If no matches are found, return an empty string
	return ""
}

// IsGroupJID is a helper function to check if the JID is from a group
func IsGroupJID(jid string) bool {
	return strings.Contains(jid, "@g.us")
}

// GetPlatformName returns the platform name based on device ID
func GetPlatformName(deviceID int) string {
	switch deviceID {
	case 0:
		return "UNKNOWN"
	case 1:
		return "CHROME"
	case 2:
		return "FIREFOX"
	case 3:
		return "IE"
	case 4:
		return "OPERA"
	case 5:
		return "SAFARI"
	case 6:
		return "EDGE"
	case 7:
		return "DESKTOP"
	case 8:
		return "IPAD"
	case 9:
		return "ANDROID_TABLET"
	case 10:
		return "OHANA"
	case 11:
		return "ALOHA"
	case 12:
		return "CATALINA"
	case 13:
		return "TCL_TV"
	default:
		return "UNKNOWN"
	}
}

// ParseJID parses a string into a JID
func ParseJID(arg string) (types.JID, error) {
	if len(arg) > 0 && arg[0] == '+' {
		arg = arg[1:]
	}
	if !strings.ContainsRune(arg, '@') {
		return types.NewJID(arg, types.DefaultUserServer), nil
	}

	recipient, err := types.ParseJID(arg)
	if err != nil {
		return recipient, fmt.Errorf("invalid JID %s: %v", arg, err)
	}

	if recipient.User == "" {
		return recipient, fmt.Errorf("invalid JID %v: no server specified", arg)
	}
	return recipient, nil
}

// FormatJID formats a JID string by removing any :number suffix
func FormatJID(jid string) types.JID {
	// Remove any :number suffix if present
	if idx := strings.LastIndex(jid, ":"); idx != -1 && strings.Contains(jid, "@s.whatsapp.net") {
		jid = jid[:idx] + jid[strings.Index(jid, "@s.whatsapp.net"):]
	}
	formattedJID, err := ParseJID(jid)
	if err != nil {
		return types.JID{}
	}
	return formattedJID
}

// ExtractedMedia represents extracted media information
type ExtractedMedia struct {
	MediaPath string `json:"media_path"`
	MimeType  string `json:"mime_type"`
	Caption   string `json:"caption"`
	FileSize  int64  `json:"file_size"`
}

// ExtractMedia is a helper function to extract media from whatsapp
func ExtractMedia(ctx context.Context, client *whatsmeow.Client, mediaFile whatsmeow.DownloadableMessage) (extractedMedia ExtractedMedia, err error) {
	return ExtractMediaWithInfo(ctx, client, mediaFile, "", "", "")
}

// ExtractMediaWithInfo is a helper function to extract media from whatsapp with additional path organization
func ExtractMediaWithInfo(ctx context.Context, client *whatsmeow.Client, mediaFile whatsmeow.DownloadableMessage, chatJID, messageID, deviceID string) (extractedMedia ExtractedMedia, err error) {
	if mediaFile == nil {
		logrus.Info("Skip download because data is nil")
		return extractedMedia, nil
	}

	data, err := client.Download(ctx, mediaFile)
	if err != nil {
		return extractedMedia, err
	}

	// Validate file size before writing to disk
	maxFileSize := config.WhatsappSettingMaxDownloadSize
	if int64(len(data)) > maxFileSize {
		return extractedMedia, fmt.Errorf("file size exceeds the maximum limit of %d bytes", maxFileSize)
	}

	var originalFilename string

	switch media := mediaFile.(type) {
	case *waE2E.ImageMessage:
		extractedMedia.MimeType = media.GetMimetype()
		extractedMedia.Caption = media.GetCaption()
	case *waE2E.AudioMessage:
		extractedMedia.MimeType = media.GetMimetype()
	case *waE2E.VideoMessage:
		extractedMedia.MimeType = media.GetMimetype()
		extractedMedia.Caption = media.GetCaption()
	case *waE2E.StickerMessage:
		extractedMedia.MimeType = media.GetMimetype()
	case *waE2E.DocumentMessage:
		extractedMedia.MimeType = media.GetMimetype()
		extractedMedia.Caption = media.GetCaption()
		originalFilename = media.GetFileName()
	}

	extension := determineMediaExtension(originalFilename, extractedMedia.MimeType)

	// Build path with device ID, chat JID, and message ID if provided
	var filename string
	if deviceID != "" && chatJID != "" && messageID != "" {
		// Whitelist-only sanitize: keep [A-Za-z0-9_-]
		dev := pathSegmentSanitizer.ReplaceAllString(deviceID, "_")
		jid := pathSegmentSanitizer.ReplaceAllString(chatJID, "_")
		msg := pathSegmentSanitizer.ReplaceAllString(messageID, "_")
		if dev == "" || jid == "" || msg == "" {
			return ExtractedMedia{}, fmt.Errorf("invalid media path segments")
		}
		// deviceID/jid/messageID + ext
		key := filepath.ToSlash(filepath.Join(dev, jid, msg)) + extension
		filename = key
		logrus.Debugf("Organized path: %s", filename)
	} else {
		// Fallback to timestamp + random ID if no message info provided (for backward compatibility)
		shortID := uuid.NewString()[:8]
		filename = fmt.Sprintf("%d-%s%s", time.Now().Unix(), shortID, extension)
	}

	// Use the storage interface to save the media
	mediaStorage := storage.GetStorage()
	if mediaStorage == nil {
		return extractedMedia, fmt.Errorf("media storage not initialized")
	}

	logrus.Debugf("📤 Attempting to save media: filename=%s, size=%d bytes", filename, len(data))
	path, err := mediaStorage.Save(ctx, data, filename)
	if err != nil {
		logrus.Errorf("❌ Failed to save media: %v", err)
		return extractedMedia, fmt.Errorf("failed to save media: %w", err)
	}
	logrus.Debugf("✅ Successfully saved media: path=%s", path)

	extractedMedia.MediaPath = mediaStorage.GetURL(path)
	extractedMedia.FileSize = int64(len(data)) // Set file size from downloaded data
	logrus.Debugf("🔗 Media URL: %s", extractedMedia.MediaPath)
	return extractedMedia, nil
}

// SanitizePhone sanitizes phone number by adding appropriate WhatsApp suffix
const maxPhoneNumberLength = 15 // Maximum digits in a phone number

func SanitizePhone(phone *string) {
	if phone != nil && len(*phone) > 0 && !strings.Contains(*phone, "@") {
		if len(*phone) <= maxPhoneNumberLength {
			*phone = fmt.Sprintf("%s%s", *phone, config.WhatsappTypeUser)
		} else {
			*phone = fmt.Sprintf("%s%s", *phone, config.WhatsappTypeGroup)
		}
	}
}

// IsOnWhatsapp checks if a number is registered on WhatsApp
func IsOnWhatsapp(client *whatsmeow.Client, jid string) bool {
	// only check if the jid a user with @s.whatsapp.net
	if strings.Contains(jid, "@s.whatsapp.net") {
		data, err := client.IsOnWhatsApp(context.Background(), []string{jid})
		if err != nil {
			logrus.Error("Failed to check if user is on whatsapp: ", err)
			return false
		}

		for _, v := range data {
			if !v.IsIn {
				return false
			}
		}
	}

	return true
}

// ValidateJidWithLogin validates JID with login check
func ValidateJidWithLogin(client *whatsmeow.Client, jid string) (types.JID, error) {
	MustLogin(client)

	if config.WhatsappAccountValidation && !IsOnWhatsapp(client, jid) {
		return types.JID{}, pkgError.InvalidJID(fmt.Sprintf("Phone %s is not on whatsapp", jid))
	}

	return ParseJID(jid)
}

// MustLogin ensures the WhatsApp client is logged in
func MustLogin(client *whatsmeow.Client) {
	if client == nil {
		panic(pkgError.InternalServerError("Whatsapp client is not initialized"))
	}
	if !client.IsConnected() {
		panic(pkgError.ErrNotConnected)
	} else if !client.IsLoggedIn() {
		panic(pkgError.ErrNotLoggedIn)
	}
}

// Internal message types for event handling
type EvtMessage struct {
	Text          string `json:"text"`
	ID            string `json:"id"`
	RepliedId     string `json:"replied_id"`
	QuotedMessage string `json:"quoted_message"`
}

type EvtReaction struct {
	Message string `json:"message"`
	ID      string `json:"id"`
}

// GetMessageDigestOrSignature generates HMAC signature for message
func GetMessageDigestOrSignature(msg, key []byte) (string, error) {
	mac := hmac.New(sha256.New, key)
	_, err := mac.Write(msg)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// BuildEventMessage builds event message structure
func BuildEventMessage(evt *events.Message) (message EvtMessage) {
	message.Text = evt.Message.GetConversation()
	message.ID = evt.Info.ID

	if extendedMessage := evt.Message.GetExtendedTextMessage(); extendedMessage != nil {
		message.Text = extendedMessage.GetText()
		message.RepliedId = extendedMessage.ContextInfo.GetStanzaID()
		message.QuotedMessage = extendedMessage.ContextInfo.GetQuotedMessage().GetConversation()
	} else if protocolMessage := evt.Message.GetProtocolMessage(); protocolMessage != nil {
		if editedMessage := protocolMessage.GetEditedMessage(); editedMessage != nil {
			if extendedText := editedMessage.GetExtendedTextMessage(); extendedText != nil {
				message.Text = extendedText.GetText()
				message.RepliedId = extendedText.ContextInfo.GetStanzaID()
				message.QuotedMessage = extendedText.ContextInfo.GetQuotedMessage().GetConversation()
			}
		}
	}

	return message
}

// BuildEventReaction builds event reaction structure
func BuildEventReaction(evt *events.Message) (waReaction EvtReaction) {
	if reactionMessage := evt.Message.GetReactionMessage(); reactionMessage != nil {
		waReaction.Message = reactionMessage.GetText()
		waReaction.ID = reactionMessage.GetKey().GetID()
	}
	return waReaction
}

// BuildForwarded checks if message is forwarded
func BuildForwarded(evt *events.Message) bool {
	if extendedText := evt.Message.GetExtendedTextMessage(); extendedText != nil {
		return extendedText.ContextInfo.GetIsForwarded()
	} else if protocolMessage := evt.Message.GetProtocolMessage(); protocolMessage != nil {
		if editedMessage := protocolMessage.GetEditedMessage(); editedMessage != nil {
			if extendedText := editedMessage.GetExtendedTextMessage(); extendedText != nil {
				return extendedText.ContextInfo.GetIsForwarded()
			}
		}
	}
	return false
}
