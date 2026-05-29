package rest

import (
	domainMessage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/message"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/gofiber/fiber/v2"
)

type Message struct {
	Service domainMessage.IMessageUsecase
}

func InitRestMessage(app fiber.Router, service domainMessage.IMessageUsecase) Message {
	rest := Message{Service: service}

	// Message action endpoints
	app.Post("/message/:message_id/reaction", rest.ReactMessage)
	app.Post("/message/:message_id/revoke", rest.RevokeMessage)
	app.Post("/message/:message_id/delete", rest.DeleteMessage)
	app.Post("/message/:message_id/update", rest.UpdateMessage)
	app.Post("/message/:message_id/read", rest.MarkAsRead)
	app.Post("/message/:message_id/star", rest.StarMessage)
	app.Post("/message/:message_id/unstar", rest.UnstarMessage)
	app.Post("/message/:message_id/pin", rest.PinMessage)
	app.Post("/message/:message_id/unpin", rest.UnpinMessage)
	app.Get("/message/:message_id/download", rest.DownloadMedia)
	return rest
}

func (controller *Message) RevokeMessage(c *fiber.Ctx) error {
	var request domainMessage.RevokeRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.RevokeMessage(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Message) DeleteMessage(c *fiber.Ctx) error {
	var request domainMessage.DeleteRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)

	err := controller.Service.DeleteMessage(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Message deleted successfully", nil)
}

func (controller *Message) UpdateMessage(c *fiber.Ctx) error {
	var request domainMessage.UpdateMessageRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.UpdateMessage(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Message) ReactMessage(c *fiber.Ctx) error {
	var request domainMessage.ReactionRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.ReactMessage(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Message) MarkAsRead(c *fiber.Ctx) error {
	var request domainMessage.MarkAsReadRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.MarkAsRead(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Message) StarMessage(c *fiber.Ctx) error {
	var request domainMessage.StarRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)
	request.IsStarred = true

	err := controller.Service.StarMessage(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Starred message successfully", nil)
}

func (controller *Message) UnstarMessage(c *fiber.Ctx) error {
	var request domainMessage.StarRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)
	request.IsStarred = false

	err := controller.Service.StarMessage(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Unstarred message successfully", nil)
}

func (controller *Message) PinMessage(c *fiber.Ctx) error {
	return controller.handlePin(c, true)
}

func (controller *Message) UnpinMessage(c *fiber.Ctx) error {
	return controller.handlePin(c, false)
}

func (controller *Message) handlePin(c *fiber.Ctx, isPinned bool) error {
	var request domainMessage.PinRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)
	request.IsPinned = isPinned

	response, err := controller.Service.PinMessage(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Message) DownloadMedia(c *fiber.Ctx) error {
	var request domainMessage.DownloadMediaRequest

	request.MessageID = c.Params("message_id")
	request.Phone = c.Query("phone")
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.DownloadMedia(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}
