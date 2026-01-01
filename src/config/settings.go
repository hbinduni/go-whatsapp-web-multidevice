package config

import (
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
)

var (
	// Project version - independent fork of aldinokemal/go-whatsapp-web-multidevice
	// Diverged at upstream v7.11.0, now maintained independently with different architecture
	AppVersion        = "v3.0.0" // Major version bump for multi-client architecture
	AppPort           = "3000"
	AppHost           = "0.0.0.0"
	AppDebug          = false
	AppOs             = "GoWA-SSE" // Identifies this as the SSE-enabled fork
	AppPlatform       = waCompanionReg.DeviceProps_PlatformType(1)
	AppBasePath       = ""
	AppTrustedProxies []string // Trusted proxy IP ranges (e.g., "0.0.0.0/0" for all, or specific CIDRs)

	// Authentication configuration
	// AuthSecret is used to sign JWT tokens (generate with: openssl rand -base64 32)
	AuthSecret       = ""
	AuthUsername     = "admin"
	AuthPasswordHash = "" // bcrypt hash of password
	// Token expiry durations
	AuthAccessTokenExpiry  = "1h"  // Access token validity (e.g., "1h", "30m")
	AuthRefreshTokenExpiry = "168h" // Refresh token validity (7 days)

	PathQrCode    = "statics/qrcode"
	PathSendItems = "statics/senditems"
	PathStorages  = "storages"

	// Database configuration
	// For PostgreSQL: "postgresql://user:password@localhost:5432/dbname"
	// For SQLite: "file:storages/whatsapp.db?_foreign_keys=on"
	DBURI     = "file:storages/whatsapp.db?_foreign_keys=on"
	DBKeysURI = ""

	// Multi-client configuration
	// Comma-separated list of phone numbers to register as clients
	// Example: "+6281234567890,+6289876543210"
	WhatsAppClients = []string{}

	WhatsappAutoReplyMessage          string
	WhatsappAutoMarkRead              = false // Auto-mark incoming messages as read
	WhatsappAutoDownloadMedia         = true  // Auto-download media from incoming messages
	WhatsappWebhook                   []string
	WhatsappWebhookSecret                   = "secret"
	WhatsappWebhookInsecureSkipVerify       = false // Skip TLS certificate verification for webhooks (insecure)
	WhatsappLogLevel                        = "ERROR"
	WhatsappSettingMaxImageSize       int64 = 20000000  // 20MB
	WhatsappSettingMaxFileSize        int64 = 50000000  // 50MB
	WhatsappSettingMaxVideoSize       int64 = 100000000 // 100MB
	WhatsappSettingMaxDownloadSize    int64 = 500000000 // 500MB
	WhatsappTypeUser                        = "@s.whatsapp.net"
	WhatsappTypeGroup                       = "@g.us"
	WhatsappAccountValidation               = true

	// Chat storage database configuration
	// For PostgreSQL: "postgresql://user:password@localhost:5432/dbname"
	// For SQLite: "file:storages/chatstorage.db"
	ChatStorageURI               = "file:storages/chatstorage.db"
	ChatStorageEnableForeignKeys = true
	ChatStorageEnableWAL         = true

	// DatabaseURL is the primary database URL for PostgreSQL in multi-client mode
	// This replaces both DBURI and ChatStorageURI when using PostgreSQL
	DatabaseURL = ""

	// History Sync Configuration
	HistorySyncEnabled       = true // Enable or disable history sync processing
	HistorySyncOnLogin       = true // Automatically process history sync on login
	HistorySyncMaxDays int32 = 90   // Maximum days of history to process (default 90 days = 3 months)
	// Options: 90 (3 months), 365 (1 year), 730 (2 years), 1095 (3 years), -1 (all available)

	// S3/MinIO Configuration (required for media storage)
	S3Endpoint        = ""
	S3Region          = "us-east-1"
	S3AccessKeyID     = ""
	S3SecretAccessKey = ""
	S3Bucket          = ""
	S3ForcePathStyle  = false
	S3PublicURL       = ""    // Optional: custom public URL for direct media access
	S3UseServerProxy  = false // Use server download endpoint for private bucket access
)
