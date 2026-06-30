package group

type SetGroupJoinApprovalModeRequest struct {
	GroupID string `json:"group_id" form:"group_id"`
	Enabled bool   `json:"enabled" form:"enabled"`
}

type SetGroupMemberAddModeRequest struct {
	GroupID string `json:"group_id" form:"group_id"`
	Mode    string `json:"mode" form:"mode"` // "admin_add" or "all_member_add"
}
