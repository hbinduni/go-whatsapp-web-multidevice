package middleware

import (
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/gofiber/fiber/v2"
)

// ContextKey types for storing values in fiber context
type ContextKey string

const (
	// ClientKey is the key for storing the ManagedClient in fiber Locals
	ClientKey ContextKey = "wa_client"
	// DeviceIDKey is the key for storing the device/phone ID in fiber Locals
	DeviceIDKey ContextKey = "device_id"
)

// MultiClientMiddleware extracts the phone number from the URL and sets the client in context
// This middleware should be used for routes that need client-specific handling
// It injects the ManagedClient into both fiber.Locals AND Go's context.Context
// so that usecases can access the client via whatsapp.GetClientFromContext(ctx)
func MultiClientMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if we're in multi-client mode
		if !whatsapp.IsMultiClientMode() {
			// Single-client mode - inject global client into context
			client := whatsapp.GetClient()
			if client != nil {
				c.Locals(string(ClientKey), client)
				// Inject into Go context for usecase access
				ctx := whatsapp.ContextWithClient(c.UserContext(), client)
				c.SetUserContext(ctx)
			}
			return c.Next()
		}

		// Get phone from URL parameter
		phone := c.Params("phone")
		if phone == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"code":    fiber.StatusBadRequest,
				"message": "Phone number is required in multi-client mode",
			})
		}

		// Get the managed client for this phone
		registry := whatsapp.GetRegistry()
		if registry == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"code":    fiber.StatusInternalServerError,
				"message": "Client registry not initialized",
			})
		}

		managedClient, err := registry.GetClient(phone)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"code":    fiber.StatusNotFound,
				"message": "Client not found for phone: " + phone,
			})
		}

		// Store client in fiber Locals (for direct access in handlers)
		c.Locals(string(ClientKey), managedClient)
		c.Locals(string(DeviceIDKey), phone)

		// Inject ManagedClient into Go context (for usecase access)
		// This allows usecases to call whatsapp.GetClientFromContext(ctx)
		ctx := whatsapp.ContextWithManagedClient(c.UserContext(), managedClient)
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// GetManagedClient extracts the ManagedClient from the fiber context
func GetManagedClient(c *fiber.Ctx) *whatsapp.ManagedClient {
	if mc, ok := c.Locals(string(ClientKey)).(*whatsapp.ManagedClient); ok {
		return mc
	}
	return nil
}

// GetDeviceID extracts the device/phone ID from the fiber context
func GetDeviceID(c *fiber.Ctx) string {
	if id, ok := c.Locals(string(DeviceIDKey)).(string); ok {
		return id
	}
	return ""
}

// RequireLoggedIn middleware checks if the client is logged in
func RequireLoggedIn() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !whatsapp.IsMultiClientMode() {
			// Single-client mode - check global client
			_, isLoggedIn, _ := whatsapp.GetConnectionStatus()
			if !isLoggedIn {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"code":    fiber.StatusUnauthorized,
					"message": "WhatsApp client is not logged in",
				})
			}
			return c.Next()
		}

		// Multi-client mode - check the specific client
		mc := GetManagedClient(c)
		if mc == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"code":    fiber.StatusInternalServerError,
				"message": "Client not found in context",
			})
		}

		if !mc.IsLoggedIn() {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    fiber.StatusUnauthorized,
				"message": "WhatsApp client " + mc.Phone + " is not logged in",
			})
		}

		return c.Next()
	}
}
