package usecase

import (
	"context"
	"errors"
	"time"

	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
)

type serviceApp struct {
	chatStorageRepo domainChatStorage.IChatStorageRepository
}

func NewAppService(chatStorageRepo domainChatStorage.IChatStorageRepository) domainApp.IAppUsecase {
	return &serviceApp{
		chatStorageRepo: chatStorageRepo,
	}
}

func (service *serviceApp) Login(ctx context.Context) (domainApp.LoginResponse, error) {
	return pairings.login(ctx, func() pairingClient {
		client := whatsapp.GetClient()
		if client == nil {
			return nil
		}
		return whatsappPairingClient{client}
	})
}

func (service *serviceApp) LoginWithCode(ctx context.Context, phoneNumber string) (loginCode string, err error) {
	pairings.mu.Lock()
	defer pairings.mu.Unlock()
	if err = validations.ValidateLoginWithCode(ctx, phoneNumber); err != nil {
		return loginCode, err
	}

	client := whatsapp.GetClient()
	if client == nil {
		return "", pkgError.ErrWaCLI
	}
	// detect is already logged in
	if client.Store.ID != nil || client.IsLoggedIn() {
		return loginCode, pkgError.ErrAlreadyLoggedIn
	}

	// Tear down any in-flight QR pairing first — otherwise its session competes
	// with PairPhone and the code handshake never completes (companion_finish is
	// never processed, so the device is never registered).
	pairings.stopLocked()

	// reconnect first
	if err = service.reconnect(); err != nil {
		return loginCode, err
	}

	// refresh client reference after reconnect
	client = whatsapp.GetClient()
	if client.IsLoggedIn() || client.Store.ID != nil {
		return loginCode, pkgError.ErrAlreadyLoggedIn
	}

	loginCode, err = client.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		logrus.Errorf("Error pairing phone: %v", err)
		return loginCode, err
	}

	logrus.Debugf("Phone pairing completed - IsConnected: %v, IsLoggedIn: %v",
		client.IsConnected(), client.IsLoggedIn())

	// Ensure global client is synchronized with service client
	whatsapp.UpdateGlobalClient(client, whatsapp.GetDB())

	logrus.Info("Phone pairing code generated; waiting for phone confirmation")
	return loginCode, nil
}

func (service *serviceApp) Logout(ctx context.Context) (err error) {
	pairings.mu.Lock()
	defer pairings.mu.Unlock()
	logrus.Debug("Starting logout process...")

	// Cancel any in-flight QR pairing session.
	pairings.stopLocked()

	client := whatsapp.GetClient()
	if client == nil {
		return pkgError.ErrWaCLI
	}

	db := whatsapp.GetDB()
	if db != nil {
		devices, dbErr := db.GetAllDevices(ctx)
		if dbErr != nil {
			logrus.Debugf("Error getting devices before logout: %v", dbErr)
		} else {
			logrus.Debugf("Devices before logout: %d found", len(devices))
		}
	}

	// Call WhatsApp client logout first to disconnect from server
	err = client.Logout(ctx)
	if err != nil {
		logrus.Debugf("WhatsApp logout failed: %v", err)
		// Continue with cleanup even if logout fails - the client may already be disconnected
	}

	// Wait for logout to fully propagate before checking device state
	time.Sleep(500 * time.Millisecond)

	// Perform complete cleanup with global client synchronization
	newDB, newCli, err := whatsapp.PerformCleanupAndUpdateGlobals(ctx, "MANUAL_LOGOUT", service.chatStorageRepo)
	if err != nil {
		logrus.Errorf("Cleanup failed: %v", err)
		return err
	}

	// Update service references
	whatsapp.UpdateGlobalClient(newCli, newDB)

	logrus.Debug("Logout process completed successfully")
	return nil
}

func (service *serviceApp) Reconnect(_ context.Context) error {
	pairings.mu.Lock()
	defer pairings.mu.Unlock()
	pairings.stopLocked()
	return service.reconnect()
}

// ReconnectIfPaired is shared by startup and health checks. Resolve and recheck
// the client under the same lock used by pairing, logout and explicit stop.
func (service *serviceApp) ReconnectIfPaired(_ context.Context) error {
	pairings.mu.Lock()
	defer pairings.mu.Unlock()
	client := whatsapp.GetClient()
	if client == nil || client.Store.ID == nil || whatsapp.IsClientStopped() || client.IsConnected() {
		return nil
	}
	err := client.Connect()
	if errors.Is(err, whatsmeow.ErrAlreadyConnected) {
		return nil
	}
	return err
}

