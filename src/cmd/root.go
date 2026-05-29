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

	// CORS settings
	if envCORSOrigins := viper.GetString("cors_allow_origins"); envCORSOrigins != "" {
		config.CORSAllowOrigins = envCORSOrigins
	}
	if envCORSHeaders := viper.GetString("cors_allow_headers"); envCORSHeaders != "" {
		config.CORSAllowHeaders = envCORSHeaders
	}
	if envCORSMethods := viper.GetString("cors_allow_methods"); envCORSMethods != "" {
		config.CORSAllowMethods = envCORSMethods
	}
	if viper.IsSet("cors_allow_credentials") {
		config.CORSAllowCredentials = viper.GetBool("cors_allow_credentials")
	}

	// Rate limiting settings
	if viper.IsSet("rate_limit_enabled") {
		config.RateLimitEnabled = viper.GetBool("rate_limit_enabled")
	}
	if viper.IsSet("rate_limit_max") {
		config.RateLimitMax = viper.GetInt("rate_limit_max")
	}
	if viper.IsSet("rate_limit_window_secs") {
		config.RateLimitWindowSecs = viper.GetInt("rate_limit_window_secs")
	}

	// Database settings
	if envDBURI := viper.GetString("db_uri"); envDBURI != "" {
		config.DBURI = envDBURI
	}
	if envDBKEYSURI := viper.GetString("db_keys_uri"); envDBKEYSURI != "" {
		config.DBKeysURI = envDBKEYSURI
	}

	// WhatsApp settings
	if envAutoReply := viper.GetString("whatsapp_auto_reply"); envAutoReply != "" {
		config.WhatsappAutoReplyMessage = envAutoReply
	}
	if viper.IsSet("whatsapp_auto_mark_read") {
		config.WhatsappAutoMarkRead = viper.GetBool("whatsapp_auto_mark_read")
	}
	if viper.IsSet("whatsapp_presence_available") {
		config.WhatsappPresenceAvailable = viper.GetBool("whatsapp_presence_available")
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

	// CORS flags
	rootCmd.PersistentFlags().StringVarP(
		&config.CORSAllowOrigins,
		"cors-origins", "",
		config.CORSAllowOrigins,
		`allowed CORS origins (comma-separated). Empty = same-origin only, "*" = all origins --cors-origins <string> | example: --cors-origins="https://example.com,https://app.example.com"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.CORSAllowHeaders,
		"cors-headers", "",
		config.CORSAllowHeaders,
		`allowed CORS headers --cors-headers <string> | example: --cors-headers="Origin, Content-Type, Accept, Authorization"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.CORSAllowMethods,
		"cors-methods", "",
		config.CORSAllowMethods,
		`allowed CORS methods --cors-methods <string> | example: --cors-methods="GET, POST, PUT, DELETE, OPTIONS"`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&config.CORSAllowCredentials,
		"cors-credentials", "",
		config.CORSAllowCredentials,
		`allow credentials in CORS requests --cors-credentials <true/false> | example: --cors-credentials=true`,
	)

	// Rate limiting flags
	rootCmd.PersistentFlags().BoolVarP(
		&config.RateLimitEnabled,
		"rate-limit", "",
		config.RateLimitEnabled,
		`enable rate limiting --rate-limit <true/false> | example: --rate-limit=true`,
	)
	rootCmd.PersistentFlags().IntVarP(
		&config.RateLimitMax,
		"rate-limit-max", "",
		config.RateLimitMax,
		`maximum requests per time window --rate-limit-max <number> | example: --rate-limit-max=100`,
	)
	rootCmd.PersistentFlags().IntVarP(
		&config.RateLimitWindowSecs,
		"rate-limit-window", "",
		config.RateLimitWindowSecs,
		`rate limit time window in seconds --rate-limit-window <number> | example: --rate-limit-window=60`,
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
		&config.WhatsappPresenceAvailable,
		"presence-available", "",
		config.WhatsappPresenceAvailable,
		`mark device as available/online on connect; true suppresses linked-phone notifications --presence-available <true/false> | example: --presence-available=true`,
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

	//preparing folder if not exist
	err := utils.CreateFolder(config.PathQrCode, config.PathSendItems, config.PathStorages)
	if err != nil {
		logrus.Errorln(err)
	}

	ctx := context.Background()

	chatStorageDB, err = initChatStorage()
	if err != nil {
		// Terminate the application if chat storage fails to initialize to avoid nil pointer panics later.
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

	// Usecase
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

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(embedIndex embed.FS, embedViews embed.FS) {
	EmbedIndex = embedIndex
	EmbedViews = embedViews
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
