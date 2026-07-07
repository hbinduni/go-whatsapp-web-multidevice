package config

import (
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
)

var (
	// Project version - independent fork of aldinokemal/go-whatsapp-web-multidevice
	// Diverged at upstream v7.11.0, now maintained independently with different architecture
	AppVersion             = "v2.0.19"
	AppPort                = "3000"
	AppHost                = "0.0.0.0"
	AppDebug               = false
	AppOs                  = "GoWA-SSE" // Identifies this as the SSE-enabled fork
	AppPlatform            = waCompanionReg.DeviceProps_PlatformType(1)
	AppBasicAuthCredential []string
	AppBasePath            = ""
	AppTrustedProxies      []string // Trusted proxy IP ranges (e.g., "0.0.0.0/0" for all, or specific CIDRs)

	// CORS Configuration
	CORSAllowOrigins     = "" // Comma-separated list of allowed origins (empty = same-origin only, "*" = all origins)
	CORSAllowHeaders     = "Origin, Content-Type, Accept, Authorization"
	CORSAllowMethods     = "GET, POST, PUT, DELETE, OPTIONS"
	CORSAllowCredentials = false // Allow credentials (cookies, authorization headers)

	// Rate Limiting Configuration
	RateLimitEnabled    = true // Enable rate limiting
	RateLimitMax        = 100  // Maximum requests per window
	RateLimitWindowSecs = 60   // Time window in seconds

	PathQrCode    = "statics/qrcode"
	PathSendItems = "statics/senditems"
	PathStorages  = "storages"

	DBURI     = "file:storages/whatsapp.db?_foreign_keys=on"
	DBKeysURI = ""

	WhatsappAutoReplyMessage          string
	WhatsappAutoMarkRead              = false // Auto-mark incoming messages as read
	WhatsappPresenceAvailable         = false // Mark this device "available" (online) on connect; when true the linked phone stops showing message notifications
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

	ChatStorageURI               = "file:storages/chatstorage.db"
	ChatStorageEnableForeignKeys = true
	ChatStorageEnableWAL         = true

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