func (service *serviceApp) reconnect() (err error) {
	logrus.Debug("Starting reconnect process...")

	client := whatsapp.GetClient()
	if client == nil {
		return pkgError.ErrWaCLI
	}
	client.Disconnect()
	err = client.Connect()

	if err != nil {
		logrus.Debugf("Reconnect failed: %v", err)
		return err
	}

	logrus.Debugf("Reconnection completed - IsConnected: %v, IsLoggedIn: %v",
		client.IsConnected(), client.IsLoggedIn())

	// Ensure global client is synchronized with service client
	whatsapp.UpdateGlobalClient(client, whatsapp.GetDB())

	return err
}

func (service *serviceApp) FirstDevice(ctx context.Context) (response domainApp.DevicesResponse, err error) {
	if whatsapp.GetClient() == nil {
		return response, pkgError.ErrWaCLI
	}

	devices, err := whatsapp.GetDB().GetFirstDevice(ctx)
	if err != nil {
		return response, err
	}

	response.Device = devices.ID.String()
	if devices.PushName != "" {
		response.Name = devices.PushName
	} else {
		response.Name = devices.BusinessName
	}

	return response, nil
}

func (service *serviceApp) FetchDevices(ctx context.Context) (response []domainApp.DevicesResponse, err error) {
	if whatsapp.GetClient() == nil {
		return response, pkgError.ErrWaCLI
	}

	devices, err := whatsapp.GetDB().GetAllDevices(ctx)
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		var d domainApp.DevicesResponse
		d.Device = device.ID.String()
		if device.PushName != "" {
			d.Name = device.PushName
		} else {
			d.Name = device.BusinessName
		}

		response = append(response, d)
	}

	return response, nil
}

// StopClient disconnects from WhatsApp without logging out
func (service *serviceApp) StopClient(_ context.Context) (response domainApp.ClientStatusResponse, err error) {
	pairings.mu.Lock()
	defer pairings.mu.Unlock()
	logrus.Debug("Stopping WhatsApp client...")
	pairings.stopLocked()

	err = whatsapp.StopClient()
	if err != nil {
		return response, err
	}

	// Get updated status
	isConnected, isLoggedIn, isStopped, deviceID := whatsapp.GetFullClientStatus()
	response.IsConnected = isConnected
	response.IsLoggedIn = isLoggedIn
	response.IsStopped = isStopped
	response.DeviceJID = deviceID
	response.StatusMessage = "Client stopped successfully. Session remains valid."

	logrus.Info("WhatsApp client stopped successfully")
	return response, nil
}

// StartClient reconnects to WhatsApp after being stopped
func (service *serviceApp) StartClient(_ context.Context) (response domainApp.ClientStatusResponse, err error) {
	pairings.mu.Lock()
	defer pairings.mu.Unlock()
	logrus.Debug("Starting WhatsApp client...")
	pairings.stopLocked()

	err = whatsapp.StartClient()
	if err != nil {
		return response, err
	}

	// Get updated status
	isConnected, isLoggedIn, isStopped, deviceID := whatsapp.GetFullClientStatus()
	response.IsConnected = isConnected
	response.IsLoggedIn = isLoggedIn
	response.IsStopped = isStopped
	response.DeviceJID = deviceID
	response.StatusMessage = "Client started successfully."

	logrus.Info("WhatsApp client started successfully")
	return response, nil
}

// GetClientStatus returns the current client connection status
func (service *serviceApp) GetClientStatus(_ context.Context) (response domainApp.ClientStatusResponse, err error) {
	isConnected, isLoggedIn, isStopped, deviceID := whatsapp.GetFullClientStatus()

	response.IsConnected = isConnected
	response.IsLoggedIn = isLoggedIn
	response.IsStopped = isStopped
	response.DeviceJID = deviceID

	if isStopped {
		response.StatusMessage = "Client is stopped (disconnected but session valid)"
	} else if isConnected && isLoggedIn {
		response.StatusMessage = "Client is connected and logged in"
	} else if isConnected {
		response.StatusMessage = "Client is connected but not logged in"
	} else {
		response.StatusMessage = "Client is disconnected"
	}

	return response, nil
}
