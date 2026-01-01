package middleware

import (
	"strings"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/usecase"
	"github.com/gofiber/fiber/v2"
)

// UsernameKey is the context key for storing the authenticated username
const UsernameKey = "auth_username"

// JWTAuth middleware validates JWT tokens from Authorization header or cookie
func JWTAuth() fiber.Handler {
	authUsecase := usecase.NewAuthUsecase()

	return func(c *fiber.Ctx) error {
		// Skip auth if not configured
		if config.AuthSecret == "" || config.AuthPasswordHash == "" {
			return c.Next()
		}

		var token string

		// Try to get token from Authorization header first
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// If no header token, try to get from cookie (for web UI)
		if token == "" {
			token = c.Cookies("access_token")
		}

		// No token found
		if token == "" {
			return unauthorizedResponse(c, "missing_token", "Authentication required")
		}

		// Validate token
		claims, err := authUsecase.ValidateToken(c.Context(), token)
		if err != nil {
			errMsg := err.Error()
			if errMsg == "token_expired" {
				return unauthorizedResponse(c, "token_expired", "Access token expired. Use /auth/refresh to get a new token.")
			}
			return unauthorizedResponse(c, "token_invalid", "Invalid authentication token")
		}

		// Store username in context for later use
		c.Locals(UsernameKey, claims.Username)

		return c.Next()
	}
}

// OptionalJWTAuth middleware validates JWT if present but doesn't require it
// Useful for routes that have different behavior for authenticated vs anonymous users
func OptionalJWTAuth() fiber.Handler {
	authUsecase := usecase.NewAuthUsecase()

	return func(c *fiber.Ctx) error {
		// Skip if auth not configured
		if config.AuthSecret == "" || config.AuthPasswordHash == "" {
			return c.Next()
		}

		var token string

		// Try to get token from Authorization header first
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// If no header token, try to get from cookie
		if token == "" {
			token = c.Cookies("access_token")
		}

		// If token exists, validate it
		if token != "" {
			claims, err := authUsecase.ValidateToken(c.Context(), token)
			if err == nil {
				c.Locals(UsernameKey, claims.Username)
			}
		}

		return c.Next()
	}
}

// GetAuthUsername retrieves the authenticated username from context
func GetAuthUsername(c *fiber.Ctx) string {
	if username, ok := c.Locals(UsernameKey).(string); ok {
		return username
	}
	return ""
}

// IsAuthenticated checks if the request has valid authentication
func IsAuthenticated(c *fiber.Ctx) bool {
	return GetAuthUsername(c) != ""
}

// unauthorizedResponse sends a standardized 401 response
func unauthorizedResponse(c *fiber.Ctx, errorCode, message string) error {
	// Check if this is an API request or browser request
	accept := c.Get("Accept")
	if strings.Contains(accept, "text/html") && !strings.Contains(c.Path(), "/api/") {
		// Redirect to login page for browser requests
		return c.Redirect(config.AppBasePath + "/login")
	}

	// Return JSON for API requests
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"code":    fiber.StatusUnauthorized,
		"error":   errorCode,
		"message": message,
	})
}
