package user

import "go.mau.fi/whatsmeow/types"

type BlockRequest struct {
	Phone string `json:"phone" form:"phone"`
}

type SetAboutRequest struct {
	Status string `json:"status" form:"status"`
}

type BlocklistResponse struct {
	Data *types.Blocklist `json:"data"`
}
