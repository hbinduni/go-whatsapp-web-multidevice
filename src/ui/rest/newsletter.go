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
