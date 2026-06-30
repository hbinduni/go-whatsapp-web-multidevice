package rest

import (
	domainNewsletter "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/newsletter"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/gofiber/fiber/v2"
)

type Newsletter struct {
	Service domainNewsletter.INewsletterUsecase
}

func InitRestNewsletter(app fiber.Router, service domainNewsletter.INewsletterUsecase) Newsletter {
	rest := Newsletter{Service: service}
	app.Post("/newsletter/unfollow", rest.Unfollow)
	app.Post("/newsletter/follow", rest.Follow)
	app.Get("/newsletter/info", rest.GetInfo)
	app.Get("/newsletter/info-from-invite", rest.GetInfoWithInvite)
	app.Post("/newsletter/mute", rest.ToggleMute)
	return rest
}

func (controller *Newsletter) Unfollow(c *fiber.Ctx) error {
	var request domainNewsletter.UnfollowRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	err := controller.Service.Unfollow(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success unfollow newsletter", nil)
}

func (controller *Newsletter) Follow(c *fiber.Ctx) error {
	var request domainNewsletter.FollowRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := controller.Service.Follow(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success follow newsletter", nil)
}

func (controller *Newsletter) GetInfo(c *fiber.Ctx) error {
	var request domainNewsletter.GetInfoRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}
	response, err := controller.Service.GetInfo(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success get newsletter info", response)
}

func (controller *Newsletter) GetInfoWithInvite(c *fiber.Ctx) error {
	var request domainNewsletter.GetInfoWithInviteRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}
	response, err := controller.Service.GetInfoWithInvite(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success get newsletter info", response)
}

func (controller *Newsletter) ToggleMute(c *fiber.Ctx) error {
	var request domainNewsletter.ToggleMuteRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := controller.Service.ToggleMute(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success toggle newsletter mute", nil)
}
