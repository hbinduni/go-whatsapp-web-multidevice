package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.mau.fi/whatsmeow/proto/waHistorySync"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/sse"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/websocket"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// Type definitions
type ExtractedMedia struct {
	MediaPath string `json:"media_path"`
	MimeType  string `json:"mime_type"`
	Caption   string `json:"caption"`
	FileSize  int64  `json:"file_size"`
}

// Global variables
var (
	globalStateMu sync.RWMutex
	cli           *whatsmeow.Client
	db            *sqlstore.Container // Add global database reference for cleanup
	keysDB        *sqlstore.Container
	log           waLog.Logger
	historySyncID int32
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

// initDatabase creates and returns a database store container based on the configured URI
func initDatabase(ctx context.Context, dbLog waLog.Logger, DBURI string) (*sqlstore.Container, error) {
	if strings.HasPrefix(DBURI, "file:") {
		return sqlstore.New(ctx, "sqlite3", DBURI, dbLog)
	} else if strings.HasPrefix(DBURI, "postgres:") {
		return sqlstore.New(ctx, "postgres", DBURI, dbLog)
	}

	return nil, fmt.Errorf("unknown database type: %s. Currently only sqlite3(file:) and postgres are supported", DBURI)
}

// NormalizeJIDFromLID converts @lid JIDs to their corresponding @s.whatsapp.net JIDs
// Returns the original JID if it's not an @lid or if LID lookup fails
func NormalizeJIDFromLID(ctx context.Context, jid types.JID, client *whatsmeow.Client) types.JID {
	// Only process @lid JIDs
	if jid.Server != "lid" {
		return jid
	}

	// Safety check
	if client == nil || client.Store == nil || client.Store.LIDs == nil {
		log.Warnf("Cannot resolve LID %s: client not available", jid.String())
		return jid
	}

	// Attempt to get the phone number for this LID
	pn, err := client.Store.LIDs.GetPNForLID(ctx, jid)
	if err != nil {
		log.Debugf("Failed to resolve LID %s to phone number: %v", jid.String(), err)
		return jid
	}

	// If we got a valid phone number, use it
	if !pn.IsEmpty() {
		log.Debugf("Resolved LID %s to phone number %s", jid.String(), pn.String())
		return pn
	}

	// Fallback to original JID
	return jid
}

// NormalizeJIDString converts a JID string from @lid format to @s.whatsapp.net format
// Uses the global WhatsApp client for LID resolution
// Returns the original string if conversion fails or JID is not an @lid
func NormalizeJIDString(ctx context.Context, jidStr string) string {
	// Quick check - if not @lid, return as-is
	if !strings.HasSuffix(jidStr, "@lid") {
		return jidStr
	}

	// Parse the JID string
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		log.Debugf("Failed to parse JID string %s: %v", jidStr, err)
		return jidStr
	}

	// Get the global client
	client := GetClient()
	if client == nil {
		log.Debugf("Cannot normalize JID %s: no WhatsApp client available", jidStr)
		return jidStr
	}

	// Normalize and return
	normalizedJID := NormalizeJIDFromLID(ctx, jid, client)
	return normalizedJID.String()
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
			// This can fail with FOREIGN KEY constraint if there are pending PreKey operations
			if err := keysDB.DeleteDevice(ctx, d); err != nil {
				// Log but don't fail - the device will be cleaned up on next sync
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

	// Create and configure the client with filtered logging to avoid noisy reconnection EOF errors
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
// This is needed when reinitializing the client after logout to ensure all
// infrastructure code uses the new client instance
func UpdateGlobalClient(newCli *whatsmeow.Client, newDB *sqlstore.Container) {
	globalStateMu.Lock()
	cli = newCli
	db = newDB
	globalStateMu.Unlock()
	log.Debugf("Global WhatsApp client updated")
}

// GetClient returns the current global client instance (alias for GetGlobalClient)
func GetClient() *whatsmeow.Client {
	globalStateMu.RLock()
	defer globalStateMu.RUnlock()
	return cli
}

// Get DB instance
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

// CleanupDatabase removes the database file (SQLite) or deletes all devices (PostgreSQL) to prevent foreign key constraint issues
func CleanupDatabase() error {
	globalStateMu.RLock()
	currentDB := db
	currentKeysDB := keysDB
	globalStateMu.RUnlock()

	// Check if using PostgreSQL
	if strings.HasPrefix(config.DBURI, "postgres:") {
		if currentDB == nil {
			return nil
		}

		ctx := context.Background()

		// Get and delete all devices
		devices, err := currentDB.GetAllDevices(ctx)
		if err != nil {
			return fmt.Errorf("failed to get devices: %v", err)
		}

		for _, device := range devices {
			if err := currentDB.DeleteDevice(ctx, device); err != nil {
				return fmt.Errorf("failed to delete device %s: %v", device.ID, err)
			}
		}

		// Also clean up keysDB if it exists and is separate
		if currentKeysDB != nil && currentKeysDB != currentDB {
			keysDevices, err := currentKeysDB.GetAllDevices(ctx)
			if err != nil {
				return fmt.Errorf("failed to get devices from keysDB: %v", err)
			}

			for _, device := range keysDevices {
				if err := currentKeysDB.DeleteDevice(ctx, device); err != nil {
					return fmt.Errorf("failed to delete device %s from keysDB: %v", device.ID, err)
				}
			}
		}

		return nil
	}

	// SQLite: Close database connections before removing the file
	if db != nil {
		if err := db.Close(); err != nil {
			return fmt.Errorf("failed to close main database: %v", err)
		}
	}

	// Close keysDB if it exists and is separate from main db
	if keysDB != nil && keysDB != db {
		if err := keysDB.Close(); err != nil {
			return fmt.Errorf("failed to close keysDB: %v", err)
		}

		// Remove keysDB file if it's also SQLite
		if config.DBKeysURI != "" && strings.HasPrefix(config.DBKeysURI, "file:") {
			keysDBPath := strings.TrimPrefix(config.DBKeysURI, "file:")
			if strings.Contains(keysDBPath, "?") {
				keysDBPath = strings.Split(keysDBPath, "?")[0]
			}

			if err := os.Remove(keysDBPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove keysDB file: %v", err)
			}
		}
	}

	// Now remove the main database file and its WAL/SHM files
	dbPath := strings.TrimPrefix(config.DBURI, "file:")
	if strings.Contains(dbPath, "?") {
		dbPath = strings.Split(dbPath, "?")[0]
	}

	// Remove SQLite WAL and SHM files first (they can hold locks)
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	_ = os.Remove(walPath) // Ignore errors - file may not exist
	_ = os.Remove(shmPath) // Ignore errors - file may not exist

	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CleanupTemporaryFiles removes history files, QR images, and send items
func CleanupTemporaryFiles() error {
	// Clean up history files
	if files, err := filepath.Glob(fmt.Sprintf("./%s/history-*", config.PathStorages)); err == nil {
		for _, f := range files {
			_ = os.Remove(f)
		}
	}

	// Clean up QR images
	if qrImages, err := filepath.Glob(fmt.Sprintf("./%s/scan-*", config.PathQrCode)); err == nil {
		for _, f := range qrImages {
			_ = os.Remove(f)
		}
	}

	// Clean up send items
	if qrItems, err := filepath.Glob(fmt.Sprintf("./%s/*", config.PathSendItems)); err == nil {
		for _, f := range qrItems {
			if !strings.Contains(f, ".gitignore") {
				_ = os.Remove(f)
			}
		}
	}

	return nil
}

// ReinitializeWhatsAppComponents reinitializes database and client components
func ReinitializeWhatsAppComponents(ctx context.Context, chatStorageRepo domainChatStorage.IChatStorageRepository) (*sqlstore.Container, *whatsmeow.Client, error) {
	newDB := InitWaDB(ctx, config.DBURI)
	var newKeysDB *sqlstore.Container
	if config.DBKeysURI != "" {
		newKeysDB = InitWaDB(ctx, config.DBKeysURI)
	}
	newCli := InitWaCLI(ctx, newDB, newKeysDB, chatStorageRepo)

	return newDB, newCli, nil
}

// PerformCompleteCleanup performs all cleanup operations in the correct order
func PerformCompleteCleanup(ctx context.Context, logPrefix string, chatStorageRepo domainChatStorage.IChatStorageRepository) (*sqlstore.Container, *whatsmeow.Client, error) {
	logrus.Debugf("[%s] Starting cleanup...", logPrefix)

	// Disconnect current client if it exists
	if current := GetClient(); current != nil {
		current.Disconnect()
		// Wait for background goroutines to finish before cleanup
		time.Sleep(2 * time.Second)
	}

	// Truncate all chatstorage data before other cleanup
	if chatStorageRepo != nil {
		if err := chatStorageRepo.TruncateAllDataWithLogging(logPrefix); err != nil {
			logrus.Errorf("[%s] Failed to truncate chatstorage: %v", logPrefix, err)
		}
	}

	// Clean up database
	if err := CleanupDatabase(); err != nil {
		return nil, nil, fmt.Errorf("database cleanup failed: %v", err)
	}

	// Reinitialize components with retry logic for file lock issues
	var newDB *sqlstore.Container
	var newCli *whatsmeow.Client
	var err error

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		newDB, newCli, err = ReinitializeWhatsAppComponents(ctx, chatStorageRepo)
		if err == nil {
			break
		}
		logrus.Warnf("[%s] Reinitialization attempt %d/%d failed: %v", logPrefix, i+1, maxRetries, err)
		if i < maxRetries-1 {
			time.Sleep(500 * time.Millisecond) // Wait before retry
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reinitialization failed after %d attempts: %v", maxRetries, err)
	}

	// Clean up temporary files (non-critical)
	_ = CleanupTemporaryFiles()

	logrus.Debugf("[%s] Cleanup completed, ready for new login", logPrefix)

	return newDB, newCli, nil
}

// PerformCleanupAndUpdateGlobals is a convenience function that performs cleanup
// and ensures global client synchronization
func PerformCleanupAndUpdateGlobals(ctx context.Context, logPrefix string, chatStorageRepo domainChatStorage.IChatStorageRepository) (*sqlstore.Container, *whatsmeow.Client, error) {
	newDB, newCli, err := PerformCompleteCleanup(ctx, logPrefix, chatStorageRepo)
	if err != nil {
		return nil, nil, err
	}

	// Ensure global client is properly synchronized
	UpdateGlobalClient(newCli, newDB)

	return newDB, newCli, nil
}

// handleRemoteLogout performs cleanup when user logs out from their phone
func handleRemoteLogout(ctx context.Context, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	logrus.Warn("[REMOTE_LOGOUT] User logged out from phone")

	// Perform complete cleanup with global client synchronization
	_, _, err := PerformCleanupAndUpdateGlobals(ctx, "REMOTE_LOGOUT", chatStorageRepo)
	if err != nil {
		logrus.Errorf("[REMOTE_LOGOUT] Cleanup failed: %v", err)
	}
}

// handler is the main event handler for WhatsApp events
func handler(ctx context.Context, rawEvt any, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	switch evt := rawEvt.(type) {
	case *events.DeleteForMe:
		handleDeleteForMe(ctx, evt, chatStorageRepo)
	case *events.AppStateSyncComplete:
		handleAppStateSyncComplete(ctx, evt)
	case *events.PairSuccess:
		handlePairSuccess(ctx, evt)
	case *events.LoggedOut:
		handleLoggedOut(ctx, chatStorageRepo)
	case *events.Connected, *events.PushNameSetting:
		handleConnectionEvents(ctx)
	case *events.StreamReplaced:
		handleStreamReplaced(ctx)
	case *events.Message:
		handleMessage(ctx, evt, chatStorageRepo)
	case *events.Receipt:
		handleReceipt(ctx, evt, chatStorageRepo)
	case *events.Presence:
		handlePresence(ctx, evt)
	case *events.HistorySync:
		handleHistorySync(ctx, evt, chatStorageRepo)
	case *events.AppState:
		handleAppState(ctx, evt)
	case *events.GroupInfo:
		handleGroupInfo(ctx, evt)
	}
}

// Event handler functions

func handleDeleteForMe(ctx context.Context, evt *events.DeleteForMe, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	log.Debugf("DeleteForMe event: message %s for %s", evt.MessageID, evt.SenderJID.String())

	// Find the message to get its chat JID
	message, err := chatStorageRepo.GetMessageByID(evt.MessageID)
	if err != nil {
		log.Errorf("Failed to find message %s for deletion: %v", evt.MessageID, err)
		return
	}

	if message == nil {
		log.Warnf("Message %s not found in database, skipping deletion", evt.MessageID)
		return
	}

	// Delete the message from database
	if err := chatStorageRepo.DeleteMessage(evt.MessageID, message.ChatJID); err != nil {
		log.Errorf("Failed to delete message %s from database: %v", evt.MessageID, err)
	}

	// Send webhook notification for delete event
	if len(config.WhatsappWebhook) > 0 {
		go func() {
			if err := forwardDeleteToWebhook(ctx, evt, message); err != nil {
				log.Errorf("Failed to forward delete event to webhook: %v", err)
			}
		}()
	}
}

func handleAppStateSyncComplete(_ context.Context, evt *events.AppStateSyncComplete) {
	client := GetClient()
	if client == nil {
		return
	}
	if len(client.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
		if err := client.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
			log.Warnf("Failed to send available presence: %v", err)
		}
	}
}

func handlePairSuccess(ctx context.Context, evt *events.PairSuccess) {
	websocket.Broadcast <- websocket.BroadcastMessage{
		Code:    "LOGIN_SUCCESS",
		Message: fmt.Sprintf("Successfully pair with %s", evt.ID.String()),
	}

	// Broadcast via SSE
	sse.BroadcastMessage(sse.EventLoginSuccess, "LOGIN_SUCCESS",
		fmt.Sprintf("Successfully paired with %s", evt.ID.String()),
		map[string]any{
			"device_id": evt.ID.String(),
		})

	primaryDB, secondaryDB := getStoreContainers()
	syncKeysDevice(ctx, primaryDB, secondaryDB)
}

func handleLoggedOut(ctx context.Context, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	// Perform comprehensive cleanup
	handleRemoteLogout(ctx, chatStorageRepo)

	// Broadcast final notification that cleanup is complete and ready for new login
	websocket.Broadcast <- websocket.BroadcastMessage{
		Code:    "LOGOUT_COMPLETE",
		Message: "Remote logout cleanup completed - ready for new login",
		Result:  nil,
	}

	// Broadcast via SSE
	sse.BroadcastMessage(sse.EventLogoutComplete, "LOGOUT_COMPLETE",
		"Remote logout cleanup completed - ready for new login", nil)
}

func handleConnectionEvents(_ context.Context) {
	client := GetClient()
	if client == nil {
		return
	}
	if len(client.Store.PushName) == 0 {
		return
	}

	// Send presence available when connecting and when the pushname is changed.
	// This makes sure that outgoing messages always have the right pushname.
	if err := client.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
		log.Warnf("Failed to send available presence: %v", err)
	}
}

func handleStreamReplaced(_ context.Context) {
	os.Exit(0)
}

func handleMessage(ctx context.Context, evt *events.Message, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	// Log message metadata (debug level to reduce verbosity)
	log.Debugf("Received message %s from %s", evt.Info.ID, evt.Info.SourceString())

	if err := chatStorageRepo.CreateMessage(ctx, evt); err != nil {
		// Log storage errors to avoid silent failures that could lead to data loss
		log.Errorf("Failed to store incoming message %s: %v", evt.Info.ID, err)
	}

	// Normalize JIDs from @lid to @s.whatsapp.net to match database format
	client := GetClient()
	normalizedChatJID := NormalizeJIDFromLID(ctx, evt.Info.Chat, client)
	normalizedSenderJID := NormalizeJIDFromLID(ctx, evt.Info.Sender, client)

	messageContent := utils.ExtractMessageTextFromEvent(evt)
	mediaType, _, _, _, _, _, _ := utils.ExtractMediaInfo(evt.Message)

	// Download media BEFORE broadcasting SSE so we can include the media URL
	var mediaPath string
	if config.WhatsappAutoDownloadMedia && client != nil {
		if img := evt.Message.GetImageMessage(); img != nil {
			deviceID := client.Store.ID.User
			chatJID := normalizedChatJID.String()
			messageID := evt.Info.ID

			if extractedMedia, err := utils.ExtractMediaWithInfo(ctx, client, img, chatJID, messageID, deviceID); err != nil {
				log.Errorf("Failed to download image: %v", err)
			} else {
				mediaPath = extractedMedia.MediaPath
				log.Debugf("📸 Media downloaded for SSE broadcast: %s", mediaPath)
			}
		}
	}

	// Broadcast via SSE for real-time updates (now includes media_path)
	sse.BroadcastMessageReceived(
		evt.Info.ID,
		normalizedChatJID.String(),
		normalizedSenderJID.String(),
		messageContent,
		evt.Info.Timestamp,
		evt.Info.IsFromMe,
		mediaType,
		mediaPath,
	)

	// Auto-mark message as read if configured
	handleAutoMarkRead(ctx, evt)

	// Handle auto-reply if configured
	handleAutoReply(ctx, evt, chatStorageRepo)

	// Forward to webhook if configured
	handleWebhookForward(ctx, evt)
}

func handleAutoMarkRead(_ context.Context, evt *events.Message) {
	// Only mark read if auto-mark read is enabled and message is incoming
	if !config.WhatsappAutoMarkRead || evt.Info.IsFromMe {
		return
	}

	client := GetClient()
	if client == nil {
		return
	}

	// Mark the message as read
	messageIDs := []types.MessageID{evt.Info.ID}
	timestamp := time.Now()
	chat := evt.Info.Chat
	sender := evt.Info.Sender

	if err := client.MarkRead(context.Background(), messageIDs, timestamp, chat, sender); err != nil {
		log.Warnf("Failed to mark message %s as read: %v", evt.Info.ID, err)
	} else {
		log.Debugf("Marked message %s as read", evt.Info.ID)
	}
}

func handleAutoReply(ctx context.Context, evt *events.Message, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	if config.WhatsappAutoReplyMessage == "" {
		return
	}

	client := GetClient()
	if client == nil {
		return
	}

	// Skip groups, broadcasts, and self messages
	if utils.IsGroupJID(evt.Info.Chat.String()) || evt.Info.IsIncomingBroadcast() || evt.Info.IsFromMe {
		return
	}

	// Only reply to direct 1:1 chats (e.g., *@s.whatsapp.net)
	if evt.Info.Chat.Server != types.DefaultUserServer {
		return
	}

	// Extra safety: skip any broadcast/status contexts
	source := evt.Info.SourceString()
	if strings.Contains(source, "broadcast") ||
		strings.HasSuffix(evt.Info.Chat.String(), "@broadcast") ||
		strings.HasPrefix(evt.Info.Chat.String(), "status@") {
		return
	}

	// Require actual typed text (not captions or synthetic labels)
	hasText := false

	// Unwrap FutureProof wrappers to access the inner message content first
	innerMsg := evt.Message
	for i := 0; i < 3; i++ { // safeguard against excessively nested wrappers
		if vm := innerMsg.GetViewOnceMessage(); vm != nil && vm.GetMessage() != nil {
			innerMsg = vm.GetMessage()
			continue
		}
		if em := innerMsg.GetEphemeralMessage(); em != nil && em.GetMessage() != nil {
			innerMsg = em.GetMessage()
			continue
		}
		if vm2 := innerMsg.GetViewOnceMessageV2(); vm2 != nil && vm2.GetMessage() != nil {
			innerMsg = vm2.GetMessage()
			continue
		}
		if vm2e := innerMsg.GetViewOnceMessageV2Extension(); vm2e != nil && vm2e.GetMessage() != nil {
			innerMsg = vm2e.GetMessage()
			continue
		}
		break
	}

	// Check for genuine typed text on the unwrapped content
	if conv := innerMsg.GetConversation(); conv != "" {
		hasText = true
	} else if ext := innerMsg.GetExtendedTextMessage(); ext != nil && ext.GetText() != "" {
		hasText = true
	} else if protoMsg := innerMsg.GetProtocolMessage(); protoMsg != nil {
		if edited := protoMsg.GetEditedMessage(); edited != nil {
			if ext := edited.GetExtendedTextMessage(); ext != nil && ext.GetText() != "" {
				hasText = true
			} else if conv := edited.GetConversation(); conv != "" {
				hasText = true
			}
		}
	}
	if !hasText {
		return
	}

	// Format recipient JID
	recipientJID := utils.FormatJID(evt.Info.Sender.String())

	// Send the auto-reply message
	response, err := client.SendMessage(
		ctx,
		recipientJID,
		&waE2E.Message{Conversation: proto.String(config.WhatsappAutoReplyMessage)},
	)

	if err != nil {
		log.Errorf("Failed to send auto-reply message: %v", err)
		return
	}

	// Store the auto-reply message in chat storage if send was successful
	if chatStorageRepo != nil {
		// Get our own JID as sender
		senderJID := ""
		if client.Store.ID != nil {
			senderJID = client.Store.ID.String()
		}

		// Store the sent auto-reply message
		if err := chatStorageRepo.StoreSentMessageWithContext(
			ctx,
			response.ID,                     // Message ID from WhatsApp response
			senderJID,                       // Our JID as sender
			recipientJID.String(),           // Recipient JID
			config.WhatsappAutoReplyMessage, // Auto-reply content
			response.Timestamp,              // Timestamp from response
		); err != nil {
			// Log storage error but don't fail the auto-reply
			log.Errorf("Failed to store auto-reply message in chat storage: %v", err)
		} else {
			log.Debugf("Auto-reply message %s stored successfully in chat storage", response.ID)
		}
	}
}

func handleWebhookForward(ctx context.Context, evt *events.Message) {
	// Skip webhook for specific protocol messages that shouldn't trigger webhooks
	if protocolMessage := evt.Message.GetProtocolMessage(); protocolMessage != nil {
		protocolType := protocolMessage.GetType().String()
		// Skip EPHEMERAL_SYNC_RESPONSE but allow REVOKE and MESSAGE_EDIT
		if protocolType == "EPHEMERAL_SYNC_RESPONSE" {
			log.Debugf("Skipping webhook for EPHEMERAL_SYNC_RESPONSE message")
			return
		}
	}

	if len(config.WhatsappWebhook) > 0 &&
		!strings.Contains(evt.Info.SourceString(), "broadcast") {
		go func(evt *events.Message) {
			if err := forwardMessageToWebhook(ctx, evt); err != nil {
				logrus.Error("Failed forward to webhook: ", err)
			}
		}(evt)
	}
}

func handleReceipt(ctx context.Context, evt *events.Receipt, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	sendReceipt := false
	var status string

	switch evt.Type {
	case types.ReceiptTypeRead, types.ReceiptTypeReadSelf:
		sendReceipt = true
		status = "read"
	case types.ReceiptTypeDelivered:
		sendReceipt = true
		status = "delivered"
	case types.ReceiptTypePlayed:
		sendReceipt = true
		status = "played"
	}

	// Update message status in database for each message ID in the receipt
	if status != "" && chatStorageRepo != nil {
		for _, messageID := range evt.MessageIDs {
			if err := chatStorageRepo.UpdateMessageStatus(ctx, messageID, status, evt.Timestamp); err != nil {
				logrus.Errorf("Failed to update status for message %s to %s: %v", messageID, status, err)
			} else {
				logrus.Debugf("Updated message %s status to %s", messageID, status)
			}
		}

		// Broadcast via SSE for real-time status updates
		// Normalize chat JID from @lid to @s.whatsapp.net to match database format
		client := GetClient()
		normalizedChatJID := NormalizeJIDFromLID(ctx, evt.Chat, client)
		sse.BroadcastReceipt(evt.MessageIDs, normalizedChatJID.String(), status, evt.Timestamp)
	}

	// Forward receipt (ack) event to webhook if configured
	// Note: Receipt events are not rate limited as they are critical for message delivery status
	if len(config.WhatsappWebhook) > 0 && sendReceipt {
		go func(e *events.Receipt) {
			if err := forwardReceiptToWebhook(ctx, e); err != nil {
				logrus.Errorf("Failed to forward ack event to webhook: %v", err)
			}
		}(evt)
	}
}

func handlePresence(ctx context.Context, evt *events.Presence) {
	// Broadcast via SSE for real-time presence updates
	// Normalize JID from @lid to @s.whatsapp.net to match database format
	client := GetClient()
	normalizedJID := NormalizeJIDFromLID(ctx, evt.From, client)
	sse.BroadcastPresenceUpdate(normalizedJID.String(), !evt.Unavailable, evt.LastSeen)
}

func handleHistorySync(ctx context.Context, evt *events.HistorySync, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	// Check if history sync is enabled
	if !config.HistorySyncEnabled {
		log.Debugf("History sync is disabled, skipping sync type: %s", evt.Data.SyncType.String())
		return
	}

	client := GetClient()
	if client == nil || client.Store == nil || client.Store.ID == nil {
		log.Warnf("Skipping history sync handling: WhatsApp client not initialized")
		return
	}
	id := atomic.AddInt32(&historySyncID, 1)
	fileName := fmt.Sprintf("%s/history-%d-%s-%d-%s.json",
		config.PathStorages,
		startupTime,
		client.Store.ID.String(),
		id,
		evt.Data.SyncType.String(),
	)

	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Errorf("Failed to open file to write history sync: %v", err)
		return
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err = enc.Encode(evt.Data); err != nil {
		log.Errorf("Failed to write history sync: %v", err)
		return
	}

	log.Infof("Wrote history sync to %s (type: %s)", fileName, evt.Data.SyncType.String())

	// Process history sync data to database
	if chatStorageRepo != nil {
		if err := processHistorySync(ctx, evt.Data, chatStorageRepo); err != nil {
			log.Errorf("Failed to process history sync to database: %v", err)
		}
	}
}

func handleAppState(_ context.Context, evt *events.AppState) {
	log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
}

// processHistorySync processes history sync data and stores messages in the database
func processHistorySync(ctx context.Context, data *waHistorySync.HistorySync, chatStorageRepo domainChatStorage.IChatStorageRepository) error {
	if data == nil {
		return nil
	}

	syncType := data.GetSyncType()
	log.Debugf("Processing history sync type: %s", syncType.String())

	switch syncType {
	case waHistorySync.HistorySync_INITIAL_BOOTSTRAP, waHistorySync.HistorySync_RECENT:
		// Process conversation messages
		return processConversationMessages(ctx, data, chatStorageRepo)
	case waHistorySync.HistorySync_PUSH_NAME:
		// Process push names to update chat names
		return processPushNames(ctx, data, chatStorageRepo)
	default:
		// Other sync types are not needed for message storage
		log.Debugf("Skipping history sync type: %s", syncType.String())
		return nil
	}
}

// processConversationMessages processes and stores conversation messages from history sync
func processConversationMessages(ctx context.Context, data *waHistorySync.HistorySync, chatStorageRepo domainChatStorage.IChatStorageRepository) error {
	conversations := data.GetConversations()
	log.Infof("Processing %d conversations from history sync", len(conversations))

	// Calculate cutoff time based on configuration
	var cutoffTime time.Time
	if config.HistorySyncMaxDays > 0 {
		cutoffTime = time.Now().AddDate(0, 0, -int(config.HistorySyncMaxDays))
		log.Infof("History sync filtering: only messages after %s will be processed (max %d days)",
			cutoffTime.Format("2006-01-02"), config.HistorySyncMaxDays)
	} else if config.HistorySyncMaxDays == -1 {
		log.Infof("History sync filtering: processing all available messages (no time limit)")
	}

	client := GetClient()

	for _, conv := range conversations {
		rawChatJID := conv.GetID()
		if rawChatJID == "" {
			continue
		}

		// Parse JID to get proper format
		jid, err := types.ParseJID(rawChatJID)
		if err != nil {
			log.Warnf("Failed to parse JID %s: %v", rawChatJID, err)
			continue
		}

		// Normalize JID (convert @lid to @s.whatsapp.net if possible)
		jid = NormalizeJIDFromLID(ctx, jid, client)
		chatJID := jid.String()

		displayName := conv.GetDisplayName()

		// Get or create chat
		chatName := chatStorageRepo.GetChatNameWithPushName(jid, chatJID, "", displayName)

		// Extract ephemeral expiration from conversation
		ephemeralExpiration := conv.GetEphemeralExpiration()

		// Process messages in the conversation
		messages := conv.GetMessages()
		log.Debugf("Processing %d messages for chat %s", len(messages), chatJID)

		// Collect messages for batch processing
		var messageBatch []*domainChatStorage.Message
		var latestTimestamp time.Time

		for _, histMsg := range messages {
			if histMsg == nil || histMsg.Message == nil {
				continue
			}

			msg := histMsg.Message
			msgKey := msg.GetKey()
			if msgKey == nil {
				continue
			}

			// Skip messages without ID
			messageID := msgKey.GetID()
			if messageID == "" {
				continue
			}

			// Extract message content and media info
			content := utils.ExtractMessageTextFromProto(msg.GetMessage())
			mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := utils.ExtractMediaInfo(msg.GetMessage())

			// Skip if there's no content and no media
			if content == "" && mediaType == "" {
				continue
			}

			// Determine sender
			sender := ""
			isFromMe := msgKey.GetFromMe()
			if isFromMe {
				// For self-messages, use the full JID format to match regular message processing
				if client != nil && client.Store.ID != nil {
					sender = client.Store.ID.String() // Use full JID instead of just User part
				} else {
					// Skip messages where we can't determine the sender to avoid NOT NULL violations
					log.Warnf("Skipping self-message %s: client ID unavailable", messageID)
					continue
				}
			} else {
				participant := msgKey.GetParticipant()
				if participant != "" {
					// For group messages, participant contains the actual sender
					if senderJID, err := types.ParseJID(participant); err == nil {
						// Normalize sender JID (convert @lid to @s.whatsapp.net if possible)
						senderJID = NormalizeJIDFromLID(ctx, senderJID, client)
						sender = senderJID.String() // Use full JID format for consistency
					} else {
						// Fallback to participant string, but ensure it's not empty
						if participant != "" {
							sender = participant
						} else {
							log.Warnf("Skipping message %s: empty participant", messageID)
							continue
						}
					}
				} else {
					// For individual chats, use the chat JID as sender with full format
					sender = jid.String() // Use full JID format for consistency
				}
			}

			// Convert timestamp from Unix seconds to time.Time
			// WhatsApp history sync timestamps are in seconds, not milliseconds
			timestamp := time.Unix(int64(msg.GetMessageTimestamp()), 0)

			// Skip messages outside configured time range
			if config.HistorySyncMaxDays > 0 && timestamp.Before(cutoffTime) {
				continue
			}

			// Track latest timestamp
			if timestamp.After(latestTimestamp) {
				latestTimestamp = timestamp
			}

			// Create message object and add to batch
			message := &domainChatStorage.Message{
				ID:            messageID,
				ChatJID:       chatJID,
				Sender:        sender,
				Content:       content,
				Timestamp:     timestamp,
				IsFromMe:      isFromMe,
				MediaType:     mediaType,
				Filename:      filename,
				URL:           url,
				MediaKey:      mediaKey,
				FileSHA256:    fileSHA256,
				FileEncSHA256: fileEncSHA256,
				FileLength:    fileLength,
			}

			messageBatch = append(messageBatch, message)
		}

		// Store or update the chat with latest message time
		if len(messageBatch) > 0 {
			chat := &domainChatStorage.Chat{
				JID:                 chatJID,
				Name:                chatName,
				LastMessageTime:     latestTimestamp,
				EphemeralExpiration: ephemeralExpiration,
			}

			// Store or update the chat
			if err := chatStorageRepo.StoreChat(chat); err != nil {
				log.Warnf("Failed to store chat %s: %v", chatJID, err)
				continue
			}

			// Store messages in batch
			if err := chatStorageRepo.StoreMessagesBatch(messageBatch); err != nil {
				log.Warnf("Failed to store messages batch for chat %s: %v", chatJID, err)
			} else {
				log.Debugf("Stored %d messages for chat %s", len(messageBatch), chatJID)
			}
		}
	}

	return nil
}

// processPushNames processes push names from history sync to update chat names
func processPushNames(ctx context.Context, data *waHistorySync.HistorySync, chatStorageRepo domainChatStorage.IChatStorageRepository) error {
	pushnames := data.GetPushnames()
	log.Debugf("Processing %d push names from history sync", len(pushnames))

	client := GetClient()

	for _, pushname := range pushnames {
		rawJIDStr := pushname.GetID()
		name := pushname.GetPushname()

		if rawJIDStr == "" || name == "" {
			continue
		}

		// Parse and normalize JID (convert @lid to @s.whatsapp.net if possible)
		jid, err := types.ParseJID(rawJIDStr)
		if err != nil {
			log.Warnf("Failed to parse JID %s in push names: %v", rawJIDStr, err)
			continue
		}
		jid = NormalizeJIDFromLID(ctx, jid, client)
		jidStr := jid.String()

		// Check if chat exists
		existingChat, err := chatStorageRepo.GetChat(jidStr)
		if err != nil || existingChat == nil {
			// Chat doesn't exist yet, skip
			continue
		}

		// Update chat name if it's different
		if existingChat.Name != name {
			existingChat.Name = name
			if err := chatStorageRepo.StoreChat(existingChat); err != nil {
				log.Warnf("Failed to update chat name for %s: %v", jidStr, err)
			} else {
				log.Debugf("Updated chat name for %s to %s", jidStr, name)
			}
		}
	}

	return nil
}

func handleGroupInfo(ctx context.Context, evt *events.GroupInfo) {
	// Only process events that have actual changes
	hasChanges := len(evt.Join) > 0 || len(evt.Leave) > 0 || len(evt.Promote) > 0 || len(evt.Demote) > 0 ||
		evt.Name != nil || evt.Topic != nil || evt.Locked != nil || evt.Announce != nil

	if !hasChanges {
		return
	}

	// Log group events for debugging
	if len(evt.Join) > 0 {
		log.Debugf("Group %s: %d users joined at %s", evt.JID, len(evt.Join), evt.Timestamp)
	}
	if len(evt.Leave) > 0 {
		log.Debugf("Group %s: %d users left at %s", evt.JID, len(evt.Leave), evt.Timestamp)
	}
	if len(evt.Promote) > 0 {
		log.Debugf("Group %s: %d users promoted at %s", evt.JID, len(evt.Promote), evt.Timestamp)
	}
	if len(evt.Demote) > 0 {
		log.Debugf("Group %s: %d users demoted at %s", evt.JID, len(evt.Demote), evt.Timestamp)
	}

	// Forward group info event to webhook if configured
	if len(config.WhatsappWebhook) > 0 {
		go func(e *events.GroupInfo) {
			if err := forwardGroupInfoToWebhook(ctx, e); err != nil {
				logrus.Errorf("Failed to forward group info event to webhook: %v", err)
			}
		}(evt)
	}
}
