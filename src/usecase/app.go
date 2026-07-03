package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	fiberUtils "github.com/gofiber/fiber/v2/utils"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
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

// qrPairing tracks the context of an in-flight QR pairing session so it can be
// torn down explicitly. whatsmeow allows only one pairing method per client: if a
// QR pairing is left active, a subsequent code pairing (PairPhone) never completes
// its companion_finish handshake because the QR channel keeps consuming pairing
// events. Cancelling this context clears whatsmeow's QR pairing state and closes
// the channel, freeing the client for code pairing.
var (
	qrPairingMu     sync.Mutex
	qrPairingCancel context.CancelFunc
)

// stopQRPairing cancels any in-flight QR pairing session. Safe to call when none
// is active. Called before starting a new pairing (QR or code) and on logout.
func stopQRPairing() {
	qrPairingMu.Lock()
	defer qrPairingMu.Unlock()
	if qrPairingCancel != nil {
		qrPairingCancel()
		qrPairingCancel = nil
	}
}

func (service *serviceApp) Login(ctx context.Context) (response domainApp.LoginResponse, err error) {
	client := whatsapp.GetClient()
	if client == nil {
		return response, pkgError.ErrWaCLI
	}

	logrus.Debug("Starting login process...")
	devices, dbErr := whatsapp.GetDB().GetAllDevices(ctx)
	if dbErr != nil {
		logrus.Debugf("Error getting devices before login: %v", dbErr)
	} else {
		logrus.Debugf("Devices before login: %d found", len(devices))
	}

	// Cancel any in-flight QR pairing before starting a new one so two pairing
	// sessions never run on a single client.
	stopQRPairing()

	// Disconnect for reconnecting
	client.Disconnect()

	chImage := make(chan string)

	// Own the QR pairing lifecycle with a cancellable context (not the request
	// ctx) so it survives the HTTP response yet can be torn down explicitly when
	// the user switches to code pairing (see stopQRPairing).
	qrCtx, qrCancel := context.WithCancel(context.Background())
	qrPairingMu.Lock()
	qrPairingCancel = qrCancel
	qrPairingMu.Unlock()

	ch, err := client.GetQRChannel(qrCtx)
	if err != nil {
		stopQRPairing()
		logrus.Debugf("GetQRChannel failed: %v", err)
		// This error means that we're already logged in, so ignore it.
		if errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			_ = client.Connect() // just connect to websocket
			if client.IsLoggedIn() {
				return response, pkgError.ErrAlreadyLoggedIn
			}
			return response, pkgError.ErrSessionSaved
		} else {
			return response, pkgError.ErrQrChannel
		}
	} else {
		go func() {
			for evt := range ch {
				// whatsmeow wraps the pairing ref in a wa.me deep-link URL
				// (https://wa.me/settings/linked_devices#<ref>,...). WhatsApp's
				// in-app "Link a Device" scanner only recognizes the bare ref, so
				// strip the wrapper before rendering/returning the QR.
				code := strings.TrimPrefix(evt.Code, "https://wa.me/settings/linked_devices#")
				response.Code = code
				response.Duration = evt.Timeout / time.Second / 2
				if evt.Event == "code" {
					qrPath := fmt.Sprintf("%s/scan-qr-%s.png", config.PathQrCode, fiberUtils.UUIDv4())
					err = qrcode.WriteFile(code, qrcode.Medium, 512, qrPath)
					if err != nil {
						logrus.Errorf("Error writing QR code to file: %v", err)
					}
					go func() {
						time.Sleep(response.Duration * time.Second)
						if err := os.Remove(qrPath); err != nil && !os.IsNotExist(err) {
							logrus.Debugf("Error removing QR image file: %v", err)
						}
					}()
					chImage <- qrPath
				} else {
					logrus.Debugf("QR event: %s, error: %v", evt.Event, evt.Error)
				}
			}
		}()
	}

	err = client.Connect()
	if err != nil {
		logrus.Errorf("Error connecting to WhatsApp: %v", err)
		return response, pkgError.ErrReconnect
	}
	response.ImagePath = <-chImage

	logrus.Debugf("Login connection established - IsConnected: %v, IsLoggedIn: %v",
		client.IsConnected(), client.IsLoggedIn())

	// Ensure global client is synchronized with service client
	whatsapp.UpdateGlobalClient(client, whatsapp.GetDB())

	return response, nil
}

func (service *serviceApp) LoginWithCode(ctx context.Context, phoneNumber string) (loginCode string, err error) {
	if err = validations.ValidateLoginWithCode(ctx, phoneNumber); err != nil {
		return loginCode, err
	}

	client := whatsapp.GetClient()
	// detect is already logged in
	if client.Store.ID != nil || client.IsLoggedIn() {
		return loginCode, pkgError.ErrAlreadyLoggedIn
	}

	// Tear down any in-flight QR pairing first — otherwise its session competes
	// with PairPhone and the code handshake never completes (companion_finish is
	// never processed, so the device is never registered).
	stopQRPairing()

	// reconnect first
	if err = service.Reconnect(ctx); err != nil {
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

	logrus.Infof("Successfully paired phone with code: %s", loginCode)
	return loginCode, nil
}

func (service *serviceApp) Logout(ctx context.Context) (err error) {
	logrus.Debug("Starting logout process...")

	// Cancel any in-flight QR pairing session.
	stopQRPairing()

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

func (service *serviceApp) Reconnect(_ context.Context) (err error) {
	logrus.Debug("Starting reconnect process...")

	client := whatsapp.GetClient()
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
	logrus.Debug("Stopping WhatsApp client...")

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
	logrus.Debug("Starting WhatsApp client...")

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
