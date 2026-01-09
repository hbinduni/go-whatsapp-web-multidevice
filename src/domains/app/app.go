package app

import (
	"context"
	"time"
)

type IAppUsecase interface {
	Login(ctx context.Context) (response LoginResponse, err error)
	LoginWithCode(ctx context.Context, phoneNumber string) (loginCode string, err error)
	Logout(ctx context.Context) (err error)
	Reconnect(ctx context.Context) (err error)
	FirstDevice(ctx context.Context) (response DevicesResponse, err error)
	FetchDevices(ctx context.Context) (response []DevicesResponse, err error)

	// StopClient disconnects from WhatsApp without logging out (session remains valid)
	// This is useful for maintenance operations like importing WhatsApp store backups
	StopClient(ctx context.Context) (response ClientStatusResponse, err error)

	// StartClient reconnects to WhatsApp after being stopped
	StartClient(ctx context.Context) (response ClientStatusResponse, err error)

	// GetClientStatus returns the current client connection status
	GetClientStatus(ctx context.Context) (response ClientStatusResponse, err error)
}

type DevicesResponse struct {
	Name   string `json:"name"`
	Device string `json:"device"`
}

type LoginResponse struct {
	ImagePath string        `json:"image_path"`
	Duration  time.Duration `json:"duration"`
	Code      string        `json:"code"`
}

// ClientStatusResponse contains the current client connection status
type ClientStatusResponse struct {
	IsConnected   bool   `json:"is_connected"`
	IsLoggedIn    bool   `json:"is_logged_in"`
	IsStopped     bool   `json:"is_stopped"`      // True if client was intentionally stopped
	DeviceJID     string `json:"device_jid,omitempty"`
	StatusMessage string `json:"status_message"`
}
