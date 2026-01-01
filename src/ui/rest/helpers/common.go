package helpers

import (
	"context"
	"io"
	"mime/multipart"
	"sync"
	"time"

	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
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
func SetAutoReconnectChecking(cli *whatsmeow.Client) {
	if cli == nil {
		logrus.Warn("[AutoReconnect] Called with nil WhatsApp client; skipping")
		return
	}

	reconnectOnce.Do(func() {
		logrus.Info("[AutoReconnect] Starting reconnect checker (every 5 minutes)")
		go func() {
			defer close(reconnectStopped)
			ticker := time.NewTicker(5 * time.Minute)
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
