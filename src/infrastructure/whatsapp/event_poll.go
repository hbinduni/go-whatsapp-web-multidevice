package whatsapp

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/sse"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// pollVoteHashes hex-encodes the SHA-256 option hashes of a decrypted poll vote.
func pollVoteHashes(vote *waE2E.PollVoteMessage) []string {
	hashes := make([]string, 0, len(vote.GetSelectedOptions()))
	for _, opt := range vote.GetSelectedOptions() {
		hashes = append(hashes, hex.EncodeToString(opt))
	}
	return hashes
}

// buildPollVotePayload converts a decrypted poll vote into a forwardable payload.
// SelectedOptions are SHA-256 hashes of the option names; consumers match them
// against the poll's option hashes. pollMessageID identifies WHICH poll the vote
// targets (distinct from messageID, the vote stanza's own id).
func buildPollVotePayload(messageID, pollMessageID, chatJID, senderJID string, isFromMe bool, ts time.Time, vote *waE2E.PollVoteMessage) map[string]any {
	return map[string]any{
		"event":                  "poll_vote",
		"message_id":             messageID,
		"poll_message_id":        pollMessageID,
		"chat_jid":               chatJID,
		"sender_jid":             senderJID,
		"is_from_me":             isFromMe,
		"selected_option_hashes": pollVoteHashes(vote),
		"timestamp":              ts.Format(time.RFC3339),
	}
}

// handlePollUpdateMessage decrypts an inbound poll vote and forwards it via SSE
// and (if configured) the webhook. Best-effort: failures are logged, never fatal.
func handlePollUpdateMessage(ctx context.Context, evt *events.Message) {
	client := GetClient()
	normalizedChatJID := NormalizeJIDFromLID(ctx, evt.Info.Chat, client)
	normalizedSenderJID := NormalizeJIDFromLID(ctx, evt.Info.Sender, client)

	vote, err := client.DecryptPollVote(ctx, evt)
	if err != nil {
		logrus.Errorf("Failed to decrypt poll vote %s: %v", evt.Info.ID, err)
		return
	}

	pollMessageID := evt.Message.GetPollUpdateMessage().GetPollCreationMessageKey().GetID()

	payload := buildPollVotePayload(
		evt.Info.ID,
		pollMessageID,
		normalizedChatJID.String(),
		normalizedSenderJID.String(),
		evt.Info.IsFromMe,
		evt.Info.Timestamp,
		vote,
	)
	// device_jid identifies which WhatsApp account received the vote (matches sibling payload builders).
	if client.Store != nil && client.Store.ID != nil {
		payload["device_jid"] = client.Store.ID.String()
	}

	sse.BroadcastPollVote(
		evt.Info.ID,
		pollMessageID,
		normalizedChatJID.String(),
		normalizedSenderJID.String(),
		pollVoteHashes(vote),
		evt.Info.Timestamp,
		evt.Info.IsFromMe,
	)

	go func() {
		if err := forwardPayloadToConfiguredWebhooks(ctx, payload, "poll_vote"); err != nil {
			logrus.Errorf("Failed to forward poll vote to webhook: %v", err)
		}
	}()
}
