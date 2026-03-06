package rest

import (
	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/gofiber/fiber/v2"
)

type User struct {
	Service domainUser.IUserUsecase
}

func InitRestUser(app fiber.Router, service domainUser.IUserUsecase) User {
	rest := User{Service: service}
	app.Get("/user/info", rest.UserInfo)
	app.Get("/user/avatar", rest.UserAvatar)
	app.Post("/user/avatar", rest.UserChangeAvatar)
	app.Post("/user/pushname", rest.UserChangePushName)
	app.Get("/user/my/privacy", rest.UserMyPrivacySetting)
	app.Post("/user/my/privacy", rest.UserUpdateMyPrivacySetting)
	app.Get("/user/my/groups", rest.UserMyListGroups)
	app.Get("/user/my/newsletters", rest.UserMyListNewsletter)
	app.Get("/user/my/contacts", rest.UserMyListContacts)
	app.Get("/user/check", rest.UserCheck)
	app.Get("/user/business-profile", rest.UserBusinessProfile)

	return rest
}

func (controller *User) UserInfo(c *fiber.Ctx) error {
	var request domainUser.InfoRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.Info(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get user info", response.Data[0])
}

func (controller *User) UserAvatar(c *fiber.Ctx) error {
	var request domainUser.AvatarRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.Avatar(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get avatar", response)
}

func (controller *User) UserChangeAvatar(c *fiber.Ctx) error {
	var request domainUser.ChangeAvatarRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		return helpers.HandleBadRequest(c, "Avatar file is required: "+err.Error())
	}
	request.Avatar = file

	err = controller.Service.ChangeAvatar(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success change avatar", nil)
}

func (controller *User) UserMyPrivacySetting(c *fiber.Ctx) error {
	response, err := controller.Service.MyPrivacySetting(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get privacy", response)
}

func (controller *User) UserUpdateMyPrivacySetting(c *fiber.Ctx) error {
	var request domainUser.UpdatePrivacySettingRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	response, err := controller.Service.UpdateMyPrivacySetting(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success update privacy setting", response)
}

func (controller *User) UserMyListGroups(c *fiber.Ctx) error {
	response, err := controller.Service.MyListGroups(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get list groups", response)
}

func (controller *User) UserMyListNewsletter(c *fiber.Ctx) error {
	response, err := controller.Service.MyListNewsletter(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get list newsletter", response)
}

func (controller *User) UserMyListContacts(c *fiber.Ctx) error {
	response, err := controller.Service.MyListContacts(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get list contacts", response)
}

func (controller *User) UserChangePushName(c *fiber.Ctx) error {
	var request domainUser.ChangePushNameRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	err := controller.Service.ChangePushName(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success change push name", nil)
}

func (controller *User) UserCheck(c *fiber.Ctx) error {
	var request domainUser.CheckRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	response, err := controller.Service.IsOnWhatsApp(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success check user", response)
}

func (controller *User) UserBusinessProfile(c *fiber.Ctx) error {
	var request domainUser.BusinessProfileRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.BusinessProfile(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get business profile", response)
}
