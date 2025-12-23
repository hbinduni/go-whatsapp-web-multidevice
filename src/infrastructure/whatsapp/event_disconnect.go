package whatsapp

import (
	"context"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/sirupsen/logrus"
)

// createDisconnectPayload creates a webhook payload for device disconnect/logout events
func createDisconnectPayload(reason string) map[string]any {
	body := make(map[string]any)

	// Add device_jid to identify which WhatsApp account disconnected
	if cli != nil && cli.Store != nil && cli.Store.ID != nil {
		body["device_jid"] = cli.Store.ID.String()
	}

	// Create payload structure
	payload := make(map[string]any)
	payload["reason"] = reason
	payload["status"] = "disconnected"

	// Wrap in payload structure
	body["payload"] = payload

	// Add metadata for webhook processing
	body["event"] = "device.disconnected"
	body["timestamp"] = time.Now().Format(time.RFC3339)

	return body
}

// forwardDisconnectToWebhook forwards device disconnect events to the configured webhook URLs
func forwardDisconnectToWebhook(ctx context.Context, reason string) error {
	if len(config.WhatsappWebhook) == 0 {
		logrus.Debug("No webhook URLs configured, skipping disconnect event forwarding")
		return nil
	}

	payload := createDisconnectPayload(reason)
	return forwardPayloadToConfiguredWebhooks(ctx, payload, "device disconnect event")
}
