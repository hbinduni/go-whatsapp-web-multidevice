package rest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	domainAdmin "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/admin"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
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

	// Chat Storage Operations (chatstorage.db)
	admin.Get("/storage/stats", rest.GetStorageStats)
	admin.Post("/storage/cleanup", rest.CleanupStorage)
	admin.Post("/storage/vacuum", rest.VacuumDatabase)
	admin.Delete("/storage/chats", rest.DeleteChats)

	// WhatsApp Store Operations (whatsapp.db - device/keys database)
	admin.Get("/whatsapp-store/stats", rest.GetWhatsAppStoreStats)
	admin.Get("/whatsapp-store/export", rest.ExportWhatsAppStore)
	admin.Post("/whatsapp-store/import", rest.ImportWhatsAppStore)
	admin.Post("/whatsapp-store/vacuum", rest.VacuumWhatsAppStore)

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

// =============================================================================
// WhatsApp Store Endpoints (device/keys database)
// =============================================================================

// GetWhatsAppStoreStats returns statistics for WhatsApp store databases
// @Summary Get WhatsApp store statistics
// @Description Returns statistics about the WhatsApp device/keys database
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} domainAdmin.WhatsAppStoreStatsResponse
// @Router /admin/whatsapp-store/stats [get]
func (controller *Admin) GetWhatsAppStoreStats(c *fiber.Ctx) error {
	response, err := controller.Service.GetWhatsAppStoreStats(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "WhatsApp store statistics retrieved successfully", response)
}

// ExportWhatsAppStore exports WhatsApp store database as a downloadable file
// @Summary Export WhatsApp store
// @Description Downloads the WhatsApp device/keys database file for backup
// @Tags Admin
// @Produce octet-stream
// @Success 200 {file} binary "WhatsApp store database file"
// @Router /admin/whatsapp-store/export [get]
func (controller *Admin) ExportWhatsAppStore(c *fiber.Ctx) error {
	response, err := controller.Service.ExportWhatsAppStore(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "EXPORT_ERROR",
			Message: err.Error(),
			Results: nil,
		})
	}

	// Set headers for file download
	filename := fmt.Sprintf("whatsapp-store-%s.db", time.Now().Format("20060102-150405"))
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Set("Content-Type", "application/octet-stream")

	// Add warning header if client is connected
	if response.Warning != "" {
		c.Set("X-Export-Warning", response.Warning)
	}

	return c.SendFile(response.MainDBPath)
}

// ImportWhatsAppStore imports WhatsApp store from uploaded backup file
// @Summary Import WhatsApp store
// @Description Imports WhatsApp device/keys database from a backup file. Requires client to be disconnected.
// @Tags Admin
// @Accept multipart/form-data
// @Produce json
// @Param file formance file true "WhatsApp store backup file (.db)"
// @Param keys_file formData file false "Keys database backup file (.db) - optional"
// @Success 200 {object} domainAdmin.WhatsAppStoreImportResponse
// @Router /admin/whatsapp-store/import [post]
func (controller *Admin) ImportWhatsAppStore(c *fiber.Ctx) error {
	// Get uploaded main database file
	file, err := c.FormFile("file")
	if err != nil {
		return helpers.HandleBadRequest(c, "No file uploaded. Please upload a WhatsApp store backup file (.db)")
	}

	// Validate file extension
	ext := filepath.Ext(file.Filename)
	if ext != ".db" && ext != ".sqlite" && ext != ".sqlite3" {
		return helpers.HandleBadRequest(c, "Invalid file type. Please upload a SQLite database file (.db, .sqlite, .sqlite3)")
	}

	// Save main DB to temp file
	tempMainPath := filepath.Join(os.TempDir(), fmt.Sprintf("wa-store-import-%d%s", time.Now().UnixNano(), ext))
	if err := c.SaveFile(file, tempMainPath); err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "UPLOAD_ERROR",
			Message: "Failed to save uploaded file",
			Results: nil,
		})
	}
	defer os.Remove(tempMainPath)

	// Handle optional keys database file
	var tempKeysPath string
	keysFile, err := c.FormFile("keys_file")
	if err == nil && keysFile != nil {
		keysExt := filepath.Ext(keysFile.Filename)
		if keysExt == ".db" || keysExt == ".sqlite" || keysExt == ".sqlite3" {
			tempKeysPath = filepath.Join(os.TempDir(), fmt.Sprintf("wa-keys-import-%d%s", time.Now().UnixNano(), keysExt))
			if err := c.SaveFile(keysFile, tempKeysPath); err == nil {
				defer os.Remove(tempKeysPath)
			} else {
				tempKeysPath = ""
			}
		}
	}

	// Perform import
	response, err := controller.Service.ImportWhatsAppStore(c.UserContext(), tempMainPath, tempKeysPath)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, response.Message, response)
}

// VacuumWhatsAppStore runs VACUUM on WhatsApp store databases
// @Summary Vacuum WhatsApp store
// @Description Runs SQLite VACUUM on WhatsApp device/keys databases to optimize and reclaim space
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} domainAdmin.WhatsAppStoreVacuumResponse
// @Router /admin/whatsapp-store/vacuum [post]
func (controller *Admin) VacuumWhatsAppStore(c *fiber.Ctx) error {
	response, err := controller.Service.VacuumWhatsAppStore(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}

	message := "WhatsApp store vacuum completed successfully"
	if response.Warning != "" {
		message = fmt.Sprintf("%s. Warning: %s", message, response.Warning)
	}

	return helpers.HandleSuccess(c, message, response)
}
