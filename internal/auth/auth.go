// Package auth implements one-phone pairing: a short-lived, single-use
// pairing token is exchanged for a session id that the phone keeps as a
// cookie. Only one session exists at a time.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PairWindow is how long a pairing token stays valid.
const PairWindow = 10 * time.Minute

const secretLen = 32 // bytes; 256-bit

type Auth struct {
	mu        sync.Mutex
	token     string
	openUntil time.Time
	session   string
	path      string // optional persistence file for the session
	now       func() time.Time
}

func New() *Auth { return &Auth{now: time.Now} }

// SetClock replaces the time source (for tests).
func (a *Auth) SetClock(now func() time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.now = now
}

func randomHex() string {
	var b [secretLen]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("auth: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

func eq(a, b string) bool {
	return a != "" && b != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// OpenPairing starts (or restarts) the pairing window and returns a fresh
// single-use token. Any previous token is invalidated.
func (a *Auth) OpenPairing() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = randomHex()
	a.openUntil = a.now().Add(PairWindow)
	return a.token
}

func (a *Auth) openLocked() bool {
	return a.token != "" && a.now().Before(a.openUntil)
}

// PairingOpen reports whether a token is currently redeemable.
func (a *Auth) PairingOpen() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.openLocked()
}

// PairingDeadline returns when the current pairing window closes (zero if
// pairing is closed).
func (a *Auth) PairingDeadline() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.openLocked() {
		return time.Time{}
	}
	return a.openUntil
}

// Exchange consumes the token and returns a new session id, replacing any
// older session. It fails if the token is wrong, used, or expired.
func (a *Auth) Exchange(token string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.openLocked() || !eq(token, a.token) {
		return "", false
	}
	a.token = ""
	a.openUntil = time.Time{}
	a.session = randomHex()
	a.saveLocked()
	return a.session, true
}

// Valid reports whether cookie is the current session id.
func (a *Auth) Valid(cookie string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return eq(cookie, a.session)
}

// Paired reports whether a session exists.
func (a *Auth) Paired() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.session != ""
}

// Revoke drops the session and any open pairing token (Re-pair).
func (a *Auth) Revoke() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token, a.session = "", ""
	a.openUntil = time.Time{}
	a.saveLocked()
}

// Persist makes the session survive restarts by storing it in path (mode
// 0600). An existing valid session in path is loaded.
func (a *Auth) Persist(path string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.path = path
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	s := strings.TrimSpace(string(b))
	if _, err := hex.DecodeString(s); err == nil && len(s) == 2*secretLen {
		a.session = s
	}
	return nil
}

func (a *Auth) saveLocked() {
	if a.path == "" {
		return
	}
	if a.session == "" {
		os.Remove(a.path)
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(a.path), ".session-*")
	if err != nil {
		return
	}
	_, err = tmp.WriteString(a.session)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), a.path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
}
