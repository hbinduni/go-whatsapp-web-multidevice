package cmd

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"os"
	"strings"
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
	"github.com/aldinokemal/go-whatsapp-web-multidevice/usecase"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.mau.fi/whatsmeow"
)

var (
	EmbedIndex embed.FS
	EmbedViews embed.FS

	// Whatsapp
	whatsappCli *whatsmeow.Client

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
	Short: fmt.Sprintf("GoWA-SSE: WhatsApp Web API with SSE support (%s)", config.AppVersion),
	Long: fmt.Sprintf(`GoWA-SSE - WhatsApp Web API Server
Version: %s

Features: SSE real-time events, S3 media storage, history sync, chat storage API.
Originally forked from aldinokemal/go-whatsapp-web-multidevice, now maintained independently.

Send WhatsApp messages over HTTP API. Requires WhatsApp multi-device account.`, config.AppVersion),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Print version banner on startup (only for rest command, not help)
		if cmd.Name() == "rest" {
			logrus.Infof("========================================")
			logrus.Infof("  GoWA-SSE: WhatsApp Web API")
			logrus.Infof("  Version: %s", config.AppVersion)
			logrus.Infof("========================================")
		}
	},
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
	if envBasicAuth := viper.GetString("app_basic_auth"); envBasicAuth != "" {
		credential := strings.Split(envBasicAuth, ",")
		config.AppBasicAuthCredential = credential
	}
	if envBasePath := viper.GetString("app_base_path"); envBasePath != "" {
		config.AppBasePath = envBasePath
	}
	if envTrustedProxies := viper.GetString("app_trusted_proxies"); envTrustedProxies != "" {
		proxies := strings.Split(envTrustedProxies, ",")
		config.AppTrustedProxies = proxies
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
	rootCmd.PersistentFlags().StringSliceVarP(
		&config.AppBasicAuthCredential,
		"basic-auth", "b",
		config.AppBasicAuthCredential,
		"basic auth credential | -b=yourUsername:yourPassword",
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.AppBasePath,
		"base-path", "",
		config.AppBasePath,
		`base path for subpath deployment --base-path <string> | example: --base-path="/gowa"`,
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
		`the database uri to store the connection data database uri (by default, we'll use sqlite3 under storages/whatsapp.db). database uri --db-uri <string> | example: --db-uri="file:storages/whatsapp.db?_foreign_keys=on or postgres://user:password@localhost:5432/whatsapp"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.DBKeysURI,
		"db-keys-uri", "",
		config.DBKeysURI,
		`the database uri to store the keys database uri (by default, we'll use the same database uri). database uri --db-keys-uri <string> | example: --db-keys-uri="file::memory:?cache=shared&_foreign_keys=on"`,
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
	// Check if using PostgreSQL (multi-client mode)
	if isPostgreSQLURI(config.ChatStorageURI) {
		db, err := sql.Open("postgres", config.ChatStorageURI)
		if err != nil {
			return nil, err
		}

		// Configure connection pool for PostgreSQL
		db.SetMaxOpenConns(50)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(time.Hour)

		if err := db.Ping(); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
		}

		logrus.Info("Connected to PostgreSQL database for chat storage")
		return db, nil
	}

	// SQLite mode (legacy single-client)
	connStr := fmt.Sprintf("%s?_journal_mode=WAL", config.ChatStorageURI)
	if config.ChatStorageEnableForeignKeys {
		connStr += "&_foreign_keys=on"
	}

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// isPostgreSQLURI checks if the URI is a PostgreSQL connection string
func isPostgreSQLURI(uri string) bool {
	return strings.HasPrefix(uri, "postgres://") || strings.HasPrefix(uri, "postgresql://")
}

// isMultiClientMode returns true if multi-client mode is configured
func isMultiClientMode() bool {
	return len(config.WhatsAppClients) > 0 || config.DatabaseURL != ""
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

	// Check if multi-client mode is enabled
	if isMultiClientMode() {
		initMultiClientMode(ctx)
	} else {
		initSingleClientMode(ctx)
	}

	// Initialize usecases (shared between modes)
	// In multi-client mode, these will use the first client's chat storage for backward compatibility
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

// initSingleClientMode initializes the application in legacy single-client mode
func initSingleClientMode(ctx context.Context) {
	logrus.Info("Initializing in single-client mode")

	var err error
	chatStorageDB, err = initChatStorage()
	if err != nil {
		logrus.Fatalf("failed to initialize chat storage: %v", err)
	}

	chatStorageRepo = chatstorage.NewStorageRepository(chatStorageDB)
	if err := chatStorageRepo.InitializeSchema(); err != nil {
		logrus.Fatalf("failed to initialize chat storage schema: %v", err)
	}

	whatsappDB := whatsapp.InitWaDB(ctx, config.DBURI)
	var keysDB *sqlstore.Container
	if config.DBKeysURI != "" {
		keysDB = whatsapp.InitWaDB(ctx, config.DBKeysURI)
	}

	whatsappCli = whatsapp.InitWaCLI(ctx, whatsappDB, keysDB, chatStorageRepo)
}

// initMultiClientMode initializes the application in multi-client mode
func initMultiClientMode(ctx context.Context) {
	logrus.Info("========================================")
	logrus.Info("  MULTI-CLIENT MODE ENABLED")
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
	whatsappDB := whatsapp.InitWaDB(ctx, config.DBURI)
	var keysDB *sqlstore.Container
	if config.DBKeysURI != "" {
		keysDB = whatsapp.InitWaDB(ctx, config.DBKeysURI)
	}

	// Initialize the client registry
	whatsapp.InitRegistry(whatsappDB, keysDB)
	registry := whatsapp.GetRegistry()

	// Register each client
	for _, phone := range config.WhatsAppClients {
		phone = strings.TrimSpace(phone)
		if phone == "" {
			continue
		}

		// Create a device-specific chat storage repository
		var clientChatStorageRepo domainChatStorage.IChatStorageRepository
		if isPostgreSQLURI(config.ChatStorageURI) {
			clientChatStorageRepo = chatstorage.NewPostgresRepository(chatStorageDB, phone)
		} else {
			// In SQLite mode with multi-client, all clients share the same storage
			// (This is a limitation - PostgreSQL is recommended for multi-client)
			clientChatStorageRepo = chatstorage.NewStorageRepository(chatStorageDB)
		}

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

		// Set the first client's repo as the default for backward compatibility
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
		if isPostgreSQLURI(config.ChatStorageURI) {
			chatStorageRepo = chatstorage.NewPostgresRepository(chatStorageDB, "default")
		} else {
			chatStorageRepo = chatstorage.NewStorageRepository(chatStorageDB)
		}
		if err := chatStorageRepo.InitializeSchema(); err != nil {
			logrus.Fatalf("failed to initialize fallback chat storage schema: %v", err)
		}
	}

	logrus.Infof("Multi-client initialization complete. Registered %d clients", registry.GetClientCount())
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(embedIndex embed.FS, embedViews embed.FS) {
	EmbedIndex = embedIndex
	EmbedViews = embedViews
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
