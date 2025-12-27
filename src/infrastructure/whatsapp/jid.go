package whatsapp

import (
	"context"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ExtractedMedia represents extracted media information
type ExtractedMedia struct {
	MediaPath string `json:"media_path"`
	MimeType  string `json:"mime_type"`
	Caption   string `json:"caption"`
	FileSize  int64  `json:"file_size"`
}

// NormalizeJIDFromLID converts @lid JIDs to their corresponding @s.whatsapp.net JIDs
// Returns the original JID if it's not an @lid or if LID lookup fails
func NormalizeJIDFromLID(ctx context.Context, jid types.JID, client *whatsmeow.Client) types.JID {
	// Only process @lid JIDs
	if jid.Server != "lid" {
		return jid
	}

	// Safety check
	if client == nil || client.Store == nil || client.Store.LIDs == nil {
		log.Warnf("Cannot resolve LID %s: client not available", jid.String())
		return jid
	}

	// Attempt to get the phone number for this LID
	pn, err := client.Store.LIDs.GetPNForLID(ctx, jid)
	if err != nil {
		log.Debugf("Failed to resolve LID %s to phone number: %v", jid.String(), err)
		return jid
	}

	// If we got a valid phone number, use it
	if !pn.IsEmpty() {
		log.Debugf("Resolved LID %s to phone number %s", jid.String(), pn.String())
		return pn
	}

	// Fallback to original JID
	return jid
}

// NormalizeJIDString converts a JID string from @lid format to @s.whatsapp.net format
// Uses the global WhatsApp client for LID resolution
// Returns the original string if conversion fails or JID is not an @lid
func NormalizeJIDString(ctx context.Context, jidStr string) string {
	// Quick check - if not @lid, return as-is
	if !strings.HasSuffix(jidStr, "@lid") {
		return jidStr
	}

	// Parse the JID string
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		log.Debugf("Failed to parse JID string %s: %v", jidStr, err)
		return jidStr
	}

	// Get the global client
	client := GetClient()
	if client == nil {
		log.Debugf("Cannot normalize JID %s: no WhatsApp client available", jidStr)
		return jidStr
	}

	// Normalize and return
	normalizedJID := NormalizeJIDFromLID(ctx, jid, client)
	return normalizedJID.String()
}
