package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCompanionPairAndScopedSend(t *testing.T) {
	e := newEnv(t, 0)
	browser, _ := e.a.Exchange(e.a.OpenPairing())
	qr := e.s.StartPairing()
	u, err := url.Parse(qr)
	if err != nil {
		t.Fatal(err)
	}
	fragment, _ := url.ParseQuery(u.Fragment)
	if want := strings.ReplaceAll(strings.ToLower(fingerprint(e.s.cfg.CAPEM)), ":", ""); fragment.Get("fp") != want || len(qr) > 200 {
		t.Fatalf("QR must carry only the short CA fingerprint: %q (%d bytes)", fragment.Get("fp"), len(qr))
	}
	request := httptest.NewRequest("POST", "https://192.168.1.5:8443/companion/pair", strings.NewReader(`{"token":"`+u.Query().Get("t")+`"}`))
	request.RemoteAddr = lanAddr
	response := httptest.NewRecorder()
	e.s.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("pair: %d %s", response.Code, response.Body.String())
	}
	var result struct {
		Credential string `json:"credential"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !e.a.Valid(browser) {
		t.Fatal("native pairing revoked browser")
	}
	for _, tc := range []struct {
		path, credential string
		want             int
	}{
		{"/companion/send", result.Credential, 200},
		{"/send", result.Credential, http.StatusUnauthorized},
		{"/companion/send", browser, http.StatusUnauthorized},
		{"/companion/send", "invalid", http.StatusUnauthorized},
	} {
		r := httptest.NewRequest("POST", "https://192.168.1.5:8443"+tc.path, strings.NewReader(`{"text":"demo","mode":"clipboard"}`))
		r.RemoteAddr = lanAddr
		r.Header.Set("Authorization", "Bearer "+tc.credential)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		e.s.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s status %d want %d: %s", tc.path, w.Code, tc.want, w.Body.String())
		}
	}
	if err := e.a.Revoke(); err != nil {
		t.Fatal(err)
	}
	if e.a.ValidCompanion(result.Credential) {
		t.Fatal("unpair did not revoke companion")
	}
}
