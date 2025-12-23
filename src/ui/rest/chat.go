package rest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	domainChat "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chat"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type Chat struct {
	Service domainChat.IChatUsecase
}

func InitRestChat(app fiber.Router, service domainChat.IChatUsecase) Chat {
	rest := Chat{Service: service}

	// Chat endpoints
	app.Get("/chats", rest.ListChats)
	app.Get("/chat/:chat_jid/messages", rest.GetChatMessages)
	app.Post("/chat/:chat_jid/pin", rest.PinChat)
	app.Post("/chat/:chat_jid/disappearing", rest.SetDisappearingTimer)

	// Storage export/import endpoints
	app.Get("/chat/export", rest.ExportStorage)
	app.Post("/chat/import", rest.ImportStorage)
	app.Post("/chat/analyze", rest.AnalyzeStorage)

	return rest
}

func (controller *Chat) ListChats(c *fiber.Ctx) error {
	var request domainChat.ListChatsRequest

	// Parse query parameters
	request.Limit = c.QueryInt("limit", 25)
	request.Offset = c.QueryInt("offset", 0)
	request.Search = c.Query("search", "")
	request.HasMedia = c.QueryBool("has_media", false)

	response, err := controller.Service.ListChats(c.UserContext(), request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success get chat list",
		Results: response,
	})
}

func (controller *Chat) GetChatMessages(c *fiber.Ctx) error {
	var request domainChat.GetChatMessagesRequest

	// Parse path parameter
	request.ChatJID = c.Params("chat_jid")

	// Parse query parameters
	request.Limit = c.QueryInt("limit", 50)
	request.Offset = c.QueryInt("offset", 0)
	request.MediaOnly = c.QueryBool("media_only", false)
	request.Search = c.Query("search", "")

	// Parse time filters
	if startTime := c.Query("start_time"); startTime != "" {
		request.StartTime = &startTime
	}
	if endTime := c.Query("end_time"); endTime != "" {
		request.EndTime = &endTime
	}

	// Parse is_from_me filter
	if isFromMeStr := c.Query("is_from_me"); isFromMeStr != "" {
		isFromMe := c.QueryBool("is_from_me")
		request.IsFromMe = &isFromMe
	}

	response, err := controller.Service.GetChatMessages(c.UserContext(), request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success get chat messages",
		Results: response,
	})
}

func (controller *Chat) PinChat(c *fiber.Ctx) error {
	var request domainChat.PinChatRequest

	// Parse path parameter
	request.ChatJID = c.Params("chat_jid")

	// Parse JSON body
	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "Invalid request body",
			Results: nil,
		})
	}

	response, err := controller.Service.PinChat(c.UserContext(), request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Message,
		Results: response,
	})
}

func (controller *Chat) SetDisappearingTimer(c *fiber.Ctx) error {
	var request domainChat.SetDisappearingTimerRequest

	// Parse path parameter
	request.ChatJID = c.Params("chat_jid")

	// Parse JSON body
	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "Invalid request body",
			Results: nil,
		})
	}

	response, err := controller.Service.SetDisappearingTimer(c.UserContext(), request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Message,
		Results: response,
	})
}

func (controller *Chat) ExportStorage(c *fiber.Ctx) error {
	filePath, err := controller.Service.ExportStorage(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "EXPORT_ERROR",
			Message: err.Error(),
			Results: nil,
		})
	}

	// Set headers for file download
	filename := fmt.Sprintf("chatstorage-%s.db", time.Now().Format("20060102-150405"))
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Set("Content-Type", "application/octet-stream")

	return c.SendFile(filePath)
}

func (controller *Chat) ImportStorage(c *fiber.Ctx) error {
	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "No file uploaded. Please upload a SQLite backup file.",
			Results: nil,
		})
	}

	// Validate file extension
	ext := filepath.Ext(file.Filename)
	if ext != ".db" && ext != ".sqlite" && ext != ".sqlite3" {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "Invalid file type. Please upload a SQLite database file (.db, .sqlite, .sqlite3)",
			Results: nil,
		})
	}

	// Save to temp file
	tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("wa-import-%d%s", time.Now().UnixNano(), ext))
	if err := c.SaveFile(file, tempPath); err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "UPLOAD_ERROR",
			Message: "Failed to save uploaded file",
			Results: nil,
		})
	}
	defer os.Remove(tempPath) // Cleanup temp file after import

	// Perform import
	response, err := controller.Service.ImportStorage(c.UserContext(), tempPath)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "IMPORT_ERROR",
			Message: err.Error(),
			Results: nil,
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Message,
		Results: response,
	})
}

func (controller *Chat) AnalyzeStorage(c *fiber.Ctx) error {
	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "No file uploaded. Please upload a SQLite backup file.",
			Results: nil,
		})
	}

	// Validate file extension
	ext := filepath.Ext(file.Filename)
	if ext != ".db" && ext != ".sqlite" && ext != ".sqlite3" {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "Invalid file type. Please upload a SQLite database file (.db, .sqlite, .sqlite3)",
			Results: nil,
		})
	}

	// Save to temp file
	tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("wa-analyze-%d%s", time.Now().UnixNano(), ext))
	if err := c.SaveFile(file, tempPath); err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "UPLOAD_ERROR",
			Message: "Failed to save uploaded file",
			Results: nil,
		})
	}
	defer os.Remove(tempPath) // Cleanup temp file after analysis

	// Perform analysis
	response, err := controller.Service.AnalyzeStorage(c.UserContext(), tempPath)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "ANALYZE_ERROR",
			Message: err.Error(),
			Results: nil,
		})
	}

	// Use original filename in response
	response.Filename = file.Filename

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Analysis completed successfully",
		Results: response,
	})
}
