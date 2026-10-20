package auth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCompanionCoexistsPersistsAndRevokes(t *testing.T) {
	a := New()
	path := filepath.Join(t.TempDir(), "session")
	if err := a.Persist(path); err != nil {
		t.Fatal(err)
	}
	browser, _ := a.Exchange(a.OpenPairing())
	companion, ok, err := a.ExchangeCompanion(a.OpenPairing())
	if err != nil || !ok {
		t.Fatalf("pair: %v %v", ok, err)
	}
	if !a.Valid(browser) || !a.ValidCompanion(companion) {
		t.Fatal("pair revoked another client")
	}
	if a.Valid(companion) || a.ValidCompanion(browser) {
		t.Fatal("credential crossed client scopes")
	}
	b := New()
	if err := b.Persist(path); err != nil {
		t.Fatal(err)
	}
	if !b.Valid(browser) || !b.ValidCompanion(companion) {
		t.Fatal("restart lost sessions")
	}
	if err := b.Revoke(); err != nil {
		t.Fatal(err)
	}
	c := New()
	if err := c.Persist(path); err != nil {
		t.Fatal(err)
	}
	if c.Valid(browser) || c.ValidCompanion(companion) || c.Paired() {
		t.Fatal("revoked clients restored")
	}
}

func TestCompanionPairingSingleUseAndIndependentExpiry(t *testing.T) {
	a := New()
	now := time.Now()
	a.SetClock(func() time.Time { return now })
	browser, _ := a.Exchange(a.OpenPairing())
	token := a.OpenPairing()
	companion, ok, err := a.ExchangeCompanion(token)
	if err != nil || !ok {
		t.Fatal("pair failed")
	}
	if _, ok, _ := a.ExchangeCompanion(token); ok {
		t.Fatal("reused token accepted")
	}
	now = now.Add(SessionTTL - time.Hour)
	if !a.Valid(browser) {
		t.Fatal("browser prematurely expired")
	}
	now = now.Add(2 * time.Hour)
	if a.ValidCompanion(companion) {
		t.Fatal("idle companion did not expire")
	}
	if !a.Valid(browser) {
		t.Fatal("companion expiry revoked browser")
	}
}

func TestCompanionOnlyPersistence(t *testing.T) {
	a := New()
	path := filepath.Join(t.TempDir(), "session")
	if err := a.Persist(path); err != nil {
		t.Fatal(err)
	}
	companion, _, _ := a.ExchangeCompanion(a.OpenPairing())
	b := New()
	if err := b.Persist(path); err != nil {
		t.Fatal(err)
	}
	if !b.ValidCompanion(companion) {
		t.Fatal("native-only session lost after restart")
	}
}

func TestCompanionReplacementAndSaveFailurePreserveOtherClient(t *testing.T) {
	a := New()
	browser, _ := a.Exchange(a.OpenPairing())
	first, _, _ := a.ExchangeCompanion(a.OpenPairing())
	second, ok, err := a.ExchangeCompanion(a.OpenPairing())
	if err != nil || !ok || a.ValidCompanion(first) || !a.ValidCompanion(second) || !a.Valid(browser) {
		t.Fatal("replacement crossed scopes or retained old native credential")
	}
	a.path = filepath.Join(t.TempDir(), "missing", "session")
	token := a.OpenPairing()
	if _, ok, err := a.ExchangeCompanion(token); ok || err == nil {
		t.Fatal("pair succeeded without durable credential")
	}
	if !a.ValidCompanion(second) || !a.Valid(browser) {
		t.Fatal("failed save lost existing credentials")
	}
	if _, ok, _ := a.ExchangeCompanion(token); ok {
		t.Fatal("failed save did not consume single-use token")
	}
}
