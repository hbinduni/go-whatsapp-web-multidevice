package cmd

import (
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/sirupsen/logrus"
)

// startAutoReconnectCheckerIfClientAvailable guards the reconnect checker behind a valid client reference.
// In multi-client mode, this uses the first available client from the registry.
func startAutoReconnectCheckerIfClientAvailable() {
	client := whatsapp.GetClient()
	if client == nil {
		logrus.Warn("No WhatsApp client available; auto-reconnect checker not started")
		return
	}
	go helpers.SetAutoReconnectChecking(client)
}
