package helpers

import (
	"context"
	"io"
	"mime/multipart"
	"sync"
	"time"

	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/sirupsen/logrus"
	waLib "go.mau.fi/whatsmeow"
)

var (
	stopReconnect     = make(chan struct{})
	reconnectStopped  = make(chan struct{})
	reconnectOnce     sync.Once
	reconnectStopOnce sync.Once
)

func SetAutoConnectAfterBooting(service domainApp.IAppUsecase) {
	time.Sleep(2 * time.Second)
	_ = service.Reconnect(context.Background())
}

// SetAutoReconnectChecking starts a background goroutine that periodically checks
// if the WhatsApp client is connected, and reconnects if not.
// Call StopAutoReconnectChecking() to gracefully stop the loop.
// This is for single-client mode only.
func SetAutoReconnectChecking(cli *waLib.Client) {
	if cli == nil {
		logrus.Warn("[AutoReconnect] Called with nil WhatsApp client; skipping")
		return
	}

	reconnectOnce.Do(func() {
		logrus.Info("[AutoReconnect] Starting single-client reconnect checker (every 1 minute)")
		go func() {
			defer close(reconnectStopped)
			ticker := time.NewTicker(1 * time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if !cli.IsConnected() {
						logrus.Info("[AutoReconnect] Client disconnected, attempting reconnect...")
						if err := cli.Connect(); err != nil {
							logrus.Warnf("[AutoReconnect] Reconnect failed: %v", err)
						} else {
							logrus.Info("[AutoReconnect] Reconnected successfully")
						}
					}
				case <-stopReconnect:
					logrus.Info("[AutoReconnect] Stopping reconnect checker")
					return
				}
			}
		}()
	})
}

// SetAutoReconnectCheckingMultiClient starts a background goroutine that periodically checks
// ALL registered WhatsApp clients and reconnects any that are disconnected.
// This is for multi-client mode - it monitors every client in the registry.
func SetAutoReconnectCheckingMultiClient() {
	reconnectOnce.Do(func() {
		logrus.Info("[AutoReconnect] Starting multi-client reconnect checker (every 1 minute)")
		go func() {
			defer close(reconnectStopped)
			ticker := time.NewTicker(1 * time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					checkAndReconnectAllClients()
				case <-stopReconnect:
					logrus.Info("[AutoReconnect] Stopping multi-client reconnect checker")
					return
				}
			}
		}()
	})
}

// checkAndReconnectAllClients checks all registered clients and reconnects any that are disconnected
func checkAndReconnectAllClients() {
	registry := whatsapp.GetRegistry()
	if registry == nil {
		logrus.Warn("[AutoReconnect] Registry not available")
		return
	}

	clients := registry.GetAllClients()
	if len(clients) == 0 {
		logrus.Debug("[AutoReconnect] No clients registered")
		return
	}

	for _, mc := range clients {
		if mc == nil || mc.Client == nil {
			continue
		}

		// Only attempt reconnect for clients that should be connected (logged in or were connected)
		if !mc.Client.IsConnected() && mc.Client.IsLoggedIn() {
			logrus.Infof("[AutoReconnect][%s] Client disconnected, attempting reconnect...", mc.Phone)
			if err := mc.Client.Connect(); err != nil {
				logrus.Warnf("[AutoReconnect][%s] Reconnect failed: %v", mc.Phone, err)
			} else {
				logrus.Infof("[AutoReconnect][%s] Reconnected successfully", mc.Phone)
				mc.SetStatus(whatsapp.StatusLoggedIn)
			}
		}
	}
}

// StopAutoReconnectChecking gracefully stops the auto-reconnect loop
func StopAutoReconnectChecking() {
	reconnectStopOnce.Do(func() {
		close(stopReconnect)
		// Wait for goroutine to finish with timeout
		select {
		case <-reconnectStopped:
			logrus.Debug("[AutoReconnect] Checker stopped")
		case <-time.After(5 * time.Second):
			logrus.Warn("[AutoReconnect] Checker stop timeout")
		}
	})
}

func MultipartFormFileHeaderToBytes(fileHeader *multipart.FileHeader) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}
