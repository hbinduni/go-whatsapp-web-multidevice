package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainAdmin "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/admin"
	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainChat "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chat"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	domainGroup "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/group"
	domainHistory "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/history"
	domainMessage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/message"
	domainNewsletter "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/newsletter"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/storage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/sse"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/websocket"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Chat Storage
	chatStorageDB   *sql.DB
	chatStorageRepo domainChatStorage.IChatStorageRepository

	// Usecase
	appUsecase        domainApp.IAppUsecase
	adminUsecase      domainAdmin.IAdminUsecase
	chatUsecase       domainChat.IChatUsecase
	sendUsecase       domainSend.ISendUsecase
	userUsecase       domainUser.IUserUsecase
	messageUsecase    domainMessage.IMessageUsecase
	groupUsecase      domainGroup.IGroupUsecase
	newsletterUsecase domainNewsletter.INewsletterUsecase
	historyUsecase    domainHistory.IHistoryUsecase
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "whatsapp",
	Short: fmt.Sprintf("GoWA-SSE: WhatsApp Web API with SSE support (%s)", config.AppVersion),
	Long: fmt.Sprintf(`GoWA-SSE - WhatsApp Web API Server
Version: %s

Features: SSE real-time events, S3 media storage, history sync, chat storage API.
Originally forked from aldinokemal/go-whatsapp-web-multidevice, now maintained independently.

Send WhatsApp messages over HTTP API. Requires WhatsApp multi-device account.`, config.AppVersion),
	Run: runServer,
}

