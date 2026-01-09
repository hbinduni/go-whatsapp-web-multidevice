package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Global variables for WhatsApp client state
var (
	globalStateMu         sync.RWMutex
	cli                   *whatsmeow.Client
	db                    *sqlstore.Container
	keysDB                *sqlstore.Container
	log                   waLog.Logger
	startupTime           = time.Now().Unix()
	clientStopped         bool // True when client is intentionally stopped (not auto-reconnect)
	originalAutoReconnect bool // Stores original auto-reconnect setting
)

// InitWaDB initializes the WhatsApp database connection
func InitWaDB(ctx context.Context, DBURI string) *sqlstore.Container {
	log = waLog.Stdout("Main", config.WhatsappLogLevel, true)
	dbLog := waLog.Stdout("Database", config.WhatsappLogLevel, true)

	storeContainer, err := initDatabase(ctx, dbLog, DBURI)
	if err != nil {
		log.Errorf("Database initialization error: %v", err)
		panic(pkgError.InternalServerError(fmt.Sprintf("Database initialization error: %v", err)))
	}

	return storeContainer
}

// initDatabase creates and returns a database store container based on the configured URI
func initDatabase(ctx context.Context, dbLog waLog.Logger, DBURI string) (*sqlstore.Container, error) {
	if strings.HasPrefix(DBURI, "file:") {
		return sqlstore.New(ctx, "sqlite3", DBURI, dbLog)
	} else if strings.HasPrefix(DBURI, "postgres:") {
		return sqlstore.New(ctx, "postgres", DBURI, dbLog)
	}

	return nil, fmt.Errorf("unknown database type: %s. Currently only sqlite3(file:) and postgres are supported", DBURI)
}

func syncKeysDevice(ctx context.Context, db, keysDB *sqlstore.Container) {
	if keysDB == nil {
		return
	}

	dev, err := db.GetFirstDevice(ctx)
	if err != nil {
		log.Errorf("Failed to get first device: %v", err)
		return
	}

	if dev == nil || dev.ID == nil {
		log.Warnf("No device found in primary DB, skipping keysDB sync")
		return
	}

	devs, err := keysDB.GetAllDevices(ctx)
	if err != nil {
		log.Errorf("Failed to get all devices from keysDB: %v", err)
		return
	}

	found := false
	for _, d := range devs {
		if d.ID != nil && dev.ID != nil && *d.ID == *dev.ID {
			found = true
		} else if d != nil {
			// Delete old devices from keysDB with error handling
			if err := keysDB.DeleteDevice(ctx, d); err != nil {
				log.Warnf("Failed to delete old device %v from keysDB (will retry later): %v", d.ID, err)
			}
		}
	}

	if !found && dev.ID != nil {
		if err := keysDB.PutDevice(ctx, dev); err != nil {
			log.Errorf("Failed to put device in keysDB: %v", err)
		}
	}
}

// InitWaCLI initializes the WhatsApp client
func InitWaCLI(ctx context.Context, storeContainer, keysStoreContainer *sqlstore.Container, chatStorageRepo domainChatStorage.IChatStorageRepository) *whatsmeow.Client {
	device, err := storeContainer.GetFirstDevice(ctx)
	if err != nil {
		log.Errorf("Failed to get device: %v", err)
		panic(err)
	}

	if device == nil {
		log.Errorf("No device found")
		panic("No device found")
	}

	// Configure device properties
	osName := fmt.Sprintf("%s %s", config.AppOs, config.AppVersion)
	store.DeviceProps.PlatformType = &config.AppPlatform
	store.DeviceProps.Os = &osName

	// Keep references for global state update after client creation
	primaryDB := storeContainer
	keysContainer := keysStoreContainer

	// Configure a separated database for accelerating encryption caching
	if keysContainer != nil && device.ID != nil {
		innerStore := sqlstore.NewSQLStore(keysStoreContainer, *device.ID)

		syncKeysDevice(ctx, primaryDB, keysContainer)
		device.Identities = innerStore
		device.Sessions = innerStore
		device.PreKeys = innerStore
		device.SenderKeys = innerStore
		device.MsgSecrets = innerStore
		device.PrivacyTokens = innerStore
	}

	// Create and configure the client with filtered logging
	baseLogger := waLog.Stdout("Client", config.WhatsappLogLevel, true)
	client := whatsmeow.NewClient(device, newFilteredLogger(baseLogger))
	client.EnableAutoReconnect = true
	client.AutoTrustIdentity = true

	client.AddEventHandler(func(rawEvt interface{}) {
		handler(ctx, rawEvt, chatStorageRepo)
	})

	globalStateMu.Lock()
	cli = client
	db = primaryDB
	keysDB = keysContainer
	globalStateMu.Unlock()

	return client
}

