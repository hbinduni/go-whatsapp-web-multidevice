package whatsapp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// ConnectionStatus represents the connection state of a WhatsApp client
type ConnectionStatus string

const (
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusLoggedIn     ConnectionStatus = "logged_in"
	StatusLoggedOut    ConnectionStatus = "logged_out"
)

// ManagedClient wraps a WhatsApp client with its associated resources
type ManagedClient struct {
	Phone           string
	Client          *whatsmeow.Client
	DB              *sqlstore.Container
	KeysDB          *sqlstore.Container
	ChatStorageRepo domainChatStorage.IChatStorageRepository
	Status          ConnectionStatus
	LastActivity    time.Time
	DeviceID        string
	mu              sync.RWMutex
}

// GetStatus returns the current connection status thread-safely
func (mc *ManagedClient) GetStatus() ConnectionStatus {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.Status
}

// SetStatus updates the connection status thread-safely
func (mc *ManagedClient) SetStatus(status ConnectionStatus) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.Status = status
	mc.LastActivity = time.Now()
}

// IsConnected checks if the client is connected
func (mc *ManagedClient) IsConnected() bool {
	if mc.Client == nil {
		return false
	}
	return mc.Client.IsConnected()
}

// IsLoggedIn checks if the client is logged in
func (mc *ManagedClient) IsLoggedIn() bool {
	if mc.Client == nil {
		return false
	}
	return mc.Client.IsLoggedIn()
}

// GetDeviceID returns the device ID if available
func (mc *ManagedClient) GetDeviceID() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	if mc.Client != nil && mc.Client.Store != nil && mc.Client.Store.ID != nil {
		return mc.Client.Store.ID.String()
	}
	return mc.DeviceID
}

// UpdateActivity updates the last activity timestamp
func (mc *ManagedClient) UpdateActivity() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.LastActivity = time.Now()
}

// ClientRegistry manages multiple WhatsApp clients
type ClientRegistry struct {
	mu      sync.RWMutex
	clients map[string]*ManagedClient // phone -> client
	db      *sqlstore.Container       // shared DB container for all clients
	keysDB  *sqlstore.Container       // shared keys DB container (optional)
	log     waLog.Logger
}

// Global registry instance
var (
	registryMu sync.RWMutex
	registry   *ClientRegistry
)

// NewClientRegistry creates a new client registry with shared database
func NewClientRegistry(db, keysDB *sqlstore.Container) *ClientRegistry {
	return &ClientRegistry{
		clients: make(map[string]*ManagedClient),
		db:      db,
		keysDB:  keysDB,
		log:     waLog.Stdout("Registry", config.WhatsappLogLevel, true),
	}
}

// InitRegistry initializes the global registry
func InitRegistry(db, keysDB *sqlstore.Container) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = NewClientRegistry(db, keysDB)
}

// GetRegistry returns the global registry instance
func GetRegistry() *ClientRegistry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry
}