// runServer starts the REST API server
func runServer(_ *cobra.Command, _ []string) {
	logrus.Infof("========================================")
	logrus.Infof("  GoWA-SSE: WhatsApp Web API")
	logrus.Infof("  Version: %s", config.AppVersion)
	logrus.Infof("========================================")

	// Validate auth configuration
	if config.AuthSecret == "" {
		logrus.Warn("⚠️  AUTH_SECRET not set - authentication disabled. Generate with: openssl rand -base64 32")
	}
	if config.AuthPasswordHash == "" && config.AuthSecret != "" {
		logrus.Warn("⚠️  AUTH_PASSWORD_HASH not set - authentication disabled")
	}

	fiberConfig := fiber.Config{
		EnableTrustedProxyCheck: true,
		BodyLimit:               int(config.WhatsappSettingMaxVideoSize),
		Network:                 "tcp",
	}

	// Configure proxy settings if trusted proxies are specified
	if len(config.AppTrustedProxies) > 0 {
		fiberConfig.TrustedProxies = config.AppTrustedProxies
		fiberConfig.ProxyHeader = fiber.HeaderXForwardedHost
	}

	app := fiber.New(fiberConfig)

	// Static assets (public)
	app.Static(config.AppBasePath+"/statics", "./statics")

	// Global middleware
	app.Use(middleware.Recovery())
	if config.AppDebug {
		app.Use(logger.New())
	}
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			// If no origins configured, allow all (development mode)
			if len(config.CorsAllowedOrigins) == 0 {
				return true
			}
			// Check against whitelist
			for _, allowed := range config.CorsAllowedOrigins {
				if origin == allowed {
					return true
				}
			}
			logrus.Warnf("[CORS] Blocked origin: %s", origin)
			return false
		},
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
	}))

	// Log CORS configuration
	if len(config.CorsAllowedOrigins) == 0 {
		logrus.Warn("⚠️  CORS_ALLOWED_ORIGINS not set - allowing all origins (not recommended for production)")
	} else {
		logrus.Infof("CORS allowed origins: %v", config.CorsAllowedOrigins)
	}

	// Create base path group
	var baseGroup fiber.Router = app
	if config.AppBasePath != "" {
		baseGroup = app.Group(config.AppBasePath)
	}

	// API info route (no auth required)
	baseGroup.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"name":    "GoWA-SSE WhatsApp Web API",
			"version": config.AppVersion,
			"status":  "running",
			"docs":    "Use a separate webclient to interact with this API",
		})
	})

	// Auth endpoints (public)
	rest.InitRestAuth(baseGroup)

	// Protected routes (JWT auth required)
	protectedGroup := baseGroup.Group("", middleware.JWTAuth())

	// Admin routes (protected)
	rest.InitRestAdmin(protectedGroup, adminUsecase)

	// API routes (protected, per-phone)
	logrus.Info("Registering routes with /api/:phone/ prefix")
	phoneGroup := protectedGroup.Group("/api/:phone", middleware.MultiClientMiddleware())

	rest.InitRestApp(phoneGroup, appUsecase)
	rest.InitRestChat(phoneGroup, chatUsecase)
	rest.InitRestSend(phoneGroup, sendUsecase)
	rest.InitRestUser(phoneGroup, userUsecase)
	rest.InitRestMessage(phoneGroup, messageUsecase)
	rest.InitRestGroup(phoneGroup, groupUsecase)
	rest.InitRestNewsletter(phoneGroup, newsletterUsecase)
	rest.InitRestHistory(phoneGroup, historyUsecase)
	rest.InitRestMedia(phoneGroup)

	// WebSocket routes (with optional auth for backward compatibility)
	websocket.RegisterRoutes(baseGroup, appUsecase)
	go websocket.RunHub()

	// SSE routes
	sse.RegisterRoutes(baseGroup)
	go sse.GetHub().Run()
	logrus.Info("SSE hub started - endpoint: /events")

	// Auto-connect after booting
	go helpers.SetAutoConnectAfterBooting(appUsecase)
	startAutoReconnectCheckerIfClientAvailable()

	// Setup graceful shutdown
	shutdownChan := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		logrus.Infof("Received signal %v, initiating graceful shutdown...", sig)

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.ShutdownTimeout)*time.Second)
		defer cancel()

		// Stop accepting new connections and wait for existing ones
		if err := app.ShutdownWithContext(ctx); err != nil {
			logrus.Errorf("Server shutdown error: %v", err)
		}

		// Stop background services
		logrus.Info("Stopping background services...")
		websocket.StopHub()
		sse.GetHub().Stop()
		helpers.StopAutoReconnectChecking()

		// Close database connections
		if chatStorageDB != nil {
			logrus.Info("Closing database connections...")
			if err := chatStorageDB.Close(); err != nil {
				logrus.Errorf("Error closing database: %v", err)
			}
		}

		logrus.Info("Graceful shutdown complete")
		close(shutdownChan)
	}()

	logrus.Infof("🌐 Server listening on port %s", config.AppPort)
	if err := app.Listen(":" + config.AppPort); err != nil {
		logrus.Fatalln("Failed to start: ", err.Error())
	}

	// Wait for shutdown to complete
	<-shutdownChan
}

func init() {
	// Load environment variables first (ignore error as .env is optional)
	_ = utils.LoadConfig(".")

	time.Local = time.UTC

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Initialize flags first, before any subcommands are added
	initFlags()

	// Then initialize other components
	cobra.OnInitialize(initEnvConfig, initApp)
}

