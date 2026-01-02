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
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Context keys for dependency injection
type contextKey string

const (
	// ContextKeyClient is the key for storing the whatsmeow.Client in context
	ContextKeyClient contextKey = "wa_client"
	// ContextKeyManagedClient is the key for storing the ManagedClient in context
	ContextKeyManagedClient contextKey = "wa_managed_client"
	// ContextKeyDeviceID is the key for storing the device/phone ID in context
	ContextKeyDeviceID contextKey = "wa_device_id"
	// ContextKeyChatStorage is the key for storing the chat storage repo in context
	ContextKeyChatStorage contextKey = "wa_chat_storage"
)

// Global variables for WhatsApp state
var (
	globalStateMu sync.RWMutex
	log           waLog.Logger
	startupTime   = time.Now().Unix()
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

// initDatabase creates and returns a database store container for PostgreSQL
func initDatabase(ctx context.Context, dbLog waLog.Logger, DBURI string) (*sqlstore.Container, error) {
	if strings.HasPrefix(DBURI, "postgres://") || strings.HasPrefix(DBURI, "postgresql://") {
		return sqlstore.New(ctx, "postgres", DBURI, dbLog)
	}

	return nil, fmt.Errorf("DATABASE_URL must be a PostgreSQL connection string (postgres:// or postgresql://), got: %s", DBURI)
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

// GetClientByPhone returns a client for a specific phone number
func GetClientByPhone(phone string) (*whatsmeow.Client, error) {
	reg := GetRegistry()
	if reg == nil {
		return nil, pkgError.ErrWaCLI
	}

	mc, err := reg.GetClient(phone)
	if err != nil {
		return nil, err
	}

	return mc.Client, nil
}

// GetManagedClientByPhone returns the full ManagedClient for a specific phone number
func GetManagedClientByPhone(phone string) (*ManagedClient, error) {
	reg := GetRegistry()
	if reg == nil {
		return nil, pkgError.ErrWaCLI
	}

	return reg.GetClient(phone)
}

// GetDB returns the shared database from registry
func GetDB() *sqlstore.Container {
	reg := GetRegistry()
	if reg != nil {
		return reg.GetDB()
	}
	return nil
}

// GetAllClientStatuses returns status for all registered clients
func GetAllClientStatuses() map[string]map[string]interface{} {
	reg := GetRegistry()
	if reg == nil {
		return nil
	}
	return reg.GetAllClientStatuses()
}

// ContextWithClient returns a new context with the WhatsApp client attached
func ContextWithClient(ctx context.Context, client *whatsmeow.Client) context.Context {
	return context.WithValue(ctx, ContextKeyClient, client)
}

// ContextWithManagedClient returns a new context with the ManagedClient attached
func ContextWithManagedClient(ctx context.Context, mc *ManagedClient) context.Context {
	ctx = context.WithValue(ctx, ContextKeyManagedClient, mc)
	ctx = context.WithValue(ctx, ContextKeyClient, mc.Client)
	ctx = context.WithValue(ctx, ContextKeyDeviceID, mc.Phone)
	if mc.ChatStorageRepo != nil {
		ctx = context.WithValue(ctx, ContextKeyChatStorage, mc.ChatStorageRepo)
	}
	return ctx
}

// GetClientFromContext extracts the WhatsApp client from context
// Returns nil if not found in context
func GetClientFromContext(ctx context.Context) *whatsmeow.Client {
	if client, ok := ctx.Value(ContextKeyClient).(*whatsmeow.Client); ok && client != nil {
		return client
	}
	return nil
}

// GetManagedClientFromContext extracts the ManagedClient from context
// Returns nil if not in multi-client mode or not found in context
func GetManagedClientFromContext(ctx context.Context) *ManagedClient {
	if mc, ok := ctx.Value(ContextKeyManagedClient).(*ManagedClient); ok {
		return mc
	}
	return nil
}

// GetDeviceIDFromContext extracts the device/phone ID from context
// Returns empty string if not found
func GetDeviceIDFromContext(ctx context.Context) string {
	if deviceID, ok := ctx.Value(ContextKeyDeviceID).(string); ok {
		return deviceID
	}
	return ""
}

// GetChatStorageFromContext extracts the chat storage repository from context
// Returns nil if not found
func GetChatStorageFromContext(ctx context.Context) domainChatStorage.IChatStorageRepository {
	if repo, ok := ctx.Value(ContextKeyChatStorage).(domainChatStorage.IChatStorageRepository); ok {
		return repo
	}
	return nil
}

// MustGetClientFromContext extracts the WhatsApp client from context
// Panics if not found - use only when client is required
func MustGetClientFromContext(ctx context.Context) *whatsmeow.Client {
	client := GetClientFromContext(ctx)
	if client == nil {
		panic("WhatsApp client not found in context")
	}
	return client
}
