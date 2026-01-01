package rest

import (
	domainChat "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chat"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/gofiber/fiber/v2"
)

type Chat struct {
	Service domainChat.IChatUsecase
}

func InitRestChat(app fiber.Router, service domainChat.IChatUsecase) Chat {
	rest := Chat{Service: service}

	// Chat endpoints
	app.Get("/chats", rest.ListChats)
	app.Get("/chat/:chat_jid/messages", rest.GetChatMessages)
	app.Post("/chat/:chat_jid/pin", rest.PinChat)
	app.Post("/chat/:chat_jid/disappearing", rest.SetDisappearingTimer)

	return rest
}

func (controller *Chat) ListChats(c *fiber.Ctx) error {
	var request domainChat.ListChatsRequest

	// Parse query parameters
	request.Limit = c.QueryInt("limit", 25)
	request.Offset = c.QueryInt("offset", 0)
	request.Search = c.Query("search", "")
	request.HasMedia = c.QueryBool("has_media", false)

	response, err := controller.Service.ListChats(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get chat list", response)
}

func (controller *Chat) GetChatMessages(c *fiber.Ctx) error {
	var request domainChat.GetChatMessagesRequest

	// Parse path parameter
	request.ChatJID = c.Params("chat_jid")

	// Parse query parameters
	request.Limit = c.QueryInt("limit", 50)
	request.Offset = c.QueryInt("offset", 0)
	request.MediaOnly = c.QueryBool("media_only", false)
	request.Search = c.Query("search", "")

	// Parse time filters
	if startTime := c.Query("start_time"); startTime != "" {
		request.StartTime = &startTime
	}
	if endTime := c.Query("end_time"); endTime != "" {
		request.EndTime = &endTime
	}

	// Parse is_from_me filter
	if isFromMeStr := c.Query("is_from_me"); isFromMeStr != "" {
		isFromMe := c.QueryBool("is_from_me")
		request.IsFromMe = &isFromMe
	}

	response, err := controller.Service.GetChatMessages(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get chat messages", response)
}

func (controller *Chat) PinChat(c *fiber.Ctx) error {
	var request domainChat.PinChatRequest

	// Parse path parameter
	request.ChatJID = c.Params("chat_jid")

	// Parse JSON body
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body")
	}

	response, err := controller.Service.PinChat(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Message, response)
}

func (controller *Chat) SetDisappearingTimer(c *fiber.Ctx) error {
	var request domainChat.SetDisappearingTimerRequest

	// Parse path parameter
	request.ChatJID = c.Params("chat_jid")

	// Parse JSON body
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body")
	}

	response, err := controller.Service.SetDisappearingTimer(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Message, response)
}
