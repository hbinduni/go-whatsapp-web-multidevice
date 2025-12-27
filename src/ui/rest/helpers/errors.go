package helpers

import (
	"errors"
	"net/http"

	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

// HandleError returns a proper HTTP error response without panicking.
// It checks if the error implements GenericError for proper status codes,
// otherwise returns 500 Internal Server Error.
func HandleError(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	response := utils.ResponseData{
		Status:  http.StatusInternalServerError,
		Code:    "INTERNAL_SERVER_ERROR",
		Message: err.Error(),
	}

	// Check if error implements GenericError interface for proper status codes
	var genericErr pkgError.GenericError
	if errors.As(err, &genericErr) {
		response.Status = genericErr.StatusCode()
		response.Code = genericErr.ErrCode()
		response.Message = genericErr.Error()
	}

	return c.Status(response.Status).JSON(response)
}

// HandleBadRequest returns a 400 Bad Request error response
func HandleBadRequest(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusBadRequest).JSON(utils.ResponseData{
		Status:  http.StatusBadRequest,
		Code:    "BAD_REQUEST",
		Message: message,
	})
}

// HandleValidationError returns a 400 validation error response
func HandleValidationError(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusBadRequest).JSON(utils.ResponseData{
		Status:  http.StatusBadRequest,
		Code:    "VALIDATION_ERROR",
		Message: message,
	})
}

// HandleSuccess returns a 200 success response with optional data
func HandleSuccess(c *fiber.Ctx, message string, results any) error {
	return c.JSON(utils.ResponseData{
		Status:  http.StatusOK,
		Code:    "SUCCESS",
		Message: message,
		Results: results,
	})
}
