package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestEventsRechecksSessionBeforeSubscription(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	r := e.authed("GET", "/events", nil, c)
	// Simulate requireAuth having succeeded just before Unpair acquires
	// the server lock, then the authorized handler resuming afterward.
	if !e.s.authed(r) {
		t.Fatal("initial authentication failed")
	}
	e.s.Unpair()
	ctx, cancel := context.WithCancel(r.Context())
	cancel() // keep a regressed streaming implementation from hanging the test
	w := httptest.NewRecorder()
	e.s.events(w, r.WithContext(ctx))
	if w.Code != http.StatusUnauthorized || len(e.s.clients) != 0 {
		t.Fatalf("revoked handler subscribed: status=%d clients=%d", w.Code, len(e.s.clients))
	}
}

func TestControlBrowserTokenBootstrapAndRefresh(t *testing.T) {
	e := newEnv(t, 0)
	e.s.cfg.WiFiName = func() string { return "private-network-name" }
	e.s.cfg.LogPath = "private-log-path"
	e.s.StartPairing()
	bad := e.do(bareLocal(e, "GET", "/pairing?k=wrong"))
	if bad.Code != http.StatusForbidden {
		t.Fatalf("wrong key: %d", bad.Code)
	}
	w := e.do(bareLocal(e, "GET", "/pairing?k="+testLocalKey))
	if w.Code != http.StatusOK || len(w.Result().Cookies()) != 0 {
		t.Fatalf("bootstrap: %d cookies=%v", w.Code, w.Result().Cookies())
	}
	m := regexp.MustCompile(`var token="([A-Za-z0-9_-]+)"`).FindStringSubmatch(w.Body.String())
	if len(m) != 2 || m[1] == testLocalKey {
		t.Fatal("missing separate browser token")
	}
	token := m[1]
	if strings.Contains(w.Body.String(), testLocalKey) {
		t.Fatal("native secret embedded in page")
	}
	for _, path := range []string{"/local/state", "/qr", "/pairing/status"} {
		r := bareLocal(e, "GET", path)
		r.Header.Set("X-CopyApp-Session", token)
		if got := e.do(r); got.Code != http.StatusOK {
			t.Fatalf("browser token %s: %d", path, got.Code)
		}
		// A service at another loopback port may collect cookies, but those
		// cookies must never authorize either reads or mutations.
		r = bareLocal(e, "GET", path)
		r.AddCookie(&http.Cookie{Name: LocalCookieName, Value: token})
		if got := e.do(r); got.Code != http.StatusForbidden {
			t.Fatalf("ambient cookie %s: %d", path, got.Code)
		}
	}
	r := bareLocal(e, "POST", "/local/repair")
	r.AddCookie(&http.Cookie{Name: LocalCookieName, Value: token})
	if got := e.do(r); got.Code != http.StatusForbidden {
		t.Fatalf("cookie mutation: %d", got.Code)
	}
	// A navigation refresh does not carry the explicit header. It receives
	// only a shell whose JS recovers the token from origin-scoped storage.
	w = e.do(bareLocal(e, "GET", "/pairing"))
	for _, secret := range []string{"private-network-name", "private-log-path", e.dir, token, testLocalKey} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("unauthenticated shell contains %q", secret)
		}
	}
	for _, need := range []string{"sessionStorage.getItem", "X-CopyApp-Session", "history.replaceState", "URL.createObjectURL", "loadQR()"} {
		if !strings.Contains(w.Body.String(), need) {
			t.Fatalf("refresh shell lacks %s", need)
		}
	}
	// Restart and idle expiry both invalidate the browser credential.
	e2 := newEnv(t, 0)
	r = bareLocal(e2, "GET", "/local/state")
	r.Header.Set("X-CopyApp-Session", token)
	if got := e2.do(r); got.Code != http.StatusForbidden {
		t.Fatalf("token survived restart: %d", got.Code)
	}
	e.s.localMu.Lock()
	e.s.localSessions[token] = time.Now().Add(-localTTL - time.Second)
	e.s.localMu.Unlock()
	r = bareLocal(e, "GET", "/local/state")
	r.Header.Set("X-CopyApp-Session", token)
	if got := e.do(r); got.Code != http.StatusForbidden {
		t.Fatalf("expired token: %d", got.Code)
	}
}
