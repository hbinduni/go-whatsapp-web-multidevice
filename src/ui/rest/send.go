package rest

import (
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/gofiber/fiber/v2"
)

type Send struct {
	Service domainSend.ISendUsecase
}

func InitRestSend(app fiber.Router, service domainSend.ISendUsecase) Send {
	rest := Send{Service: service}
	app.Post("/send/message", rest.SendText)
	app.Post("/send/image", rest.SendImage)
	app.Post("/send/file", rest.SendFile)
	app.Post("/send/video", rest.SendVideo)
	app.Post("/send/sticker", rest.SendSticker)
	app.Post("/send/contact", rest.SendContact)
	app.Post("/send/link", rest.SendLink)
	app.Post("/send/location", rest.SendLocation)
	app.Post("/send/audio", rest.SendAudio)
	app.Post("/send/poll", rest.SendPoll)
	app.Post("/send/poll/vote", rest.SendPollVote)
	app.Post("/send/presence", rest.SendPresence)
	app.Post("/send/chat-presence", rest.SendChatPresence)
	return rest
}

func (controller *Send) SendText(c *fiber.Ctx) error {
	var request domainSend.MessageRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendText(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendImage(c *fiber.Ctx) error {
	var request domainSend.ImageRequest
	request.Compress = true

	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	if file, err := c.FormFile("image"); err == nil {
		request.Image = file
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendImage(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendFile(c *fiber.Ctx) error {
	var request domainSend.FileRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	file, err := c.FormFile("file")
	if err != nil {
		return helpers.HandleBadRequest(c, "File is required: "+err.Error())
	}

	request.File = file
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendFile(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendVideo(c *fiber.Ctx) error {
	var request domainSend.VideoRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	// Try to get file but ignore error if not provided
	if videoFile, errFile := c.FormFile("video"); errFile == nil {
		request.Video = videoFile
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendVideo(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendSticker(c *fiber.Ctx) error {
	var request domainSend.StickerRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	// Try to get file but ignore error if not provided
	if stickerFile, errFile := c.FormFile("sticker"); errFile == nil {
		request.Sticker = stickerFile
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendSticker(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendContact(c *fiber.Ctx) error {
	var request domainSend.ContactRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendContact(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendLink(c *fiber.Ctx) error {
	var request domainSend.LinkRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendLink(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendLocation(c *fiber.Ctx) error {
	var request domainSend.LocationRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendLocation(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendAudio(c *fiber.Ctx) error {
	var request domainSend.AudioRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	// Try to get file but ignore error if not provided
	if audioFile, errFile := c.FormFile("audio"); errFile == nil {
		request.Audio = audioFile
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendAudio(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendPoll(c *fiber.Ctx) error {
	var request domainSend.PollRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendPoll(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendPollVote(c *fiber.Ctx) error {
	var request domainSend.PollVoteRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendPollVote(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendPresence(c *fiber.Ctx) error {
	var request domainSend.PresenceRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	response, err := controller.Service.SendPresence(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}

func (controller *Send) SendChatPresence(c *fiber.Ctx) error {
	var request domainSend.ChatPresenceRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendChatPresence(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Status, response)
}
