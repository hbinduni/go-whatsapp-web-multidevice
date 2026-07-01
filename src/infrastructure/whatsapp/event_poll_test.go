package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

func TestBuildPollVotePayload(t *testing.T) {
	vote := &waE2E.PollVoteMessage{
		SelectedOptions: [][]byte{{0x01, 0x02}, {0x03, 0x04}},
	}
	ts := time.Unix(1700000000, 0)
	payload := buildPollVotePayload("VOTEID", "POLLID", "123@g.us", "456@s.whatsapp.net", false, ts, vote)

	if payload["message_id"] != "VOTEID" {
		t.Errorf("message_id = %v, want VOTEID", payload["message_id"])
	}
	if payload["poll_message_id"] != "POLLID" {
		t.Errorf("poll_message_id = %v, want POLLID", payload["poll_message_id"])
	}
	if payload["chat_jid"] != "123@g.us" {
		t.Errorf("chat_jid = %v, want 123@g.us", payload["chat_jid"])
	}
	hashes, ok := payload["selected_option_hashes"].([]string)
	if !ok || len(hashes) != 2 {
		t.Fatalf("selected_option_hashes = %v, want 2 hex strings", payload["selected_option_hashes"])
	}
	if hashes[0] != "0102" {
		t.Errorf("hashes[0] = %s, want 0102", hashes[0])
	}
}
