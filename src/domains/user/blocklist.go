package user

import (
	"encoding/json"

	"go.mau.fi/whatsmeow/types"
)

type BlockRequest struct {
	Phone string `json:"phone" form:"phone"`
}

type SetAboutRequest struct {
	Status string `json:"status" form:"status"`
}

type BlocklistResponse struct {
	Data *types.Blocklist `json:"data"`
}

// MarshalJSON preserves the REST contract after whatsmeow replaced JIDs with Items.
func (response BlocklistResponse) MarshalJSON() ([]byte, error) {
	type blocklistData struct {
		DHash string
		JIDs  []types.JID
	}
	var data *blocklistData
	if response.Data != nil {
		data = &blocklistData{DHash: response.Data.DHash}
		for _, item := range response.Data.Items {
			jid := item.PN
			if jid.IsEmpty() {
				jid = item.LID
			}
			data.JIDs = append(data.JIDs, jid)
		}
	}
	return json.Marshal(struct {
		Data *blocklistData `json:"data"`
	}{Data: data})
}