// UpdateGlobalClient updates the global cli variable with a new client instance
func UpdateGlobalClient(newCli *whatsmeow.Client, newDB *sqlstore.Container) {
	globalStateMu.Lock()
	cli = newCli
	db = newDB
	globalStateMu.Unlock()
	log.Debugf("Global WhatsApp client updated")
}

// GetClient returns the current global client instance
func GetClient() *whatsmeow.Client {
	globalStateMu.RLock()
	defer globalStateMu.RUnlock()
	return cli
}

// GetDB returns the current global database instance
func GetDB() *sqlstore.Container {
	globalStateMu.RLock()
	defer globalStateMu.RUnlock()
	return db
}

func getStoreContainers() (*sqlstore.Container, *sqlstore.Container) {
	globalStateMu.RLock()
	defer globalStateMu.RUnlock()
	return db, keysDB
}

// GetConnectionStatus returns the current connection status of the global client
func GetConnectionStatus() (isConnected bool, isLoggedIn bool, deviceID string) {
	globalStateMu.RLock()
	currentClient := cli
	globalStateMu.RUnlock()
	if currentClient == nil {
		return false, false, ""
	}

	isConnected = currentClient.IsConnected()
	isLoggedIn = currentClient.IsLoggedIn()

	if currentClient.Store != nil && currentClient.Store.ID != nil {
		deviceID = currentClient.Store.ID.String()
	}

	return isConnected, isLoggedIn, deviceID
}

// IsClientStopped returns true if the client was intentionally stopped
func IsClientStopped() bool {
	globalStateMu.RLock()
	defer globalStateMu.RUnlock()
	return clientStopped
}

// StopClient disconnects the WhatsApp client without logging out
// The session remains valid and can be reconnected with StartClient
func StopClient() error {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()

	if cli == nil {
		return fmt.Errorf("client not initialized")
	}

	if clientStopped {
		return fmt.Errorf("client is already stopped")
	}

	// Save original auto-reconnect setting and disable it
	originalAutoReconnect = cli.EnableAutoReconnect
	cli.EnableAutoReconnect = false

	// Disconnect from WhatsApp (session remains valid)
	cli.Disconnect()

	clientStopped = true
	log.Infof("WhatsApp client stopped (disconnected without logout)")

	return nil
}

// StartClient reconnects the WhatsApp client after being stopped
func StartClient() error {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()

	if cli == nil {
		return fmt.Errorf("client not initialized")
	}

	if !clientStopped {
		// If not stopped, just ensure we're connected
		if cli.IsConnected() {
			return nil // Already connected
		}
	}

	// Restore auto-reconnect setting
	cli.EnableAutoReconnect = originalAutoReconnect

	// Reconnect to WhatsApp
	err := cli.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	clientStopped = false
	log.Infof("WhatsApp client started (reconnected)")

	return nil
}

// GetFullClientStatus returns comprehensive client status including stopped state
func GetFullClientStatus() (isConnected bool, isLoggedIn bool, isStopped bool, deviceID string) {
	globalStateMu.RLock()
	defer globalStateMu.RUnlock()

	isStopped = clientStopped

	if cli == nil {
		return false, false, isStopped, ""
	}

	isConnected = cli.IsConnected()
	isLoggedIn = cli.IsLoggedIn()

	if cli.Store != nil && cli.Store.ID != nil {
		deviceID = cli.Store.ID.String()
	}

	return isConnected, isLoggedIn, isStopped, deviceID
}
