package cmd

import (
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/sirupsen/logrus"
)

// startAutoReconnectCheckerIfClientAvailable guards the reconnect checker behind a valid client reference.
// In multi-client mode, this monitors ALL registered clients.
// In single-client mode, this uses the global client.
func startAutoReconnectCheckerIfClientAvailable() {
	if whatsapp.IsMultiClientMode() {
		// In multi-client mode, start a checker that monitors ALL clients
		go helpers.SetAutoReconnectCheckingMultiClient()
		return
	}

	// Single-client mode: monitor the global client
	client := whatsapp.GetClient()
	if client == nil {
		logrus.Warn("No WhatsApp client available; auto-reconnect checker not started")
		return
	}
	go helpers.SetAutoReconnectChecking(client)
}
