package validations

import (
	"context"
	"testing"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
)

func TestValidateBlockUser(t *testing.T) {
	if err := ValidateBlockUser(context.Background(), domainUser.BlockRequest{}); err == nil {
		t.Error("expected error for empty phone, got nil")
	}
	if err := ValidateBlockUser(context.Background(), domainUser.BlockRequest{Phone: "628123@s.whatsapp.net"}); err != nil {
		t.Errorf("expected no error for valid phone, got %v", err)
	}
}

func TestValidateSetAbout(t *testing.T) {
	if err := ValidateSetAbout(context.Background(), domainUser.SetAboutRequest{}); err == nil {
		t.Error("expected error for empty status, got nil")
	}
}
