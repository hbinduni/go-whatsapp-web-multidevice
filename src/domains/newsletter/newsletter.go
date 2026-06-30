package newsletter

import (
	"context"

	"go.mau.fi/whatsmeow/types"
)

type INewsletterUsecase interface {
	Unfollow(ctx context.Context, request UnfollowRequest) (err error)
	Follow(ctx context.Context, request FollowRequest) (err error)
	GetInfo(ctx context.Context, request GetInfoRequest) (response *types.NewsletterMetadata, err error)
	GetInfoWithInvite(ctx context.Context, request GetInfoWithInviteRequest) (response *types.NewsletterMetadata, err error)
	ToggleMute(ctx context.Context, request ToggleMuteRequest) (err error)
}

type UnfollowRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id"`
}

type FollowRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id"`
}

type GetInfoRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id" query:"newsletter_id"`
}

type GetInfoWithInviteRequest struct {
	Key string `json:"key" form:"key" query:"key"`
}

type ToggleMuteRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id"`
	Mute         bool   `json:"mute" form:"mute"`
}