// RegisterClient registers and initializes a new WhatsApp client for a phone number
func (r *ClientRegistry) RegisterClient(ctx context.Context, phone string, chatStorageRepo domainChatStorage.IChatStorageRepository) (*ManagedClient, error) {
	// Validate phone number format before acquiring lock
	normalizedPhone, err := ValidatePhone(phone)
	if err != nil {
		return nil, fmt.Errorf("invalid phone number: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if client already exists (try both original and normalized)
	if existing, ok := r.clients[phone]; ok {
		r.log.Infof("Client for %s already registered, returning existing", phone)
		return existing, nil
	}
	if phone != normalizedPhone {
		if existing, ok := r.clients[normalizedPhone]; ok {
			r.log.Infof("Client for %s already registered (normalized), returning existing", normalizedPhone)
			return existing, nil
		}
	}

	// Get or create device for this phone number
	device, err := r.getOrCreateDevice(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create device for %s: %w", phone, err)
	}

	// Configure device properties
	osName := fmt.Sprintf("%s %s", config.AppOs, config.AppVersion)
	store.DeviceProps.PlatformType = &config.AppPlatform
	store.DeviceProps.Os = &osName

	// Configure keys database if available
	if r.keysDB != nil && device.ID != nil {
		innerStore := sqlstore.NewSQLStore(r.keysDB, *device.ID)
		device.Identities = innerStore
		device.Sessions = innerStore
		device.PreKeys = innerStore
		device.SenderKeys = innerStore
		device.MsgSecrets = innerStore
		device.PrivacyTokens = innerStore
	}

	// Create the WhatsApp client
	baseLogger := waLog.Stdout(fmt.Sprintf("Client[%s]", phone), config.WhatsappLogLevel, true)
	client := whatsmeow.NewClient(device, newFilteredLogger(baseLogger))
	client.EnableAutoReconnect = true
	client.AutoTrustIdentity = true

	// Create managed client
	managedClient := &ManagedClient{
		Phone:           phone,
		Client:          client,
		DB:              r.db,
		KeysDB:          r.keysDB,
		ChatStorageRepo: chatStorageRepo,
		Status:          StatusDisconnected,
		LastActivity:    time.Now(),
	}

	// Store device ID if available
	if device.ID != nil {
		managedClient.DeviceID = device.ID.String()
	}

	// Add event handler for this client
	client.AddEventHandler(func(rawEvt interface{}) {
		handlerMultiClient(ctx, rawEvt, managedClient)
	})

	// Store in registry
	r.clients[phone] = managedClient
	r.log.Infof("Registered client for phone: %s", phone)

	return managedClient, nil
}

// getOrCreateDevice gets an existing device or creates a new one for the phone
func (r *ClientRegistry) getOrCreateDevice(ctx context.Context, phone string) (*store.Device, error) {
	// Try to get existing devices
	devices, err := r.db.GetAllDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	// Look for a device matching this phone number
	for _, device := range devices {
		if device.ID != nil {
			// Check if this device's JID matches the phone
			if device.ID.User == phone || normalizePhone(device.ID.User) == normalizePhone(phone) {
				r.log.Infof("Found existing device for phone %s: %s", phone, device.ID.String())
				return device, nil
			}
		}
	}

	// No existing device found, create a new one
	r.log.Infof("Creating new device for phone: %s", phone)
	device := r.db.NewDevice()
	return device, nil
}

// normalizePhone removes common prefixes and formats from phone numbers for comparison
func normalizePhone(phone string) string {
	// Remove common prefixes like + or leading zeros
	result := phone
	if len(result) > 0 && result[0] == '+' {
		result = result[1:]
	}
	// Remove any non-digit characters
	cleaned := ""
	for _, c := range result {
		if c >= '0' && c <= '9' {
			cleaned += string(c)
		}
	}
	return cleaned
}

// ValidatePhone validates that the phone string is a valid phone number format
// Returns the normalized phone number (digits only) and any validation error
func ValidatePhone(phone string) (string, error) {
	if phone == "" {
		return "", fmt.Errorf("phone number cannot be empty")
	}

	// Normalize to digits only
	normalized := normalizePhone(phone)

	// Check minimum length (shortest valid phone numbers are ~7 digits)
	if len(normalized) < 7 {
		return "", fmt.Errorf("phone number too short: %q (minimum 7 digits, got %d)", phone, len(normalized))
	}

	// Check maximum length (E.164 standard max is 15 digits)
	if len(normalized) > 15 {
		return "", fmt.Errorf("phone number too long: %q (maximum 15 digits, got %d)", phone, len(normalized))
	}

	// Verify it contains only digits after normalization
	for _, c := range normalized {
		if c < '0' || c > '9' {
			return "", fmt.Errorf("phone number contains invalid characters: %q", phone)
		}
	}

	return normalized, nil
}

// GetClient retrieves a client by phone number
func (r *ClientRegistry) GetClient(phone string) (*ManagedClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact match first
	if client, ok := r.clients[phone]; ok {
		return client, nil
	}

	// Try normalized match
	normalizedPhone := normalizePhone(phone)
	for key, client := range r.clients {
		if normalizePhone(key) == normalizedPhone {
			return client, nil
		}
	}

	return nil, fmt.Errorf("client not found for phone: %s", phone)
}

// GetAllClients returns all registered clients
func (r *ClientRegistry) GetAllClients() []*ManagedClient {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clients := make([]*ManagedClient, 0, len(r.clients))
	for _, client := range r.clients {
		clients = append(clients, client)
	}
	return clients
}

// GetClientCount returns the number of registered clients
func (r *ClientRegistry) GetClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

// UnregisterClient removes a client from the registry
func (r *ClientRegistry) UnregisterClient(phone string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	client, ok := r.clients[phone]
	if !ok {
		return fmt.Errorf("client not found for phone: %s", phone)
	}

	// Disconnect the client
	if client.Client != nil && client.Client.IsConnected() {
		client.Client.Disconnect()
	}

	delete(r.clients, phone)
	r.log.Infof("Unregistered client for phone: %s", phone)

	return nil
}

// ConnectClient connects a specific client
func (r *ClientRegistry) ConnectClient(phone string) error {
	client, err := r.GetClient(phone)
	if err != nil {
		return err
	}

	if client.Client.IsConnected() {
		return nil // Already connected
	}

	client.SetStatus(StatusConnecting)
	err = client.Client.Connect()
	if err != nil {
		client.SetStatus(StatusDisconnected)
		return fmt.Errorf("failed to connect client %s: %w", phone, err)
	}

	if client.Client.IsLoggedIn() {
		client.SetStatus(StatusLoggedIn)
	} else {
		client.SetStatus(StatusConnected)
	}

	return nil
}

// DisconnectClient disconnects a specific client
func (r *ClientRegistry) DisconnectClient(phone string) error {
	client, err := r.GetClient(phone)
	if err != nil {
		return err
	}

	if client.Client != nil && client.Client.IsConnected() {
		client.Client.Disconnect()
	}
	client.SetStatus(StatusDisconnected)

	return nil
}

// ReinitializeClient reinitializes a client after logout
// This creates a fresh whatsmeow client for the phone number
func (r *ClientRegistry) ReinitializeClient(ctx context.Context, phone string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mc, ok := r.clients[phone]
	if !ok {
		return fmt.Errorf("client not found: %s", phone)
	}

	// Disconnect old client if connected
	if mc.Client != nil && mc.Client.IsConnected() {
		mc.Client.Disconnect()
	}

	// Wait for disconnect to complete
	time.Sleep(500 * time.Millisecond)

	// Get or create a fresh device
	device, err := r.getOrCreateDevice(ctx, phone)
	if err != nil {
		return fmt.Errorf("failed to get/create device for %s: %w", phone, err)
	}

	// Configure device properties
	osName := fmt.Sprintf("%s %s", config.AppOs, config.AppVersion)
	store.DeviceProps.PlatformType = &config.AppPlatform
	store.DeviceProps.Os = &osName

	// Configure keys database if available
	if r.keysDB != nil && device.ID != nil {
		innerStore := sqlstore.NewSQLStore(r.keysDB, *device.ID)
		device.Identities = innerStore
		device.Sessions = innerStore
		device.PreKeys = innerStore
		device.SenderKeys = innerStore
		device.MsgSecrets = innerStore
		device.PrivacyTokens = innerStore
	}

	// Create new WhatsApp client
	baseLogger := waLog.Stdout(fmt.Sprintf("Client[%s]", phone), config.WhatsappLogLevel, true)
	newClient := whatsmeow.NewClient(device, newFilteredLogger(baseLogger))
	newClient.EnableAutoReconnect = true
	newClient.AutoTrustIdentity = true

	// Update managed client with new whatsmeow client
	mc.mu.Lock()
	mc.Client = newClient
	mc.Status = StatusDisconnected
	mc.LastActivity = time.Now()
	if device.ID != nil {
		mc.DeviceID = device.ID.String()
	} else {
		mc.DeviceID = ""
	}
	mc.mu.Unlock()

	// Add event handler for new client
	newClient.AddEventHandler(func(rawEvt interface{}) {
		handlerMultiClient(ctx, rawEvt, mc)
	})

	r.log.Infof("Reinitialized client for phone: %s", phone)

	return nil
}

// ConnectAll connects all registered clients
func (r *ClientRegistry) ConnectAll() []error {
	clients := r.GetAllClients()
	var errors []error

	for _, client := range clients {
		if err := r.ConnectClient(client.Phone); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// DisconnectAll disconnects all registered clients
func (r *ClientRegistry) DisconnectAll() {
	clients := r.GetAllClients()

	for _, client := range clients {
		_ = r.DisconnectClient(client.Phone)
	}
}

// GetClientStatus returns connection status for a specific client
func (r *ClientRegistry) GetClientStatus(phone string) (isConnected bool, isLoggedIn bool, deviceID string, err error) {
	client, err := r.GetClient(phone)
	if err != nil {
		return false, false, "", err
	}

	return client.IsConnected(), client.IsLoggedIn(), client.GetDeviceID(), nil
}

// GetAllClientStatuses returns status for all clients
func (r *ClientRegistry) GetAllClientStatuses() map[string]map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statuses := make(map[string]map[string]interface{})
	for phone, client := range r.clients {
		statuses[phone] = map[string]interface{}{
			"phone":         phone,
			"status":        string(client.GetStatus()),
			"is_connected":  client.IsConnected(),
			"is_logged_in":  client.IsLoggedIn(),
			"device_id":     client.GetDeviceID(),
			"last_activity": client.LastActivity,
		}
	}

	return statuses
}

// GetDB returns the shared database container
func (r *ClientRegistry) GetDB() *sqlstore.Container {
	return r.db
}

// CanAddClient checks if we can add more clients (below MAX_CLIENTS limit)
func (r *ClientRegistry) CanAddClient() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients) < config.MaxClients
}

// GetMaxClients returns the configured maximum clients limit
func (r *ClientRegistry) GetMaxClients() int {
	return config.MaxClients
}
