package rest

import (
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainAuth "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/auth"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/usecase"
	"github.com/gofiber/fiber/v2"
)

// Auth handles authentication REST endpoints
type Auth struct {
	authUsecase *usecase.AuthUsecase
}

// InitRestAuth initializes authentication REST endpoints
func InitRestAuth(app fiber.Router) Auth {
	rest := Auth{
		authUsecase: usecase.NewAuthUsecase().(*usecase.AuthUsecase),
	}

	auth := app.Group("/auth")
	auth.Post("/login", rest.Login)
	auth.Post("/refresh", rest.Refresh)
	auth.Post("/logout", rest.Logout)

	return rest
}

// Login handles user authentication
// @Summary Login
// @Description Authenticate with username and password to get access token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body domainAuth.LoginRequest true "Login credentials"
// @Success 200 {object} domainAuth.LoginResponse
// @Router /auth/login [post]
func (controller *Auth) Login(c *fiber.Ctx) error {
	var req domainAuth.LoginRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"code":    fiber.StatusBadRequest,
			"error":   "invalid_request",
			"message": "Invalid request body",
		})
	}

	// Validate credentials and get access token
	response, err := controller.authUsecase.Login(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"code":    fiber.StatusUnauthorized,
			"error":   "invalid_credentials",
			"message": "Invalid username or password",
		})
	}

	// Generate refresh token
	refreshToken, refreshExpiry, err := controller.authUsecase.GenerateRefreshToken(req.Username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"code":    fiber.StatusInternalServerError,
			"error":   "token_error",
			"message": "Failed to generate refresh token",
		})
	}

	// Set refresh token as httpOnly cookie
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Expires:  refreshExpiry,
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		Path:     config.AppBasePath + "/auth",
	})

	// Also set access token as cookie for web UI convenience
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    response.Token,
		Expires:  response.ExpiresAt,
		HTTPOnly: false, // Accessible by JS for API calls
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		Path:     "/",
	})

	return c.JSON(fiber.Map{
		"code":    200,
		"message": "Login successful",
		"data": fiber.Map{
			"access_token": response.Token,
			"expires_at":   response.ExpiresAt,
			"token_type":   "Bearer",
		},
	})
}

// Refresh generates a new access token using the refresh token
// @Summary Refresh access token
// @Description Use refresh token to get a new access token
// @Tags Auth
// @Produce json
// @Success 200 {object} domainAuth.LoginResponse
// @Router /auth/refresh [post]
func (controller *Auth) Refresh(c *fiber.Ctx) error {
	// Get refresh token from cookie
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"code":    fiber.StatusUnauthorized,
			"error":   "refresh_expired",
			"message": "Session expired. Please login again.",
		})
	}

	// Generate new access token
	response, err := controller.authUsecase.RefreshAccessToken(refreshToken)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "token_expired" {
			// Clear expired refresh token cookie
			c.Cookie(&fiber.Cookie{
				Name:     "refresh_token",
				Value:    "",
				Expires:  time.Now().Add(-time.Hour),
				HTTPOnly: true,
				Secure:   c.Protocol() == "https",
				SameSite: "Strict",
				Path:     config.AppBasePath + "/auth",
			})

			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    fiber.StatusUnauthorized,
				"error":   "refresh_expired",
				"message": "Session expired. Please login again.",
			})
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"code":    fiber.StatusUnauthorized,
			"error":   "token_invalid",
			"message": "Invalid refresh token",
		})
	}

	// Extend refresh token (sliding session)
	newRefreshToken, refreshExpiry, err := controller.authUsecase.GenerateRefreshToken(config.AuthUsername)
	if err == nil {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    newRefreshToken,
			Expires:  refreshExpiry,
			HTTPOnly: true,
			Secure:   c.Protocol() == "https",
			SameSite: "Strict",
			Path:     config.AppBasePath + "/auth",
		})
	}

	// Update access token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    response.Token,
		Expires:  response.ExpiresAt,
		HTTPOnly: false,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		Path:     "/",
	})

	return c.JSON(fiber.Map{
		"code":    200,
		"message": "Token refreshed successfully",
		"data": fiber.Map{
			"access_token": response.Token,
			"expires_at":   response.ExpiresAt,
			"token_type":   "Bearer",
		},
	})
}

// Logout clears authentication tokens
// @Summary Logout
// @Description Clear authentication tokens and end session
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /auth/logout [post]
func (controller *Auth) Logout(c *fiber.Ctx) error {
	// Clear refresh token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		Path:     config.AppBasePath + "/auth",
	})

	// Clear access token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: false,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		Path:     "/",
	})

	return c.JSON(fiber.Map{
		"code":    200,
		"message": "Logged out successfully",
	})
}
