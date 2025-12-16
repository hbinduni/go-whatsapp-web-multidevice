package rest

import (
	"github.com/aldinokemal/go-whatsapp-web-multidevice/domains/history"
	"github.com/gofiber/fiber/v2"
)

func InitRestHistory(app fiber.Router, service history.IHistoryUsecase) {
	app.Post("/history/sync", func(c *fiber.Ctx) error {
		var request history.HistorySyncRequest
		if err := c.BodyParser(&request); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		response, err := service.RequestHistorySync(c.Context(), request)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(response)
	})

	app.Get("/history/status", func(c *fiber.Ctx) error {
		response, err := service.GetHistorySyncStatus(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(response)
	})
}