// initEnvConfig loads configuration from environment variables
func initEnvConfig() {
	// Application settings
	if envPort := viper.GetString("app_port"); envPort != "" {
		config.AppPort = envPort
	}
	if envDebug := viper.GetBool("app_debug"); envDebug {
		config.AppDebug = envDebug
	}
	if envOs := viper.GetString("app_os"); envOs != "" {
		config.AppOs = envOs
	}
	if envBasePath := viper.GetString("app_base_path"); envBasePath != "" {
		config.AppBasePath = envBasePath
	}
	if envPublicURL := viper.GetString("app_public_url"); envPublicURL != "" {
		config.AppPublicURL = envPublicURL
	}
	if envTrustedProxies := viper.GetString("app_trusted_proxies"); envTrustedProxies != "" {
		proxies := strings.Split(envTrustedProxies, ",")
		config.AppTrustedProxies = proxies
	}

	// Authentication settings
	if envAuthSecret := viper.GetString("auth_secret"); envAuthSecret != "" {
		config.AuthSecret = envAuthSecret
	}
	if envAuthUsername := viper.GetString("auth_username"); envAuthUsername != "" {
		config.AuthUsername = envAuthUsername
	}
	if envAuthPasswordHash := viper.GetString("auth_password_hash"); envAuthPasswordHash != "" {
		config.AuthPasswordHash = envAuthPasswordHash
	}
	if envAccessExpiry := viper.GetString("auth_access_token_expiry"); envAccessExpiry != "" {
		config.AuthAccessTokenExpiry = envAccessExpiry
	}
	if envRefreshExpiry := viper.GetString("auth_refresh_token_expiry"); envRefreshExpiry != "" {
		config.AuthRefreshTokenExpiry = envRefreshExpiry
	}

	// Database settings
	if envDBURI := viper.GetString("db_uri"); envDBURI != "" {
		config.DBURI = envDBURI
	}
	if envDBKEYSURI := viper.GetString("db_keys_uri"); envDBKEYSURI != "" {
		config.DBKeysURI = envDBKEYSURI
	}
	if envDatabaseURL := viper.GetString("database_url"); envDatabaseURL != "" {
		config.DatabaseURL = envDatabaseURL
		// Use same URL for WhatsApp DB and Chat Storage in PostgreSQL mode
		config.DBURI = envDatabaseURL
		config.ChatStorageURI = envDatabaseURL
	}

	// Multi-client settings
	if envClients := viper.GetString("whatsapp_clients"); envClients != "" {
		clients := strings.Split(envClients, ",")
		for i, c := range clients {
			clients[i] = strings.TrimSpace(c)
		}
		config.WhatsAppClients = clients
	}

	// CORS settings
	if envCorsOrigins := viper.GetString("cors_allowed_origins"); envCorsOrigins != "" {
		origins := strings.Split(envCorsOrigins, ",")
		for i, o := range origins {
			origins[i] = strings.TrimSpace(o)
		}
		config.CorsAllowedOrigins = origins
	}

	// Shutdown timeout
	if viper.IsSet("shutdown_timeout") {
		config.ShutdownTimeout = viper.GetInt("shutdown_timeout")
	}

	// FFmpeg/Media processing timeouts
	if viper.IsSet("ffmpeg_thumbnail_timeout") {
		config.FFmpegThumbnailTimeout = viper.GetInt("ffmpeg_thumbnail_timeout")
	}
	if viper.IsSet("ffmpeg_compress_timeout") {
		config.FFmpegCompressTimeout = viper.GetInt("ffmpeg_compress_timeout")
	}
	if viper.IsSet("ffmpeg_convert_timeout") {
		config.FFmpegConvertTimeout = viper.GetInt("ffmpeg_convert_timeout")
	}

	// WhatsApp settings
	if envAutoReply := viper.GetString("whatsapp_auto_reply"); envAutoReply != "" {
		config.WhatsappAutoReplyMessage = envAutoReply
	}
	if viper.IsSet("whatsapp_auto_mark_read") {
		config.WhatsappAutoMarkRead = viper.GetBool("whatsapp_auto_mark_read")
	}
	if viper.IsSet("whatsapp_auto_download_media") {
		config.WhatsappAutoDownloadMedia = viper.GetBool("whatsapp_auto_download_media")
	}
	if envWebhook := viper.GetString("whatsapp_webhook"); envWebhook != "" {
		webhook := strings.Split(envWebhook, ",")
		config.WhatsappWebhook = webhook
	}
	if envWebhookSecret := viper.GetString("whatsapp_webhook_secret"); envWebhookSecret != "" {
		config.WhatsappWebhookSecret = envWebhookSecret
	}
	if viper.IsSet("whatsapp_webhook_insecure_skip_verify") {
		config.WhatsappWebhookInsecureSkipVerify = viper.GetBool("whatsapp_webhook_insecure_skip_verify")
	}
	if viper.IsSet("whatsapp_account_validation") {
		config.WhatsappAccountValidation = viper.GetBool("whatsapp_account_validation")
	}

	// History Sync settings
	if viper.IsSet("whatsapp_history_sync_enabled") {
		config.HistorySyncEnabled = viper.GetBool("whatsapp_history_sync_enabled")
	}
	if viper.IsSet("whatsapp_history_sync_on_login") {
		config.HistorySyncOnLogin = viper.GetBool("whatsapp_history_sync_on_login")
	}
	if viper.IsSet("whatsapp_history_sync_max_days") {
		config.HistorySyncMaxDays = viper.GetInt32("whatsapp_history_sync_max_days")
	}

	// S3/MinIO settings (required for media storage)
	if envS3Endpoint := viper.GetString("s3_endpoint"); envS3Endpoint != "" {
		config.S3Endpoint = envS3Endpoint
	}
	if envS3Region := viper.GetString("s3_region"); envS3Region != "" {
		config.S3Region = envS3Region
	}
	if envS3AccessKeyID := viper.GetString("s3_access_key_id"); envS3AccessKeyID != "" {
		config.S3AccessKeyID = envS3AccessKeyID
	}
	if envS3SecretAccessKey := viper.GetString("s3_secret_access_key"); envS3SecretAccessKey != "" {
		config.S3SecretAccessKey = envS3SecretAccessKey
	}
	if envS3Bucket := viper.GetString("s3_bucket"); envS3Bucket != "" {
		config.S3Bucket = envS3Bucket
	}
	if viper.IsSet("s3_force_path_style") {
		config.S3ForcePathStyle = viper.GetBool("s3_force_path_style")
	}
	if envS3PublicURL := viper.GetString("s3_public_url"); envS3PublicURL != "" {
		config.S3PublicURL = envS3PublicURL
	}
	if viper.IsSet("s3_use_server_proxy") {
		config.S3UseServerProxy = viper.GetBool("s3_use_server_proxy")
	}
}

