// Package auth implements one-phone pairing: a short-lived, single-use
// pairing token is exchanged for a session id that the phone keeps as a
// cookie. A separately paired native companion has its own credential.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PairWindow is how long a pairing token stays valid.
const PairWindow = 10 * time.Minute

// SessionTTL is the sliding session lifetime: a phone that has not used
// CopyApp for this long must pair again.
const SessionTTL = 180 * 24 * time.Hour

const secretLen = 32 // bytes; 256-bit

type Auth struct {
	mu        sync.Mutex
	token     string
	openUntil time.Time
	companion string
	companionUsed time.Time
	session   string
	lastUsed  time.Time // last successful Valid (sliding expiry)
	savedUse  time.Time // lastUsed as last written to path
	path      string    // optional persistence file for the session
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
// older browser session. The companion is preserved. It fails if the token
// is wrong, used, or expired.
func (a *Auth) Exchange(token string) (string, bool) {
	sid, ok, _ := a.ExchangeWithError(token)
	return sid, ok
}

// ExchangeWithError also reports persistence failures. A browser must not
// receive a session cookie until the new session is durably committed.
func (a *Auth) ExchangeWithError(token string) (string, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.openLocked() || !eq(token, a.token) {
		return "", false, nil
	}
	a.token = ""
	a.openUntil = time.Time{}
	oldSession, oldUsed, oldSaved := a.session, a.lastUsed, a.savedUse
	a.session = randomHex()
	a.lastUsed = a.now()
	err := a.saveLocked()
	if err == nil && a.path != "" {
		err = os.Remove(a.path + ".revoked")
		if os.IsNotExist(err) {
			err = nil
		}
	}
	if err != nil {
		a.session, a.lastUsed, a.savedUse = oldSession, oldUsed, oldSaved
		return "", false, err
	}
	return a.session, true, nil
}

// Valid reports whether cookie is the current session id. A session unused
// for SessionTTL expires; each use extends it.
func (a *Auth) Valid(cookie string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !eq(cookie, a.session) {
		return false
	}
	now := a.now()
	if !a.lastUsed.IsZero() && now.Sub(a.lastUsed) > SessionTTL {
		a.session, a.lastUsed = "", time.Time{}
		a.saveLocked()
		return false
	}
	a.lastUsed = now
	if now.Sub(a.savedUse) > time.Hour {
		a.saveLocked()
	}
	return true
}

// Paired reports whether a browser or companion session exists.
func (a *Auth) Paired() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.session != "" || a.companion != ""
}

// Revoke drops both client credentials and any open pairing token (Re-pair).
func (a *Auth) Revoke() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token, a.session, a.companion = "", "", ""
	a.openUntil = time.Time{}
	return a.saveLocked()
}

// Persist makes the session survive restarts by storing it in path (mode
// 0600). An existing valid session in path is loaded.
func (a *Auth) Persist(path string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.path = path
	a.session, a.lastUsed, a.savedUse = "", time.Time{}, time.Time{}
	a.companion, a.companionUsed = "", time.Time{}
	// The tombstone takes precedence even if the old session file was locked
	// when Unpair ran. Never restore a session whose revocation was committed.
	if _, err := os.Stat(path + ".revoked"); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Legacy "SESSION [LAST-USED-UNIX]", or the backward-compatible extension
	// "BROWSER LAST-USED-UNIX COMPANION COMPANION-LAST-USED-UNIX". A missing
	// browser is represented by "-" for companion-only installations.
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return nil
	}
	if len(f) >= 4 && len(f[2]) == 2*secretLen {
		if _, err := hex.DecodeString(f[2]); err == nil {
			if u, err := strconv.ParseInt(f[3], 10, 64); err == nil && u > 0 && a.now().Sub(time.Unix(u, 0)) <= SessionTTL {
				a.companion, a.companionUsed = f[2], time.Unix(u, 0)
			}
		}
	}
	s := f[0]
	if s == "-" { return nil }
	if _, err := hex.DecodeString(s); err != nil || len(s) != 2*secretLen {
		return nil
	}
	used := a.now()
	if len(f) > 1 {
		if u, err := strconv.ParseInt(f[1], 10, 64); err == nil && u > 0 {
			used = time.Unix(u, 0)
		}
	}
	if a.now().Sub(used) > SessionTTL {
		a.saveLocked() // expired: forget it
		return nil
	}
	a.session, a.lastUsed, a.savedUse = s, used, used
	return nil
}

func (a *Auth) saveLocked() error {
	if a.path == "" {
		return nil
	}
	if a.session == "" && a.companion == "" {
		// Commit a tombstone before attempting deletion: Windows can refuse
		// deletion of a read-only or open file while the directory is writable.
		_, markerErr := os.Stat(a.path + ".revoked")
		if os.IsNotExist(markerErr) {
			markerErr = writeAtomic(a.path+".revoked", []byte("revoked\n"))
		}
		removeErr := os.Remove(a.path)
		if markerErr == nil || removeErr == nil || os.IsNotExist(removeErr) {
			return nil
		}
		return errors.Join(markerErr, removeErr)
	}
	browser, companion := a.session, a.companion
	if browser == "" { browser = "-" }
	if companion == "" { companion = "-" }
	err := writeAtomic(a.path, []byte(browser+" "+strconv.FormatInt(a.lastUsed.Unix(), 10)+" "+companion+" "+strconv.FormatInt(a.companionUsed.Unix(), 10)))
	if err == nil {
		a.savedUse = a.lastUsed
	}
	return err
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}
