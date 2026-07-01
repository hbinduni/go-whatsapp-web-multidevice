package validations

import (
	"context"
	"testing"

	domainNewsletter "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/newsletter"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/stretchr/testify/assert"
)

func TestValidateUnfollowNewsletter(t *testing.T) {
	type args struct {
		request domainNewsletter.UnfollowRequest
	}
	tests := []struct {
		name string
		args args
		err  any
	}{
		{
			name: "should success with valid newsletter id",
			args: args{request: domainNewsletter.UnfollowRequest{
				NewsletterID: "120363123456789@newsletter",
			}},
			err: nil,
		},
		{
			name: "should success with different newsletter id format",
			args: args{request: domainNewsletter.UnfollowRequest{
				NewsletterID: "newsletter-abc123xyz",
			}},
			err: nil,
		},
		{
			name: "should error with empty newsletter id",
			args: args{request: domainNewsletter.UnfollowRequest{
				NewsletterID: "",
			}},
			err: pkgError.ValidationError("newsletter_id: cannot be blank."),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUnfollowNewsletter(context.Background(), tt.args.request)
			assert.Equal(t, tt.err, err)
		})
	}
}

func TestValidateFollowNewsletter(t *testing.T) {
	if err := ValidateFollowNewsletter(context.Background(), domainNewsletter.FollowRequest{}); err == nil {
		t.Error("expected error for empty newsletter_id, got nil")
	}
	if err := ValidateFollowNewsletter(context.Background(), domainNewsletter.FollowRequest{NewsletterID: "123@newsletter"}); err != nil {
		t.Errorf("expected no error for valid newsletter_id, got %v", err)
	}
}

func TestValidateGetNewsletterInfo(t *testing.T) {
	if err := ValidateGetNewsletterInfo(context.Background(), domainNewsletter.GetInfoRequest{}); err == nil {
		t.Error("expected error for empty newsletter_id, got nil")
	}
	if err := ValidateGetNewsletterInfo(context.Background(), domainNewsletter.GetInfoRequest{NewsletterID: "123@newsletter"}); err != nil {
		t.Errorf("expected no error for valid newsletter_id, got %v", err)
	}
}

func TestValidateGetNewsletterInfoWithInvite(t *testing.T) {
	if err := ValidateGetNewsletterInfoWithInvite(context.Background(), domainNewsletter.GetInfoWithInviteRequest{}); err == nil {
		t.Error("expected error for empty key, got nil")
	}
	if err := ValidateGetNewsletterInfoWithInvite(context.Background(), domainNewsletter.GetInfoWithInviteRequest{Key: "abc123"}); err != nil {
		t.Errorf("expected no error for valid key, got %v", err)
	}
}

func TestValidateNewsletterMute(t *testing.T) {
	if err := ValidateNewsletterMute(context.Background(), domainNewsletter.ToggleMuteRequest{}); err == nil {
		t.Error("expected error for empty newsletter_id, got nil")
	}
	if err := ValidateNewsletterMute(context.Background(), domainNewsletter.ToggleMuteRequest{NewsletterID: "123@newsletter"}); err != nil {
		t.Errorf("expected no error for valid newsletter_id, got %v", err)
	}
}
