package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
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

func (service *serviceApp) Login(ctx context.Context) (response domainApp.LoginResponse, err error) {
	client := whatsapp.GetClientFromContext(ctx)
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

	// Disconnect for reconnecting
	client.Disconnect()

	chImage := make(chan string)

	ch, err := client.GetQRChannel(ctx)
	if err != nil {
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
				response.Code = evt.Code
				response.Duration = evt.Timeout / time.Second / 2
				if evt.Event == "code" {
					qrPath := fmt.Sprintf("%s/scan-qr-%s.png", config.PathQrCode, fiberUtils.UUIDv4())
					err = qrcode.WriteFile(evt.Code, qrcode.Medium, 512, qrPath)
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

	client := whatsapp.GetClientFromContext(ctx)
	// detect is already logged in
	if client.Store.ID != nil || client.IsLoggedIn() {
		return loginCode, pkgError.ErrAlreadyLoggedIn
	}

	// reconnect first
	if err = service.Reconnect(ctx); err != nil {
		return loginCode, err
	}

	// refresh client reference after reconnect
	client = whatsapp.GetClientFromContext(ctx)
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

	client := whatsapp.GetClientFromContext(ctx)
	if client == nil {
		return pkgError.ErrWaCLI
	}

	// Get the phone number from context for registry operations
	phone := whatsapp.GetDeviceIDFromContext(ctx)

	// Call WhatsApp client logout first to disconnect from server
	err = client.Logout(ctx)
	if err != nil {
		logrus.Debugf("WhatsApp logout failed: %v", err)
		// Continue with cleanup even if logout fails - the client may already be disconnected
	}

	// Truncate chat storage for this client
	if service.chatStorageRepo != nil {
		if truncErr := service.chatStorageRepo.TruncateAllDataWithLogging("MANUAL_LOGOUT"); truncErr != nil {
			logrus.Errorf("Failed to truncate chat storage: %v", truncErr)
		}
	}

	// Reinitialize the client in the registry with a fresh state
	registry := whatsapp.GetRegistry()
	if registry != nil && phone != "" {
		if reinitErr := registry.ReinitializeClient(ctx, phone); reinitErr != nil {
			logrus.Errorf("Failed to reinitialize client: %v", reinitErr)
			return reinitErr
		}

		// Reconnect the reinitialized client
		if connectErr := registry.ConnectClient(phone); connectErr != nil {
			logrus.Warnf("Failed to reconnect client after logout: %v", connectErr)
		}
	}

	logrus.Debug("Logout process completed successfully")
	return nil
}

func (service *serviceApp) Reconnect(ctx context.Context) (err error) {
	logrus.Debug("Starting reconnect process...")

	client := whatsapp.GetClientFromContext(ctx)
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
	if whatsapp.GetClientFromContext(ctx) == nil {
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
	if whatsapp.GetClientFromContext(ctx) == nil {
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
