package auth

import (
	"os"
	"time"
)

// ExchangeCompanion consumes a fresh, single-use pairing token and replaces only
// the native credential. Existing browser credentials remain valid.
func (a *Auth) ExchangeCompanion(token string) (string, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.openLocked() || !eq(token, a.token) { return "", false, nil }
	a.token, a.openUntil = "", time.Time{}
	old, used, saved := a.companion, a.companionUsed, a.savedUse
	a.companion, a.companionUsed = randomHex(), a.now()
	err := a.saveLocked()
	if err == nil && a.path != "" {
		err = os.Remove(a.path + ".revoked")
		if os.IsNotExist(err) { err = nil }
	}
	if err != nil { a.companion, a.companionUsed, a.savedUse = old, used, saved; return "", false, err }
	return a.companion, true, nil
}

// ValidCompanion authorizes only the companion endpoint, with independent expiry.
func (a *Auth) ValidCompanion(value string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !eq(value, a.companion) { return false }
	now := a.now()
	if now.Sub(a.companionUsed) > SessionTTL {
		a.companion, a.companionUsed = "", time.Time{}
		a.saveLocked()
		return false
	}
	if now.Sub(a.companionUsed) > time.Hour {
		a.companionUsed = now
		a.saveLocked()
	}
	return true
}
