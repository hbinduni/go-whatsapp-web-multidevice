package cmd

import (
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
)

// startAutoReconnectCheckerIfClientAvailable starts a checker that monitors ALL registered clients
func startAutoReconnectCheckerIfClientAvailable() {
	go helpers.SetAutoReconnectCheckingMultiClient()
}
