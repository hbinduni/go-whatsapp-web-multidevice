package cmd

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/sse"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/websocket"
	"github.com/dustin/go-humanize"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
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

	engine := html.NewFileSystem(http.FS(EmbedIndex), ".html")
	engine.AddFunc("isEnableBasicAuth", func(token any) bool {
		return token != nil
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

	app.Use(middleware.Recovery())
	app.Use(middleware.BasicAuth())
	if config.AppDebug {
		app.Use(logger.New())
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	if len(config.AppBasicAuthCredential) > 0 {
		account := make(map[string]string)
		for _, basicAuth := range config.AppBasicAuthCredential {
			ba := strings.Split(basicAuth, ":")
			if len(ba) != 2 {
				logrus.Fatalln("Basic auth is not valid, please this following format <user>:<secret>")
			}
			account[ba[0]] = ba[1]
		}

		app.Use(basicauth.New(basicauth.Config{
			Users: account,
		}))
	}

	// Create base path group or use app directly
	var apiGroup fiber.Router = app
	if config.AppBasePath != "" {
		apiGroup = app.Group(config.AppBasePath)
	}

	// Register routes based on mode
	if isMultiClientMode() {
		// Multi-client mode: routes are prefixed with /api/:phone/
		// All routes go through MultiClientMiddleware which injects client into context
		logrus.Info("Registering multi-client routes with /api/:phone/ prefix")

		// Admin routes (not per-phone, manage all clients)
		rest.InitRestAdmin(apiGroup, adminUsecase)

		// Create per-phone route group with middleware
		phoneGroup := apiGroup.Group("/api/:phone", middleware.MultiClientMiddleware())

		// Register all per-phone routes
		rest.InitRestApp(phoneGroup, appUsecase)
		rest.InitRestChat(phoneGroup, chatUsecase)
		rest.InitRestSend(phoneGroup, sendUsecase)
		rest.InitRestUser(phoneGroup, userUsecase)
		rest.InitRestMessage(phoneGroup, messageUsecase)
		rest.InitRestGroup(phoneGroup, groupUsecase)
		rest.InitRestNewsletter(phoneGroup, newsletterUsecase)
		rest.InitRestHistory(phoneGroup, historyUsecase)
		rest.InitRestMedia(phoneGroup) // Media download endpoint for private S3 buckets
	} else {
		// Single-client mode (legacy): routes without phone prefix
		logrus.Info("Registering single-client routes (legacy mode)")
		rest.InitRestApp(apiGroup, appUsecase)
		rest.InitRestAdmin(apiGroup, adminUsecase)
		rest.InitRestChat(apiGroup, chatUsecase)
		rest.InitRestSend(apiGroup, sendUsecase)
		rest.InitRestUser(apiGroup, userUsecase)
		rest.InitRestMessage(apiGroup, messageUsecase)
		rest.InitRestGroup(apiGroup, groupUsecase)
		rest.InitRestNewsletter(apiGroup, newsletterUsecase)
		rest.InitRestHistory(apiGroup, historyUsecase)
		rest.InitRestMedia(apiGroup) // Media download endpoint for private S3 buckets
	}

	apiGroup.Get("/", func(c *fiber.Ctx) error {
		return c.Render("views/index", fiber.Map{
			"AppHost":        fmt.Sprintf("%s://%s", c.Protocol(), c.Hostname()),
			"AppVersion":     config.AppVersion,
			"AppBasePath":    config.AppBasePath,
			"BasicAuthToken": c.UserContext().Value(middleware.AuthorizationValue("BASIC_AUTH")),
			"MaxFileSize":    humanize.Bytes(uint64(config.WhatsappSettingMaxFileSize)),
			"MaxVideoSize":   humanize.Bytes(uint64(config.WhatsappSettingMaxVideoSize)),
		})
	})

	websocket.RegisterRoutes(apiGroup, appUsecase)
	go websocket.RunHub()

	// SSE (Server-Sent Events) for real-time updates
	sse.RegisterRoutes(apiGroup)
	go sse.GetHub().Run()
	logrus.Info("SSE hub started - endpoint: /events")

	// Set auto reconnect to whatsapp server after booting
	go helpers.SetAutoConnectAfterBooting(appUsecase)

	// Set auto reconnect checking with a guaranteed client instance
	startAutoReconnectCheckerIfClientAvailable()

	if err := app.Listen(":" + config.AppPort); err != nil {
		logrus.Fatalln("Failed to start: ", err.Error())
	}
}
