package validations

import (
	"context"
	"testing"

	domainGroup "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/group"
)

func TestValidateCommunity(t *testing.T) {
	if err := ValidateCommunity(context.Background(), domainGroup.CommunityRequest{}); err == nil {
		t.Error("expected error for empty community_id, got nil")
	}
	if err := ValidateCommunity(context.Background(), domainGroup.CommunityRequest{CommunityID: "123@g.us"}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateLinkGroup(t *testing.T) {
	if err := ValidateLinkGroup(context.Background(), domainGroup.LinkGroupRequest{CommunityID: "123@g.us"}); err == nil {
		t.Error("expected error for empty group_id, got nil")
	}
	if err := ValidateLinkGroup(context.Background(), domainGroup.LinkGroupRequest{CommunityID: "123@g.us", GroupID: "456@g.us"}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
