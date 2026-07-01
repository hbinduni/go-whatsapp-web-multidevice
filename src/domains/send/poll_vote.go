package send

type PollVoteRequest struct {
	BaseRequest
	PollMessageID string   `json:"poll_message_id" form:"poll_message_id"`
	OptionNames   []string `json:"option_names" form:"option_names"`
}