func initFlags() {
	// Application flags
	rootCmd.PersistentFlags().StringVarP(
		&config.AppPort,
		"port", "p",
		config.AppPort,
		"change port number with --port <number> | example: --port=8080",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&config.AppDebug,
		"debug", "d",
		config.AppDebug,
		"hide or displaying log with --debug <true/false> | example: --debug=true",
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.AppOs,
		"os", "",
		config.AppOs,
		`os name --os <string> | example: --os="Chrome"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.AppBasePath,
		"base-path", "",
		config.AppBasePath,
		`base path for subpath deployment --base-path <string> | example: --base-path="/gowa"`,
	)

	// Authentication flags
	rootCmd.PersistentFlags().StringVarP(
		&config.AuthSecret,
		"auth-secret", "",
		config.AuthSecret,
		`JWT signing secret (generate with: openssl rand -base64 32)`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.AuthUsername,
		"auth-username", "",
		config.AuthUsername,
		`admin username for login`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.AuthPasswordHash,
		"auth-password-hash", "",
		config.AuthPasswordHash,
		`bcrypt hash of admin password`,
	)
	rootCmd.PersistentFlags().StringSliceVarP(
		&config.AppTrustedProxies,
		"trusted-proxies", "",
		config.AppTrustedProxies,
		`trusted proxy IP ranges for reverse proxy deployments --trusted-proxies <string> | example: --trusted-proxies="0.0.0.0/0" or --trusted-proxies="10.0.0.0/8,172.16.0.0/12"`,
	)

	// Database flags
	rootCmd.PersistentFlags().StringVarP(
		&config.DBURI,
		"db-uri", "",
		config.DBURI,
		`PostgreSQL database URI for WhatsApp connection data. --db-uri <string> | example: --db-uri="postgres://user:password@localhost:5432/whatsapp"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.DBKeysURI,
		"db-keys-uri", "",
		config.DBKeysURI,
		`PostgreSQL database URI for keys storage (optional, defaults to db-uri). --db-keys-uri <string>`,
	)

	// WhatsApp flags
	rootCmd.PersistentFlags().StringVarP(
		&config.WhatsappAutoReplyMessage,
		"autoreply", "",
		config.WhatsappAutoReplyMessage,
		`auto reply when received message --autoreply <string> | example: --autoreply="Don't reply this message"`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&config.WhatsappAutoMarkRead,
		"auto-mark-read", "",
		config.WhatsappAutoMarkRead,
		`auto mark incoming messages as read --auto-mark-read <true/false> | example: --auto-mark-read=true`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&config.WhatsappAutoDownloadMedia,
		"auto-download-media", "",
		config.WhatsappAutoDownloadMedia,
		`auto download media from incoming messages --auto-download-media <true/false> | example: --auto-download-media=false`,
	)
	rootCmd.PersistentFlags().StringSliceVarP(
		&config.WhatsappWebhook,
		"webhook", "w",
		config.WhatsappWebhook,
		`forward event to webhook --webhook <string> | example: --webhook="https://yourcallback.com/callback"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.WhatsappWebhookSecret,
		"webhook-secret", "",
		config.WhatsappWebhookSecret,
		`secure webhook request --webhook-secret <string> | example: --webhook-secret="super-secret-key"`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&config.WhatsappWebhookInsecureSkipVerify,
		"webhook-insecure-skip-verify", "",
		config.WhatsappWebhookInsecureSkipVerify,
		`skip TLS certificate verification for webhooks (INSECURE - use only for development/self-signed certs) --webhook-insecure-skip-verify <true/false> | example: --webhook-insecure-skip-verify=true`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&config.WhatsappAccountValidation,
		"account-validation", "",
		config.WhatsappAccountValidation,
		`enable or disable account validation --account-validation <true/false> | example: --account-validation=true`,
	)
}

func initChatStorage() (*sql.DB, error) {
	// Validate PostgreSQL URI
	if !strings.HasPrefix(config.DatabaseURL, "postgres://") && !strings.HasPrefix(config.DatabaseURL, "postgresql://") {
		return nil, fmt.Errorf("DATABASE_URL must be a PostgreSQL connection string (postgres:// or postgresql://)")
	}

	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		return nil, err
	}

	// Configure connection pool for PostgreSQL
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	logrus.Info("Connected to PostgreSQL database")
	return db, nil
}

func initApp() {
	// Configure log formatter with timestamps
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	if config.AppDebug {
		config.WhatsappLogLevel = "DEBUG"
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Preparing folder if not exist
	err := utils.CreateFolder(config.PathQrCode, config.PathSendItems, config.PathStorages)
	if err != nil {
		logrus.Errorln(err)
	}

	// Validate required configuration
	if config.DatabaseURL == "" {
		logrus.Fatal("DATABASE_URL is required. Set it to a PostgreSQL connection string.")
	}
	if len(config.WhatsAppClients) == 0 {
		logrus.Fatal("WHATSAPP_CLIENTS is required. Set it to a comma-separated list of phone numbers.")
	}

	ctx := context.Background()

	// Initialize S3 storage (required for media)
	if storage.IsS3ConfigValid() {
		s3Config := &storage.S3Config{
			Endpoint:        config.S3Endpoint,
			Region:          config.S3Region,
			AccessKeyID:     config.S3AccessKeyID,
			SecretAccessKey: config.S3SecretAccessKey,
			Bucket:          config.S3Bucket,
			ForcePathStyle:  config.S3ForcePathStyle,
			PublicURL:       config.S3PublicURL,
			UseServerProxy:  config.S3UseServerProxy,
		}
		if err := storage.InitStorage(s3Config); err != nil {
			logrus.Fatalf("failed to initialize S3 storage: %v", err)
		}
	} else {
		logrus.Warn("S3 storage not configured - media download/upload features will be disabled")
	}

	// Initialize multi-client mode
	initClients(ctx)

	// Initialize usecases
	appUsecase = usecase.NewAppService(chatStorageRepo)
	adminUsecase = usecase.NewAdminService(chatStorageRepo)
	chatUsecase = usecase.NewChatService(chatStorageRepo)
	sendUsecase = usecase.NewSendService(appUsecase, chatStorageRepo)
	userUsecase = usecase.NewUserService()
	messageUsecase = usecase.NewMessageService(chatStorageRepo)
	groupUsecase = usecase.NewGroupService()
	newsletterUsecase = usecase.NewNewsletterService()
	historyUsecase = usecase.NewHistoryService()
}

// initClients initializes all WhatsApp clients
func initClients(ctx context.Context) {
	logrus.Info("========================================")
	logrus.Info("  Multi-Client WhatsApp API")
	logrus.Infof("  Clients: %v", config.WhatsAppClients)
	logrus.Info("========================================")

	// Enable multi-client mode in whatsapp package
	whatsapp.SetMultiClientMode(true)

	var err error

	// Initialize shared database connection
	chatStorageDB, err = initChatStorage()
	if err != nil {
		logrus.Fatalf("failed to initialize chat storage: %v", err)
	}

	// Initialize WhatsApp database (shared for all clients)
	whatsappDB := whatsapp.InitWaDB(ctx, config.DatabaseURL)

	// Initialize the client registry
	whatsapp.InitRegistry(whatsappDB, nil)
	registry := whatsapp.GetRegistry()

	// Register each client
	for _, phone := range config.WhatsAppClients {
		phone = strings.TrimSpace(phone)
		if phone == "" {
			continue
		}

		// Validate phone number format early
		normalizedPhone, err := whatsapp.ValidatePhone(phone)
		if err != nil {
			logrus.Fatalf("Invalid phone number in WHATSAPP_CLIENTS: %v", err)
		}

		// Use normalized phone for consistency
		phone = normalizedPhone

		// Create a device-specific chat storage repository
		clientChatStorageRepo := chatstorage.NewPostgresRepository(chatStorageDB, phone)

		// Initialize schema for this client
		if err := clientChatStorageRepo.InitializeSchema(); err != nil {
			logrus.Errorf("Failed to initialize schema for client %s: %v", phone, err)
			continue
		}

		// Register the client
		mc, err := registry.RegisterClient(ctx, phone, clientChatStorageRepo)
		if err != nil {
			logrus.Errorf("Failed to register client %s: %v", phone, err)
			continue
		}

		logrus.Infof("Registered client: %s", phone)

		// Set the first client's repo as the default
		if chatStorageRepo == nil {
			chatStorageRepo = clientChatStorageRepo
		}

		// Try to connect if already logged in
		if mc.Client != nil {
			go func(client *whatsapp.ManagedClient) {
				if err := client.Client.Connect(); err != nil {
					logrus.Warnf("Failed to connect client %s: %v", client.Phone, err)
				} else {
					if client.Client.IsLoggedIn() {
						client.SetStatus(whatsapp.StatusLoggedIn)
						logrus.Infof("Client %s connected and logged in", client.Phone)
					} else {
						client.SetStatus(whatsapp.StatusConnected)
						logrus.Infof("Client %s connected (not logged in yet)", client.Phone)
					}
				}
			}(mc)
		}
	}

	// Ensure we have at least a fallback chat storage repo
	if chatStorageRepo == nil {
		chatStorageRepo = chatstorage.NewPostgresRepository(chatStorageDB, "default")
		if err := chatStorageRepo.InitializeSchema(); err != nil {
			logrus.Fatalf("failed to initialize fallback chat storage schema: %v", err)
		}
	}

	logrus.Infof("Initialization complete. Registered %d clients", registry.GetClientCount())
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
