package cmd

import (
	"fmt"
	"net/http"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/sse"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/websocket"
	"github.com/dustin/go-humanize"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var restCmd = &cobra.Command{
	Use:   "rest",
	Short: "Start REST API server for WhatsApp Web",
	Long:  `GoWA-SSE REST API server - WhatsApp Web API with SSE support, S3 storage, and chat history.`,
	Run:   restServer,
}

func init() {
	rootCmd.AddCommand(restCmd)
}

func restServer(_ *cobra.Command, _ []string) {
	logrus.Infof("🚀 Starting REST API mode (%s)...", config.AppVersion)

	// Validate auth configuration
	if config.AuthSecret == "" {
		logrus.Warn("⚠️  AUTH_SECRET not set - authentication disabled. Generate with: openssl rand -base64 32")
	}
	if config.AuthPasswordHash == "" && config.AuthSecret != "" {
		logrus.Warn("⚠️  AUTH_PASSWORD_HASH not set - authentication disabled. Generate with: ./whatsapp hash-password")
	}

	engine := html.NewFileSystem(http.FS(EmbedIndex), ".html")
	engine.AddFunc("isAuthenticated", func(username any) bool {
		return username != nil && username != ""
	})

	fiberConfig := fiber.Config{
		Views:                   engine,
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
	app.Use(config.AppBasePath+"/components", filesystem.New(filesystem.Config{
		Root:       http.FS(EmbedViews),
		PathPrefix: "views/components",
		Browse:     true,
	}))
	app.Use(config.AppBasePath+"/assets", filesystem.New(filesystem.Config{
		Root:       http.FS(EmbedViews),
		PathPrefix: "views/assets",
		Browse:     true,
	}))

	// Global middleware
	app.Use(middleware.Recovery())
	if config.AppDebug {
		app.Use(logger.New())
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
	}))

	// Create base path group
	var baseGroup fiber.Router = app
	if config.AppBasePath != "" {
		baseGroup = app.Group(config.AppBasePath)
	}

	// Public routes (no auth required)
	// Login page
	baseGroup.Get("/login", func(c *fiber.Ctx) error {
		// If already authenticated, redirect to dashboard
		if c.Cookies("access_token") != "" {
			return c.Redirect(config.AppBasePath + "/")
		}
		return c.Render("views/login", fiber.Map{
			"AppVersion":  config.AppVersion,
			"AppBasePath": config.AppBasePath,
		})
	})

	// Auth endpoints (public)
	rest.InitRestAuth(baseGroup)

	// Protected routes (JWT auth required)
	protectedGroup := baseGroup.Group("", middleware.JWTAuth())

	// Dashboard (main page)
	protectedGroup.Get("/", func(c *fiber.Ctx) error {
		return c.Render("views/index", fiber.Map{
			"AppHost":     fmt.Sprintf("%s://%s", c.Protocol(), c.Hostname()),
			"AppVersion":  config.AppVersion,
			"AppBasePath": config.AppBasePath,
			"Username":    middleware.GetAuthUsername(c),
			"MaxFileSize": humanize.Bytes(uint64(config.WhatsappSettingMaxFileSize)),
			"MaxVideoSize": humanize.Bytes(uint64(config.WhatsappSettingMaxVideoSize)),
			"Clients":     config.WhatsAppClients,
		})
	})

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

	logrus.Infof("🌐 Server listening on port %s", config.AppPort)
	if err := app.Listen(":" + config.AppPort); err != nil {
		logrus.Fatalln("Failed to start: ", err.Error())
	}
}
