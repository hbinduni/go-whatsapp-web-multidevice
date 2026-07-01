package sse

import "time"

// BroadcastPollVote pushes a decrypted poll vote to connected SSE clients.
func BroadcastPollVote(messageID, pollMessageID, chatJID, senderJID string, selectedHashes []string, timestamp time.Time, isFromMe bool) {
	BroadcastMessage(EventPollVote, "POLL_VOTE", "Poll vote received", map[string]any{
		"message_id":             messageID,
		"poll_message_id":        pollMessageID,
		"chat_jid":               chatJID,
		"sender_jid":             senderJID,
		"selected_option_hashes": selectedHashes,
		"timestamp":              timestamp,
		"is_from_me":             isFromMe,
	})
}
