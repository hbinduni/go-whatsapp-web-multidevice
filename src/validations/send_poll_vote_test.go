package validations

import (
	"context"
	"testing"

	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
)

func TestValidateSendPollVote(t *testing.T) {
	// missing phone
	if err := ValidateSendPollVote(context.Background(), domainSend.PollVoteRequest{PollMessageID: "ABC", OptionNames: []string{"Yes"}}); err == nil {
		t.Error("expected error for empty phone, got nil")
	}
	// missing poll_message_id
	if err := ValidateSendPollVote(context.Background(), domainSend.PollVoteRequest{BaseRequest: domainSend.BaseRequest{Phone: "628@s.whatsapp.net"}, OptionNames: []string{"Yes"}}); err == nil {
		t.Error("expected error for empty poll_message_id, got nil")
	}
	// valid (empty OptionNames is allowed = retract vote)
	if err := ValidateSendPollVote(context.Background(), domainSend.PollVoteRequest{BaseRequest: domainSend.BaseRequest{Phone: "628@s.whatsapp.net"}, PollMessageID: "ABC"}); err != nil {
		t.Errorf("expected no error for retract vote, got %v", err)
	}
}
