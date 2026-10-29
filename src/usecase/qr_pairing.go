package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// One owner controls connection changes. QR refreshes share the current session,
// and switching methods waits for the old handler and connection to stop.
type qrPairingManager struct {
	mu      sync.Mutex
	session *qrPairingSession
}

var pairings qrPairingManager

var errQRExpired = errors.New("WhatsApp QR expired; request a new QR")
var errQRCancelled = errors.New("QR pairing cancelled")

type pairingClient interface {
	AddEventHandler(whatsmeow.EventHandler) uint32
	RemoveEventHandler(uint32) bool
	Connect() error
	Disconnect()
	HasSession() bool
}

type whatsappPairingClient struct{ *whatsmeow.Client }

func (c whatsappPairingClient) HasSession() bool { return c.Store.ID != nil || c.IsLoggedIn() }

func (m *qrPairingManager) stopLocked() {
	if m.session != nil {
		m.session.stop()
		m.session = nil
	}
}

func (m *qrPairingManager) login(ctx context.Context, getClient func() pairingClient) (domainApp.LoginResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	client := getClient()
	if client == nil {
		return domainApp.LoginResponse{}, pkgError.ErrWaCLI
	}
	if m.session != nil {
		select {
		case <-m.session.done:
			m.session.mu.Lock()
			terminal := m.session.err
			m.session.mu.Unlock()
			m.stopLocked()
			if terminal != nil && !errors.Is(terminal, errQRExpired) && !errors.Is(terminal, errQRCancelled) && !errors.Is(terminal, pkgError.ErrAlreadyLoggedIn) {
				return domainApp.LoginResponse{}, terminal
			}
		default:
		}
	}
	if client.HasSession() {
		return domainApp.LoginResponse{}, pkgError.ErrAlreadyLoggedIn
	}
	if m.session == nil {
		client.Disconnect()
		m.session = newQRPairingSession(client)
		if err := client.Connect(); err != nil {
			m.stopLocked()
			return domainApp.LoginResponse{}, fmt.Errorf("connect for QR pairing: %w", err)
		}
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	response, err := m.session.current(waitCtx)
	if err != nil && !errors.Is(err, pkgError.ErrAlreadyLoggedIn) {
		m.stopLocked()
	}
	return response, err
}

type qrPairingSession struct {
	client    pairingClient
	id        string
	cancel    context.CancelFunc
	done      chan struct{}
	incoming  chan any
	handlerID uint32
	mu        sync.Mutex
	changed   chan struct{}
	response  domainApp.LoginResponse
	expires   time.Time
	err       error
}

func newQRPairingSession(client pairingClient) *qrPairingSession {
	ctx, cancel := context.WithCancel(context.Background())
	s := &qrPairingSession{client: client, id: utils.UUIDv4(), cancel: cancel, done: make(chan struct{}), incoming: make(chan any, 16), changed: make(chan struct{})}
	s.handlerID = client.AddEventHandler(func(evt any) {
		switch evt.(type) {
		case *events.QR, *events.RotateADVSecret, *events.PairSuccess, *events.PairError,
			*events.PairPasskeyRequest, *events.PairPasskeyError, *events.ClientOutdated,
			*events.QRScannedWithoutMultidevice, *events.Disconnected, *events.ConnectFailure,
			*events.LoggedOut, *events.TemporaryBan:
			select {
			case s.incoming <- evt:
			case <-ctx.Done():
			}
		}
	})
	go s.run(ctx)
	return s
}

func (s *qrPairingSession) stop() { s.cancel(); <-s.done }

func (s *qrPairingSession) current(ctx context.Context) (domainApp.LoginResponse, error) {
	for {
		s.mu.Lock()
		result, expiry, err, changed := s.response, s.expires, s.err, s.changed
		s.mu.Unlock()
		if err != nil {
			return domainApp.LoginResponse{}, err
		}
		// Leave a second for transport so the browser never advertises a QR past
		// the gateway's rotation. The absolute expiry also covers slow responses.
		remaining := time.Until(expiry) - time.Second
		if result.ImagePath != "" && remaining > 0 {
			result.Duration = time.Duration(math.Floor(remaining.Seconds()))
			if result.Duration > 0 {
				return result, nil
			}
		}
		select {
		case <-ctx.Done():
			return domainApp.LoginResponse{}, fmt.Errorf("waiting for WhatsApp QR: %w", ctx.Err())
		case <-changed:
		case <-s.done:
			s.mu.Lock()
			err = s.err
			s.mu.Unlock()
			if err == nil {
				err = errors.New("QR pairing stopped; request a new QR")
			}
			return domainApp.LoginResponse{}, err
		}
	}
}

func (s *qrPairingSession) publish(code string, expiry time.Time) error {
	code = strings.TrimPrefix(code, "https://wa.me/settings/linked_devices#")
	filename := fmt.Sprintf("%s/scan-qr-%s.png", config.PathQrCode, utils.UUIDv4())
	if err := qrcode.WriteFile(code, qrcode.Medium, 512, filename); err != nil {
		return fmt.Errorf("render WhatsApp QR: %w", err)
	}
	s.mu.Lock()
	previous := s.response.ImagePath
	s.response = domainApp.LoginResponse{Code: code, ImagePath: filename, SessionID: s.id, ExpiresAt: expiry.UnixMilli()}
	s.expires = expiry
	close(s.changed)
	s.changed = make(chan struct{})
	s.mu.Unlock()
	if previous != "" {
		_ = os.Remove(previous)
	}
	return nil
}

func (s *qrPairingSession) finish(err error) {
	s.mu.Lock()
	s.err = err
	close(s.changed)
	s.changed = make(chan struct{})
	s.mu.Unlock()
}

func (s *qrPairingSession) run(ctx context.Context) {
	var terminal error
	paired := false
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer func() {
		timer.Stop()
		// Unblock an event callback before RemoveEventHandler takes its write lock.
		s.cancel()
		s.client.RemoveEventHandler(s.handlerID)
		if !paired && !s.client.HasSession() {
			s.client.Disconnect()
		}
		if terminal == nil {
			terminal = errQRCancelled
		}
		s.finish(terminal)
		s.mu.Lock()
		filename := s.response.ImagePath
		s.mu.Unlock()
		if filename != "" {
			_ = os.Remove(filename)
		}
		close(s.done)
	}()
	var codes []string
	var current string
	var expires time.Time
	var tick <-chan time.Time
	emitNext := func() error {
		if len(codes) == 0 {
			return errQRExpired
		}
		lifetime := 20 * time.Second
		if len(codes) == 6 {
			lifetime = 60 * time.Second
		}
		current, codes = codes[0], codes[1:]
		expires = time.Now().Add(lifetime)
		if err := s.publish(current, expires); err != nil {
			return err
		}
		timer.Reset(lifetime)
		tick = timer.C
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			if terminal = emitNext(); terminal != nil {
				logrus.Info("QR pairing expired")
				return
			}
		case raw := <-s.incoming:
			switch evt := raw.(type) {
			case *events.QR:
				codes = append([]string(nil), evt.Codes...)
				if terminal = emitNext(); terminal != nil {
					return
				}
			case *events.RotateADVSecret:
				// WhatsApp can rotate the authentication secret immediately after a scan.
				// Publish a new image without blocking on the HTTP request that already ended.
				current = strings.Replace(current, evt.OldSecret, evt.NewSecret, 1)
				for i := range codes {
					codes[i] = strings.Replace(codes[i], evt.OldSecret, evt.NewSecret, 1)
				}
				if current != "" {
					if terminal = s.publish(current, expires); terminal != nil {
						return
					}
				}
			case *events.PairSuccess:
				paired = true
				terminal = pkgError.ErrAlreadyLoggedIn
				logrus.Info("QR pairing completed")
				return
			case *events.PairError:
				terminal = fmt.Errorf("WhatsApp rejected QR pairing: %w", evt.Error)
			case *events.PairPasskeyRequest:
				terminal = errors.New("WhatsApp requires passkey verification; this gateway cannot complete that linking method")
			case *events.PairPasskeyError:
				terminal = fmt.Errorf("WhatsApp passkey verification failed: %w", evt.Error)
			case *events.ClientOutdated:
				terminal = errors.New("WhatsApp rejected the gateway version; update the gateway before retrying")
			case *events.QRScannedWithoutMultidevice:
				terminal = errors.New("scan from WhatsApp Settings > Linked Devices > Link a Device")
			case *events.Disconnected:
				terminal = errors.New("WhatsApp disconnected during QR pairing; request a new QR")
			case *events.ConnectFailure:
				terminal = fmt.Errorf("WhatsApp rejected the connection (reason %v)", evt.Reason)
			case *events.LoggedOut, *events.TemporaryBan:
				terminal = errors.New("WhatsApp rejected the linking session; check the phone before retrying")
			}
			if terminal != nil {
				logrus.Warn(terminal)
				return
			}
		}
	}
}
