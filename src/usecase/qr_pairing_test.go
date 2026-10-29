package usecase

import (
	"context"
	"errors"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// Only the WhatsApp network boundary is replaced; lifecycle, rendering, timers
// and concurrent refreshes exercise the production code.
type pairingPeer struct {
	mu             sync.Mutex
	handler        whatsmeow.EventHandler
	connects       int
	saved          bool
	connectError   error
	onConnect      any
	disconnectGate chan struct{}
}

func (p *pairingPeer) AddEventHandler(h whatsmeow.EventHandler) uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handler = h
	return 1
}
func (p *pairingPeer) RemoveEventHandler(uint32) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handler = nil
	return true
}
func (p *pairingPeer) Connect() error {
	p.mu.Lock()
	p.connects++
	evt, err := p.onConnect, p.connectError
	p.mu.Unlock()
	if evt == nil {
		evt = &events.QR{Codes: []string{"first,secret", "second,secret", "third,secret", "fourth,secret", "fifth,secret", "sixth,secret"}}
	}
	if err == nil {
		p.emit(evt)
	}
	return err
}
func (p *pairingPeer) Disconnect() {
	p.mu.Lock()
	gate := p.disconnectGate
	p.mu.Unlock()
	if gate != nil {
		<-gate
	}
}
func (p *pairingPeer) HasSession() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.saved }
func (p *pairingPeer) emit(evt any) {
	p.mu.Lock()
	h := p.handler
	p.mu.Unlock()
	if h != nil {
		h(evt)
	}
}
func (p *pairingPeer) connections() int { p.mu.Lock(); defer p.mu.Unlock(); return p.connects }
func qrTestManager(t *testing.T) (*qrPairingManager, *pairingPeer) {
	t.Helper()
	old := config.PathQrCode
	config.PathQrCode = t.TempDir()
	m, p := &qrPairingManager{}, &pairingPeer{}
	t.Cleanup(func() { m.mu.Lock(); m.stopLocked(); m.mu.Unlock(); config.PathQrCode = old })
	return m, p
}
func TestQRRotationDoesNotBlockPairingCompletion(t *testing.T) {
	m, p := qrTestManager(t)
	first, err := m.login(context.Background(), func() pairingClient { return p })
	if err != nil {
		t.Fatal(err)
	}
	p.emit(&events.RotateADVSecret{OldSecret: "secret", NewSecret: "rotated"})
	p.mu.Lock()
	p.saved = true
	p.mu.Unlock()
	p.emit(&events.PairSuccess{})
	select {
	case <-m.session.done:
	case <-time.After(time.Second):
		t.Fatal("QR rotation blocked pairing completion after the HTTP response returned")
	}
	if _, err = os.Stat(first.ImagePath); !os.IsNotExist(err) {
		t.Fatalf("completed pairing left its QR image: %v", err)
	}
}
func TestQRConcurrentRefreshKeepsOnePairingSession(t *testing.T) {
	m, p := qrTestManager(t)
	var wg sync.WaitGroup
	results := make(chan string, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := m.login(context.Background(), func() pairingClient { return p })
			if err != nil {
				t.Error(err)
				return
			}
			results <- r.SessionID + "|" + r.Code
		}()
	}
	wg.Wait()
	close(results)
	var first string
	for got := range results {
		if first == "" {
			first = got
		}
		if got != first {
			t.Fatalf("refresh invalidated a QR already returned to another caller: %q != %q", got, first)
		}
	}
	if p.connections() != 1 {
		t.Fatalf("refresh opened %d connections, want 1", p.connections())
	}
}
func TestQRSecretRotationPublishesCurrentImage(t *testing.T) {
	m, p := qrTestManager(t)
	first, err := m.login(context.Background(), func() pairingClient { return p })
	if err != nil {
		t.Fatal(err)
	}
	s := m.session
	s.mu.Lock()
	changed := s.changed
	s.mu.Unlock()
	p.emit(&events.RotateADVSecret{OldSecret: "secret", NewSecret: "new-secret"})
	<-changed
	current, err := m.login(context.Background(), func() pairingClient { return p })
	if err != nil {
		t.Fatal(err)
	}
	if current.Code != "first,new-secret" || current.ImagePath == first.ImagePath || current.SessionID != first.SessionID {
		t.Fatalf("rotation did not replace the current QR inside the same session: %+v", current)
	}
	if _, err = os.Stat(current.ImagePath); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(first.ImagePath); !os.IsNotExist(err) {
		t.Fatal("invalidated QR image still exists")
	}
}
func TestQRStopsBeforeStartingReplacement(t *testing.T) {
	m, p := qrTestManager(t)
	if _, err := m.login(context.Background(), func() pairingClient { return p }); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	p.mu.Lock()
	p.disconnectGate = gate
	p.mu.Unlock()
	stopped := make(chan struct{})
	go func() { m.mu.Lock(); m.stopLocked(); m.mu.Unlock(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("stop returned before connection cleanup completed")
	case <-time.After(20 * time.Millisecond):
	}
	p.mu.Lock()
	p.disconnectGate = nil
	p.mu.Unlock()
	close(gate)
	<-stopped
	next, err := m.login(context.Background(), func() pairingClient { return p })
	if err != nil {
		t.Fatal(err)
	}
	if next.Code != "first,secret" || p.connections() != 2 {
		t.Fatal("replacement did not start cleanly")
	}
}
func TestQRHandshakeErrorReturnsWithoutWaitingForImage(t *testing.T) {
	m, p := qrTestManager(t)
	p.onConnect = &events.PairError{Error: errors.New("signature mismatch")}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := m.login(ctx, func() pairingClient { return p })
	if err == nil || !strings.Contains(err.Error(), "signature mismatch") {
		t.Fatalf("want pairing failure, got %v", err)
	}
	if m.session != nil {
		t.Fatal("failed session was not cleaned up")
	}
}
func TestQRConnectAndRenderFailuresCanRetry(t *testing.T) {
	m, p := qrTestManager(t)
	p.connectError = errors.New("network unavailable")
	if _, err := m.login(context.Background(), func() pairingClient { return p }); err == nil {
		t.Fatal("connect failure was ignored")
	}
	p.connectError = nil
	goodDir := config.PathQrCode
	config.PathQrCode = goodDir + "/missing"
	if _, err := m.login(context.Background(), func() pairingClient { return p }); err == nil {
		t.Fatal("render failure was ignored")
	}
	config.PathQrCode = goodDir
	if _, err := m.login(context.Background(), func() pairingClient { return p }); err != nil {
		t.Fatal(err)
	}
}
func TestQRDoesNotDisconnectSavedSession(t *testing.T) {
	m, p := qrTestManager(t)
	p.saved = true
	if _, err := m.login(context.Background(), func() pairingClient { return p }); !errors.Is(err, pkgError.ErrAlreadyLoggedIn) {
		t.Fatalf("got %v", err)
	}
	if p.connections() != 0 || m.session != nil {
		t.Fatal("attempted to replace a paired device")
	}
}
func TestQRExpiryRotatesWithoutAnHTTPConsumer(t *testing.T) {
	m, p := qrTestManager(t)
	synctest.Test(t, func(t *testing.T) {
		first, err := m.login(context.Background(), func() pairingClient { return p })
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(61 * time.Second)
		next, err := m.login(context.Background(), func() pairingClient { return p })
		if err != nil {
			t.Fatal(err)
		}
		if next.Code != "second,secret" || next.SessionID != first.SessionID || next.Duration > 19 {
			t.Fatalf("got stale QR after rotation: %+v", next)
		}
		time.Sleep(101 * time.Second)
		synctest.Wait()
		select {
		case <-m.session.done:
		default:
			t.Fatal("expired QR session did not finish")
		}
		if _, err := m.login(context.Background(), func() pairingClient { return p }); err != nil {
			t.Fatal(err)
		}
		m.mu.Lock()
		m.stopLocked()
		m.mu.Unlock()
	})
}

func TestQRResolvesClientAfterConcurrentReplacement(t *testing.T) {
	m, old := qrTestManager(t)
	current := old
	m.mu.Lock()
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		close(started)
		_, err := m.login(context.Background(), func() pairingClient { return current })
		finished <- err
	}()
	<-started
	replacement := &pairingPeer{}
	current = replacement
	m.mu.Unlock()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if old.connections() != 0 || replacement.connections() != 1 {
		t.Fatal("login reused a client captured before logout replaced it")
	}
}

func TestQRTerminalFailureSurvivesUntilNextRefresh(t *testing.T) {
	m, p := qrTestManager(t)
	if _, err := m.login(context.Background(), func() pairingClient { return p }); err != nil {
		t.Fatal(err)
	}
	// A store Save failure can leave an ID even though pairing failed.
	p.mu.Lock()
	p.saved = true
	p.mu.Unlock()
	p.emit(&events.PairError{Error: errors.New("store save failed")})
	<-m.session.done
	_, err := m.login(context.Background(), func() pairingClient { return p })
	if err == nil || !strings.Contains(err.Error(), "store save failed") {
		t.Fatalf("terminal pairing failure was lost: %v", err)
	}
}
