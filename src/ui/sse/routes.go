package sse

import (
	"bufio"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

// RegisterRoutes registers SSE routes with the Fiber app
func RegisterRoutes(app fiber.Router) {
	app.Get("/events", handleSSE)
	app.Get("/events/status", handleSSEStatus)
}

// handleSSE handles SSE connections for real-time events
func handleSSE(c *fiber.Ctx) error {
	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Headers", "Cache-Control")
	c.Set("Access-Control-Allow-Credentials", "true")

	// Create a new client
	client := NewClient()
	hub := GetHub()

	// Register client with hub
	hub.Register(client)

	logrus.Infof("[SSE] New client connected: %s from %s", client.ID, c.IP())

	// Send initial connection event
	initialEvent := Event{
		Type:      EventConnectionStatus,
		Code:      "CONNECTED",
		Message:   "SSE connection established",
		Timestamp: time.Now(),
		Data: map[string]any{
			"client_id":    client.ID,
			"connected_at": time.Now(),
		},
	}

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Send initial event
		fmt.Fprint(w, FormatSSE(initialEvent))
		w.Flush()

		// Keep-alive ticker (send comment every 30 seconds to prevent timeout)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case event, ok := <-client.Channel:
				if !ok {
					// Channel closed, client disconnected
					logrus.Infof("[SSE] Client channel closed: %s", client.ID)
					return
				}
				// Send event to client
				sseData := FormatSSE(event)
				if sseData != "" {
					_, err := fmt.Fprint(w, sseData)
					if err != nil {
						logrus.Errorf("[SSE] Error writing to client %s: %v", client.ID, err)
						hub.Unregister(client.ID)
						return
					}
					err = w.Flush()
					if err != nil {
						logrus.Errorf("[SSE] Error flushing to client %s: %v", client.ID, err)
						hub.Unregister(client.ID)
						return
					}
				}

			case <-ticker.C:
				// Send keep-alive comment
				_, err := fmt.Fprint(w, ": keep-alive\n\n")
				if err != nil {
					logrus.Debugf("[SSE] Keep-alive failed for client %s: %v", client.ID, err)
					hub.Unregister(client.ID)
					return
				}
				err = w.Flush()
				if err != nil {
					logrus.Debugf("[SSE] Keep-alive flush failed for client %s: %v", client.ID, err)
					hub.Unregister(client.ID)
					return
				}
			}
		}
	})

	return nil
}

// handleSSEStatus returns the current SSE hub status
func handleSSEStatus(c *fiber.Ctx) error {
	hub := GetHub()
	return c.JSON(fiber.Map{
		"status":        "ok",
		"client_count":  hub.ClientCount(),
		"server_time":   time.Now(),
	})
}
