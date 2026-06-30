package usecase

import (
	"context"

	domainNewsletter "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/newsletter"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"go.mau.fi/whatsmeow/types"
)

type serviceNewsletter struct{}

func NewNewsletterService() domainNewsletter.INewsletterUsecase {
	return &serviceNewsletter{}
}

func (service serviceNewsletter) Unfollow(ctx context.Context, request domainNewsletter.UnfollowRequest) (err error) {
	if err = validations.ValidateUnfollowNewsletter(ctx, request); err != nil {
		return err
	}

	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.NewsletterID)
	if err != nil {
		return err
	}

	return whatsapp.GetClient().UnfollowNewsletter(ctx, JID)
}

func (service serviceNewsletter) Follow(ctx context.Context, request domainNewsletter.FollowRequest) (err error) {
	if err = validations.ValidateFollowNewsletter(ctx, request); err != nil {
		return err
	}
	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.NewsletterID)
	if err != nil {
		return err
	}
	return whatsapp.GetClient().FollowNewsletter(ctx, JID)
}

func (service serviceNewsletter) GetInfo(ctx context.Context, request domainNewsletter.GetInfoRequest) (response *types.NewsletterMetadata, err error) {
	if err = validations.ValidateGetNewsletterInfo(ctx, request); err != nil {
		return nil, err
	}
	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.NewsletterID)
	if err != nil {
		return nil, err
	}
	return whatsapp.GetClient().GetNewsletterInfo(ctx, JID)
}

func (service serviceNewsletter) GetInfoWithInvite(ctx context.Context, request domainNewsletter.GetInfoWithInviteRequest) (response *types.NewsletterMetadata, err error) {
	if err = validations.ValidateGetNewsletterInfoWithInvite(ctx, request); err != nil {
		return nil, err
	}
	utils.MustLogin(whatsapp.GetClient())
	return whatsapp.GetClient().GetNewsletterInfoWithInvite(ctx, request.Key)
}

func (service serviceNewsletter) ToggleMute(ctx context.Context, request domainNewsletter.ToggleMuteRequest) (err error) {
	if err = validations.ValidateNewsletterMute(ctx, request); err != nil {
		return err
	}
	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.NewsletterID)
	if err != nil {
		return err
	}
	return whatsapp.GetClient().NewsletterToggleMute(ctx, JID, request.Mute)
}
