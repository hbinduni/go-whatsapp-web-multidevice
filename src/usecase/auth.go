package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainAuth "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/auth"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Token types for JWT claims
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// JWTClaims extends jwt.RegisteredClaims with custom fields
type JWTClaims struct {
	Username  string `json:"username"`
	TokenType string `json:"type"`
	jwt.RegisteredClaims
}

// AuthUsecase implements the auth use case
type AuthUsecase struct{}

// NewAuthUsecase creates a new auth usecase
func NewAuthUsecase() domainAuth.IAuthUsecase {
	return &AuthUsecase{}
}

// Login validates credentials and returns access token + sets refresh token
func (u *AuthUsecase) Login(ctx context.Context, req domainAuth.LoginRequest) (*domainAuth.LoginResponse, error) {
	// Validate username
	if req.Username != config.AuthUsername {
		return nil, errors.New("invalid credentials")
	}

	// Validate password against bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(config.AuthPasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Parse token expiry durations
	accessExpiry, err := time.ParseDuration(config.AuthAccessTokenExpiry)
	if err != nil {
		accessExpiry = time.Hour // Default to 1 hour
	}

	// Generate access token
	accessToken, expiresAt, err := u.generateToken(req.Username, TokenTypeAccess, accessExpiry)
	if err != nil {
		return nil, err
	}

	return &domainAuth.LoginResponse{
		Token:     accessToken,
		ExpiresAt: expiresAt,
	}, nil
}

// GenerateRefreshToken creates a refresh token for the given username
func (u *AuthUsecase) GenerateRefreshToken(username string) (string, time.Time, error) {
	refreshExpiry, err := time.ParseDuration(config.AuthRefreshTokenExpiry)
	if err != nil {
		refreshExpiry = 7 * 24 * time.Hour // Default to 7 days
	}

	return u.generateToken(username, TokenTypeRefresh, refreshExpiry)
}

// RefreshAccessToken generates a new access token from a valid refresh token
func (u *AuthUsecase) RefreshAccessToken(refreshToken string) (*domainAuth.LoginResponse, error) {
	// Validate refresh token
	claims, err := u.validateToken(refreshToken, TokenTypeRefresh)
	if err != nil {
		return nil, err
	}

	// Parse access token expiry
	accessExpiry, err := time.ParseDuration(config.AuthAccessTokenExpiry)
	if err != nil {
		accessExpiry = time.Hour
	}

	// Generate new access token
	accessToken, expiresAt, err := u.generateToken(claims.Username, TokenTypeAccess, accessExpiry)
	if err != nil {
		return nil, err
	}

	return &domainAuth.LoginResponse{
		Token:     accessToken,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateToken validates a JWT token and returns the claims
func (u *AuthUsecase) ValidateToken(ctx context.Context, tokenString string) (*domainAuth.Claims, error) {
	claims, err := u.validateToken(tokenString, TokenTypeAccess)
	if err != nil {
		return nil, err
	}

	return &domainAuth.Claims{
		Username: claims.Username,
	}, nil
}

// HashPassword hashes a password using bcrypt
func (u *AuthUsecase) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// generateToken creates a JWT token with the given claims
func (u *AuthUsecase) generateToken(username, tokenType string, expiry time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(expiry)

	claims := JWTClaims{
		Username:  username,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gowa-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(config.AuthSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return signedToken, expiresAt, nil
}

// validateToken parses and validates a JWT token
func (u *AuthUsecase) validateToken(tokenString string, expectedType string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(config.AuthSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token_expired")
		}
		return nil, errors.New("token_invalid")
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("token_invalid")
	}

	if claims.TokenType != expectedType {
		return nil, errors.New("token_invalid")
	}

	return claims, nil
}
