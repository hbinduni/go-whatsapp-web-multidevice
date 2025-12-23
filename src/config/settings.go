package config

import (
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
)

var (
	AppVersion             = "v7.11.0"    // Upstream version from aldinokemal
	AppForkVersion         = "v1.2.10-sse" // Fork version (binduni's version)
	AppPort                = "3000"
	AppDebug               = false
	AppOs                  = "AldinoKemal"
	AppPlatform            = waCompanionReg.DeviceProps_PlatformType(1)
	AppBasicAuthCredential []string
	AppBasePath            = ""
	AppTrustedProxies      []string // Trusted proxy IP ranges (e.g., "0.0.0.0/0" for all, or specific CIDRs)

	McpPort = "8080"
	McpHost = "localhost"

	PathQrCode    = "statics/qrcode"
	PathSendItems = "statics/senditems"
	PathMedia     = "statics/media"
	PathStorages  = "storages"

	DBURI     = "file:storages/whatsapp.db?_foreign_keys=on"
	DBKeysURI = ""

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

	ChatStorageURI               = "file:storages/chatstorage.db"
	ChatStorageEnableForeignKeys = true
	ChatStorageEnableWAL         = true

	// History Sync Configuration
	HistorySyncEnabled       = true // Enable or disable history sync processing
	HistorySyncOnLogin       = true // Automatically process history sync on login
	HistorySyncMaxDays int32 = 90   // Maximum days of history to process (default 90 days = 3 months)
	// Options: 90 (3 months), 365 (1 year), 730 (2 years), 1095 (3 years), -1 (all available)

	// Media Storage Configuration
	MediaStorageType = "local" // "local" or "s3"

	// S3/MinIO Configuration
	S3Endpoint        = ""
	S3Region          = "us-east-1"
	S3AccessKeyID     = ""
	S3SecretAccessKey = ""
	S3Bucket          = ""
	S3ForcePathStyle  = false
	S3PublicURL       = ""
	S3UseServerProxy  = false // Use server download endpoint for private bucket access
)
