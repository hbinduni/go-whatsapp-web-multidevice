package rest

import (
	domainAdmin "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/admin"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
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

	// Storage management endpoints
	admin.Get("/storage/stats", rest.GetStorageStats)
	admin.Post("/storage/cleanup", rest.CleanupStorage)
	admin.Post("/storage/vacuum", rest.VacuumDatabase)
	admin.Delete("/storage/chats", rest.DeleteChats)

	// Client management endpoints
	admin.Get("/clients", rest.ListClients)
	admin.Post("/clients", rest.AddClient)             // Add new client dynamically
	admin.Delete("/clients/:phone", rest.RemoveClient) // Remove client (keeps chat history)
	admin.Get("/clients/:phone/status", rest.GetClientStatus)
	admin.Post("/clients/:phone/connect", rest.ConnectClient)
	admin.Post("/clients/:phone/disconnect", rest.DisconnectClient)

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

// VacuumDatabase runs VACUUM to optimize and reclaim space
// @Summary Vacuum database
// @Description Runs VACUUM to optimize the database and reclaim space
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

// ============================================================================
// Client Management Endpoints
// ============================================================================

// ListClients returns all registered clients and their status
// @Summary List all clients
// @Description Returns a list of all registered WhatsApp clients with their connection status
// @Tags Admin
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /admin/clients [get]
func (controller *Admin) ListClients(c *fiber.Ctx) error {
	statuses := whatsapp.GetAllClientStatuses()
	if statuses == nil {
		return helpers.HandleError(c, fiber.NewError(fiber.StatusInternalServerError, "Client registry not initialized"))
	}

	return helpers.HandleSuccess(c, "Client list retrieved", map[string]interface{}{
		"client_count": len(statuses),
		"clients":      statuses,
	})
}

// GetClientStatus returns the status of a specific client
// @Summary Get client status
// @Description Returns the connection status of a specific WhatsApp client
// @Tags Admin
// @Produce json
// @Param phone path string true "Phone number"
// @Success 200 {object} map[string]interface{}
// @Router /admin/clients/{phone}/status [get]
func (controller *Admin) GetClientStatus(c *fiber.Ctx) error {
	phone := c.Params("phone")
	if phone == "" {
		return helpers.HandleBadRequest(c, "Phone number is required")
	}

	registry := whatsapp.GetRegistry()
	if registry == nil {
		return helpers.HandleError(c, fiber.NewError(fiber.StatusInternalServerError, "Client registry not initialized"))
	}

	isConnected, isLoggedIn, deviceID, err := registry.GetClientStatus(phone)
	if err != nil {
		return helpers.HandleError(c, fiber.NewError(fiber.StatusNotFound, "Client not found: "+phone))
	}

	return helpers.HandleSuccess(c, "Client status retrieved", map[string]interface{}{
		"phone":        phone,
		"is_connected": isConnected,
		"is_logged_in": isLoggedIn,
		"device_id":    deviceID,
	})
}

// ConnectClient connects a specific client
// @Summary Connect client
// @Description Connects a specific WhatsApp client
// @Tags Admin
// @Produce json
// @Param phone path string true "Phone number"
// @Success 200 {object} map[string]interface{}
// @Router /admin/clients/{phone}/connect [post]
func (controller *Admin) ConnectClient(c *fiber.Ctx) error {
	phone := c.Params("phone")
	if phone == "" {
		return helpers.HandleBadRequest(c, "Phone number is required")
	}

	registry := whatsapp.GetRegistry()
	if registry == nil {
		return helpers.HandleError(c, fiber.NewError(fiber.StatusInternalServerError, "Client registry not initialized"))
	}

	if err := registry.ConnectClient(phone); err != nil {
		return helpers.HandleError(c, fiber.NewError(fiber.StatusInternalServerError, "Failed to connect client: "+err.Error()))
	}

	return helpers.HandleSuccess(c, "Client connected successfully", map[string]interface{}{
		"phone":     phone,
		"connected": true,
	})
}

// DisconnectClient disconnects a specific client
// @Summary Disconnect client
// @Description Disconnects a specific WhatsApp client
// @Tags Admin
// @Produce json
// @Param phone path string true "Phone number"
// @Success 200 {object} map[string]interface{}
// @Router /admin/clients/{phone}/disconnect [post]
func (controller *Admin) DisconnectClient(c *fiber.Ctx) error {
	phone := c.Params("phone")
	if phone == "" {
		return helpers.HandleBadRequest(c, "Phone number is required")
	}

	registry := whatsapp.GetRegistry()
	if registry == nil {
		return helpers.HandleError(c, fiber.NewError(fiber.StatusInternalServerError, "Client registry not initialized"))
	}

	if err := registry.DisconnectClient(phone); err != nil {
		return helpers.HandleError(c, fiber.NewError(fiber.StatusInternalServerError, "Failed to disconnect client: "+err.Error()))
	}

	return helpers.HandleSuccess(c, "Client disconnected successfully", map[string]interface{}{
		"phone":        phone,
		"disconnected": true,
	})
}

// AddClient dynamically adds a new WhatsApp client
// @Summary Add a new WhatsApp client
// @Description Dynamically adds a new WhatsApp client to the system. The client will need to be logged in via QR code.
// @Tags Admin
// @Accept json
// @Produce json
// @Param request body domainAdmin.AddClientRequest true "Add client request"
// @Success 200 {object} domainAdmin.AddClientResponse
// @Failure 400 {object} map[string]interface{} "Invalid request or max clients reached"
// @Router /admin/clients [post]
func (controller *Admin) AddClient(c *fiber.Ctx) error {
	var request domainAdmin.AddClientRequest

	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	if request.Phone == "" {
		return helpers.HandleBadRequest(c, "Phone number is required")
	}

	response, err := controller.Service.AddClient(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Client added successfully", response)
}

// RemoveClient removes a WhatsApp client (keeps chat history)
// @Summary Remove a WhatsApp client
// @Description Removes a WhatsApp client from the system. The client will be disconnected and logged out. Chat history is preserved.
// @Tags Admin
// @Produce json
// @Param phone path string true "Phone number of the client to remove"
// @Success 200 {object} domainAdmin.RemoveClientResponse
// @Failure 400 {object} map[string]interface{} "Invalid phone number"
// @Failure 404 {object} map[string]interface{} "Client not found"
// @Router /admin/clients/{phone} [delete]
func (controller *Admin) RemoveClient(c *fiber.Ctx) error {
	phone := c.Params("phone")
	if phone == "" {
		return helpers.HandleBadRequest(c, "Phone number is required")
	}

	response, err := controller.Service.RemoveClient(c.UserContext(), phone)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Client removed successfully", response)
}
