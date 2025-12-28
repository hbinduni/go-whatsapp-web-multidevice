package rest

import (
	domainAdmin "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/admin"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/gofiber/fiber/v2"
)

// Admin handles admin-related REST endpoints
type Admin struct {
	Service domainAdmin.IAdminUsecase
}

// InitRestAdmin initializes admin REST endpoints
func InitRestAdmin(app fiber.Router, service domainAdmin.IAdminUsecase) Admin {
	rest := Admin{Service: service}

	admin := app.Group("/admin")
	admin.Get("/storage/stats", rest.GetStorageStats)
	admin.Post("/storage/cleanup", rest.CleanupStorage)
	admin.Post("/storage/vacuum", rest.VacuumDatabase)
	admin.Delete("/storage/chats", rest.DeleteChats)

	return rest
}

// GetStorageStats returns detailed storage statistics
// @Summary Get storage statistics
// @Description Returns detailed statistics about the chat storage database
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} domainAdmin.StorageStatsResponse
// @Router /admin/storage/stats [get]
func (controller *Admin) GetStorageStats(c *fiber.Ctx) error {
	response, err := controller.Service.GetStorageStats(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Storage statistics retrieved successfully", response)
}

// CleanupStorage performs cleanup operations
// @Summary Cleanup storage
// @Description Performs cleanup operations based on the request (pattern, older_than, empty_chats)
// @Tags Admin
// @Accept json
// @Produce json
// @Param request body domainAdmin.CleanupRequest true "Cleanup request"
// @Success 200 {object} domainAdmin.CleanupResponse
// @Router /admin/storage/cleanup [post]
func (controller *Admin) CleanupStorage(c *fiber.Ctx) error {
	var request domainAdmin.CleanupRequest

	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	response, err := controller.Service.CleanupStorage(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	message := "Cleanup completed successfully"
	if request.DryRun {
		message = "Cleanup dry run completed (no changes made)"
	}

	return helpers.HandleSuccess(c, message, response)
}

// VacuumDatabase runs SQLite VACUUM to optimize and reclaim space
// @Summary Vacuum database
// @Description Runs SQLite VACUUM to optimize the database and reclaim space
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} domainAdmin.VacuumResponse
// @Router /admin/storage/vacuum [post]
func (controller *Admin) VacuumDatabase(c *fiber.Ctx) error {
	response, err := controller.Service.VacuumDatabase(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Database vacuum completed successfully", response)
}

// DeleteChats deletes chats by pattern or specific JIDs
// @Summary Delete chats
// @Description Deletes chats by pattern or specific JIDs
// @Tags Admin
// @Accept json
// @Produce json
// @Param request body domainAdmin.DeleteChatsRequest true "Delete chats request"
// @Success 200 {object} domainAdmin.DeleteChatsResponse
// @Router /admin/storage/chats [delete]
func (controller *Admin) DeleteChats(c *fiber.Ctx) error {
	var request domainAdmin.DeleteChatsRequest

	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	response, err := controller.Service.DeleteChats(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	message := "Chats deleted successfully"
	if request.DryRun {
		message = "Delete dry run completed (no changes made)"
	}

	return helpers.HandleSuccess(c, message, response)
}
