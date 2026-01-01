package websocket

import (
	"context"
	"encoding/json"
	"sync"

	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/sirupsen/logrus"
)

type client struct{}

type BroadcastMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Result  any    `json:"result"`
}

var (
	clients    = make(map[*websocket.Conn]client)
	clientsMu  sync.RWMutex
	Register   = make(chan *websocket.Conn)
	Broadcast  = make(chan BroadcastMessage)
	Unregister = make(chan *websocket.Conn)
	stopHub    = make(chan struct{})
)

func handleRegister(conn *websocket.Conn) {
	clientsMu.Lock()
	clients[conn] = client{}
	count := len(clients)
	clientsMu.Unlock()
	logrus.Infof("[WebSocket] Client connected from %s (total: %d)", conn.RemoteAddr(), count)
}

func handleUnregister(conn *websocket.Conn) {
	clientsMu.Lock()
	delete(clients, conn)
	count := len(clients)
	clientsMu.Unlock()
	logrus.Infof("[WebSocket] Client disconnected from %s (total: %d)", conn.RemoteAddr(), count)
}

func broadcastMessage(message BroadcastMessage) {
	marshalMessage, err := json.Marshal(message)
	if err != nil {
		logrus.Errorf("[WebSocket] Failed to marshal message: %v", err)
		return
	}

	// Collect connections to remove (don't modify map during iteration)
	var toRemove []*websocket.Conn

	clientsMu.RLock()
	for conn := range clients {
		if err := conn.WriteMessage(websocket.TextMessage, marshalMessage); err != nil {
			logrus.Warnf("[WebSocket] Write error to %s: %v", conn.RemoteAddr(), err)
			toRemove = append(toRemove, conn)
		}
	}
	clientsMu.RUnlock()

	// Remove failed connections
	for _, conn := range toRemove {
		closeConnection(conn)
	}
}

func closeConnection(conn *websocket.Conn) {
	if err := conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
		logrus.Debugf("[WebSocket] Write close message error: %v", err)
	}
	if err := conn.Close(); err != nil {
		logrus.Debugf("[WebSocket] Close connection error: %v", err)
	}
	clientsMu.Lock()
	delete(clients, conn)
	clientsMu.Unlock()
}

// RunHub processes WebSocket events. Call StopHub() to gracefully shutdown.
func RunHub() {
	for {
		select {
		case conn := <-Register:
			handleRegister(conn)

		case conn := <-Unregister:
			handleUnregister(conn)

		case message := <-Broadcast:
			logrus.Debugf("[WebSocket] Broadcasting message: %s", message.Code)
			broadcastMessage(message)

		case <-stopHub:
			logrus.Info("[WebSocket] Hub shutting down")
			clientsMu.Lock()
			for conn := range clients {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				_ = conn.Close()
			}
			clients = make(map[*websocket.Conn]client)
			clientsMu.Unlock()
			return
		}
	}
}

// StopHub gracefully stops the WebSocket hub
func StopHub() {
	close(stopHub)
}

// ClientCount returns the number of connected WebSocket clients
func ClientCount() int {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	return len(clients)
}

func RegisterRoutes(app fiber.Router, service domainApp.IAppUsecase) {
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.SendStatus(fiber.StatusUpgradeRequired)
	})

	app.Get("/ws", websocket.New(func(conn *websocket.Conn) {
		defer func() {
			Unregister <- conn
			_ = conn.Close()
		}()

		Register <- conn

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logrus.Warnf("[WebSocket] Read error from %s: %v", conn.RemoteAddr(), err)
				}
				return
			}

			if messageType == websocket.TextMessage {
				var messageData BroadcastMessage
				if err := json.Unmarshal(message, &messageData); err != nil {
					logrus.Warnf("[WebSocket] Unmarshal error: %v", err)
					continue // Don't close connection on bad message
				}

				if messageData.Code == "FETCH_DEVICES" {
					devices, _ := service.FetchDevices(context.Background())
					Broadcast <- BroadcastMessage{
						Code:    "LIST_DEVICES",
						Message: "Device found",
						Result:  devices,
					}
				}
			} else {
				logrus.Debugf("[WebSocket] Unsupported message type: %d", messageType)
			}
		}
	}))
}
