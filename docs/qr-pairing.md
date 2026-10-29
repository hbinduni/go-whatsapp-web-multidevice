# QR pairing lifecycle

QR login now has one owner per gateway. Repeated `GET /app/login` requests read the active QR instead of disconnecting and starting another login. The response includes `qr_session_id`, `qr_duration` (remaining whole seconds), and `qr_expires_at` (Unix milliseconds). Clients may poll only when the session ID is present; older releases restart pairing on every login request.

The previous implementation sent every rotated QR image to an unbuffered channel, but the HTTP handler read it only once. The next rotation blocked the reader, so pairing completion and failure events stopped being consumed. Cancelling the library QR context and immediately reconnecting the same client also allowed old cleanup or event handlers to affect the replacement connection. Concurrent login requests were not serialized.

The managed session consumes WhatsApp events until completion, publishes rotated images without waiting for an HTTP consumer, and removes its event handler before cleanup completes. Refreshes share the session. Switching to phone-code pairing, logout, stop and explicit reconnect wait for cleanup. Startup and periodic reconnect only operate on saved sessions under the same lifecycle lock. A successful pairing leaves Whatsmeow's normal reconnect intact.

QR images are removed on rotation and session completion. Consumers should not cache them, should use the relative duration to avoid clock skew, and should stop polling when the dialog closes. Failures are logged without QR payloads or pairing codes. Passkey challenges still require a supported user authentication flow; this change does not bypass WhatsApp account restrictions.

Regression coverage is in `src/usecase/qr_pairing_test.go`: concurrent refresh, rotation without an HTTP consumer, secret rotation, stop-before-replacement, failed handshake/render/connect, expiry, saved-session protection and concurrent client replacement. Run `go test -race ./...` and `go vet ./...` from `src/`.
