package auth

import (
	"context"
	"time"
)

// LoginRequest represents a login attempt
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse contains the JWT token after successful login
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Claims represents the JWT claims
type Claims struct {
	Username string `json:"username"`
}

// IAuthUsecase defines the authentication use case interface
type IAuthUsecase interface {
	// Login validates credentials and returns a JWT token
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)

	// ValidateToken validates a JWT token and returns the claims
	ValidateToken(ctx context.Context, token string) (*Claims, error)

	// HashPassword hashes a password using bcrypt
	HashPassword(password string) (string, error)
}
