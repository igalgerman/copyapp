package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenSingleUse(t *testing.T) {
	a := New()
	tok := a.OpenPairing()
	if _, ok := a.Exchange(tok); !ok {
		t.Fatal("first exchange must work")
	}
	if _, ok := a.Exchange(tok); ok {
		t.Fatal("second exchange must fail")
	}
}

func TestTokenExpires(t *testing.T) {
	a := New()
	base := time.Now()
	a.now = func() time.Time { return base }
	tok := a.OpenPairing()
	a.now = func() time.Time { return base.Add(PairWindow + time.Second) }
	if _, ok := a.Exchange(tok); ok {
		t.Fatal("expired token accepted")
	}
	if a.PairingOpen() {
		t.Fatal("pairing still open after expiry")
	}
}

func TestTokenValidJustBeforeExpiry(t *testing.T) {
	base := time.Now()
	a := New()
	a.now = func() time.Time { return base }
	tok := a.OpenPairing()
	a.now = func() time.Time { return base.Add(PairWindow - time.Second) }
	if !a.PairingOpen() {
		t.Fatal("pairing should be open")
	}
	if _, ok := a.Exchange(tok); !ok {
		t.Fatal("token rejected inside window")
	}
	if a.PairingOpen() {
		t.Fatal("pairing must close after use")
	}
}

func TestRevoke(t *testing.T) {
	a := New()
	c, _ := a.Exchange(a.OpenPairing())
	a.Revoke()
	if a.Valid(c) {
		t.Fatal("session survived revoke")
	}
	if a.Paired() {
		t.Fatal("still paired")
	}
}

func TestRevokeClosesPairing(t *testing.T) {
	a := New()
	tok := a.OpenPairing()
	a.Revoke()
	if _, ok := a.Exchange(tok); ok {
		t.Fatal("token survived revoke")
	}
}

func TestWrongAndEmptyTokens(t *testing.T) {
	a := New()
	if _, ok := a.Exchange(""); ok {
		t.Fatal("empty token accepted with pairing closed")
	}
	tok := a.OpenPairing()
	if len(tok) != 64 {
		t.Fatalf("token should be 32 bytes hex, got %d chars", len(tok))
	}
	for _, bad := range []string{"", "x", tok[:63], tok + "0"} {
		if _, ok := a.Exchange(bad); ok {
			t.Fatalf("bad token %q accepted", bad)
		}
	}
	if _, ok := a.Exchange(tok); !ok {
		t.Fatal("real token rejected after bad attempts")
	}
}

func TestValidAndReplace(t *testing.T) {
	a := New()
	if a.Valid("") {
		t.Fatal("empty cookie valid with no session")
	}
	c1, _ := a.Exchange(a.OpenPairing())
	if !a.Valid(c1) || a.Valid("") || a.Valid(c1[:10]) {
		t.Fatal("Valid wrong")
	}
	c2, _ := a.Exchange(a.OpenPairing())
	if c1 == c2 || a.Valid(c1) || !a.Valid(c2) {
		t.Fatal("new pairing must replace the old session")
	}
}

func TestOpenPairingRotatesToken(t *testing.T) {
	a := New()
	t1 := a.OpenPairing()
	t2 := a.OpenPairing()
	if t1 == t2 {
		t.Fatal("token not rotated")
	}
	if _, ok := a.Exchange(t1); ok {
		t.Fatal("old token still valid")
	}
}

func TestSetClock(t *testing.T) {
	base := time.Now()
	a := New()
	a.SetClock(func() time.Time { return base })
	tok := a.OpenPairing()
	if d := a.PairingDeadline(); !d.Equal(base.Add(PairWindow)) {
		t.Fatalf("deadline %v", d)
	}
	a.SetClock(func() time.Time { return base.Add(PairWindow) })
	if _, ok := a.Exchange(tok); ok {
		t.Fatal("token valid at deadline")
	}
	if !a.PairingDeadline().IsZero() {
		t.Fatal("deadline should be zero when closed")
	}
}

func TestPersist(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session")
	a := New()
	if err := a.Persist(p); err != nil {
		t.Fatal(err)
	}
	c, _ := a.Exchange(a.OpenPairing())
	b := New()
	if err := b.Persist(p); err != nil {
		t.Fatal(err)
	}
	if !b.Valid(c) {
		t.Fatal("session not restored")
	}
	b.Revoke()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("session file not removed on revoke")
	}
	cc := New()
	cc.Persist(p)
	if cc.Valid(c) {
		t.Fatal("revoked session restored")
	}
}

// Real users need time to switch Wi-Fi / mobile data before scanning.
func TestPairWindowIsTenMinutes(t *testing.T) {
	if PairWindow != 10*time.Minute {
		t.Fatalf("PairWindow = %v, want 10m", PairWindow)
	}
}
