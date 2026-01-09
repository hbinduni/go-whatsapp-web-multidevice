package rest

import (
	"fmt"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/gofiber/fiber/v2"
)

type App struct {
	Service domainApp.IAppUsecase
}

func InitRestApp(app fiber.Router, service domainApp.IAppUsecase) App {
	rest := App{Service: service}
	app.Get("/app/login", rest.Login)
	app.Get("/app/login-with-code", rest.LoginWithCode)
	app.Get("/app/logout", rest.Logout)
	app.Get("/app/reconnect", rest.Reconnect)
	app.Get("/app/devices", rest.Devices)
	app.Get("/app/status", rest.ConnectionStatus)

	// Client start/stop endpoints (disconnect without logout)
	app.Post("/app/stop", rest.StopClient)
	app.Post("/app/start", rest.StartClient)

	return App{Service: service}
}

func (handler *App) Login(c *fiber.Ctx) error {
	response, err := handler.Service.Login(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Login success",
		Results: map[string]any{
			"qr_link":     fmt.Sprintf("%s://%s%s/%s", c.Protocol(), c.Hostname(), config.AppBasePath, response.ImagePath),
			"qr_duration": response.Duration,
		},
	})
}

func (handler *App) LoginWithCode(c *fiber.Ctx) error {
	pairCode, err := handler.Service.LoginWithCode(c.UserContext(), c.Query("phone"))
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Login with code success",
		Results: map[string]any{
			"pair_code": pairCode,
		},
	})
}

func (handler *App) Logout(c *fiber.Ctx) error {
	err := handler.Service.Logout(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success logout", nil)
}

func (handler *App) Reconnect(c *fiber.Ctx) error {
	err := handler.Service.Reconnect(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Reconnect success", nil)
}

func (handler *App) Devices(c *fiber.Ctx) error {
	devices, err := handler.Service.FetchDevices(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Fetch device success", devices)
}

func (handler *App) ConnectionStatus(c *fiber.Ctx) error {
	response, err := handler.Service.GetClientStatus(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.StatusMessage, response)
}

// StopClient stops the WhatsApp client without logging out
// @Summary Stop WhatsApp client
// @Description Disconnects from WhatsApp without invalidating the session. Useful for maintenance operations.
// @Tags App
// @Produce json
// @Success 200 {object} domainApp.ClientStatusResponse
// @Router /app/stop [post]
func (handler *App) StopClient(c *fiber.Ctx) error {
	response, err := handler.Service.StopClient(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.StatusMessage, response)
}

// StartClient starts the WhatsApp client after being stopped
// @Summary Start WhatsApp client
// @Description Reconnects to WhatsApp after being stopped. Session remains valid.
// @Tags App
// @Produce json
// @Success 200 {object} domainApp.ClientStatusResponse
// @Router /app/start [post]
func (handler *App) StartClient(c *fiber.Ctx) error {
	response, err := handler.Service.StartClient(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.StatusMessage, response)
}
