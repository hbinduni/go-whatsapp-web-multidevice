package validations

import (
	"context"
	"testing"

	domainGroup "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/group"
)

func TestValidateSetGroupJoinApprovalMode(t *testing.T) {
	if err := ValidateSetGroupJoinApprovalMode(context.Background(), domainGroup.SetGroupJoinApprovalModeRequest{}); err == nil {
		t.Error("expected error for empty group_id, got nil")
	}
	if err := ValidateSetGroupJoinApprovalMode(context.Background(), domainGroup.SetGroupJoinApprovalModeRequest{GroupID: "123@g.us", Enabled: true}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateSetGroupMemberAddMode(t *testing.T) {
	if err := ValidateSetGroupMemberAddMode(context.Background(), domainGroup.SetGroupMemberAddModeRequest{GroupID: "123@g.us"}); err == nil {
		t.Error("expected error for empty mode, got nil")
	}
	if err := ValidateSetGroupMemberAddMode(context.Background(), domainGroup.SetGroupMemberAddModeRequest{GroupID: "123@g.us", Mode: "invalid_mode"}); err == nil {
		t.Error("expected error for invalid mode, got nil")
	}
	if err := ValidateSetGroupMemberAddMode(context.Background(), domainGroup.SetGroupMemberAddModeRequest{GroupID: "123@g.us", Mode: "admin_add"}); err != nil {
		t.Errorf("expected no error for admin_add, got %v", err)
	}
}
