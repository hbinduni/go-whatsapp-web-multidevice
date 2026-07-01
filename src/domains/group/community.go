package group

import "go.mau.fi/whatsmeow/types"

type CommunityRequest struct {
	CommunityID string `json:"community_id" form:"community_id" query:"community_id"`
}

type LinkGroupRequest struct {
	CommunityID string `json:"community_id" form:"community_id"`
	GroupID     string `json:"group_id" form:"group_id"`
}

type SubGroupsResponse struct {
	Data []*types.GroupLinkTarget `json:"data"`
}

type LinkedParticipantsResponse struct {
	Participants []string `json:"participants"`
}
