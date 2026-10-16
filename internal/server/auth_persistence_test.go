package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestPairPersistenceFailureDoesNotIssueCookie(t *testing.T) {
	e := newEnv(t, 0)
	path := filepath.Join(t.TempDir(), "session.json")
	if err := e.a.Persist(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	e.s.StartPairing()
	w := e.do(e.req("GET", "/pair?t="+e.s.pairToken, nil))
	if w.Code != http.StatusInternalServerError || len(w.Result().Cookies()) != 0 || e.a.Paired() {
		t.Fatalf("unsaved pairing accepted: status=%d cookies=%v paired=%v", w.Code, w.Result().Cookies(), e.a.Paired())
	}
}
