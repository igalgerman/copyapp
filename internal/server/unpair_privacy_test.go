package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/igalgerman/copyapp/internal/store"
)

func TestUnpairDoesNotShareOldOutboxWithNextPhone(t *testing.T) {
	e := newEnv(t, 0)
	old := e.pair()
	id, err := e.s.Publish(store.Item{Kind: "text", Data: []byte("private clipboard"), Mime: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "original.txt")
	if err := os.WriteFile(p, []byte("user original"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileID, err := e.s.Publish(store.Item{Kind: "file", Path: p, Name: "original.txt"})
	if err != nil {
		t.Fatal(err)
	}
	e.s.Unpair()
	if got := e.st.List(); len(got) != 0 {
		t.Fatalf("old transfers survived unpair: %d", len(got))
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "user original" {
		t.Fatalf("original changed: %q %v", b, err)
	}
	next := e.pair()
	for _, itemID := range []string{id, fileID} {
		if w := e.do(e.authed("GET", "/item/"+itemID, nil, next)); w.Code != http.StatusNotFound {
			t.Fatalf("new phone sees old item: %d", w.Code)
		}
	}
	if w := e.do(e.authed("GET", "/status", nil, old)); w.Code != http.StatusUnauthorized {
		t.Fatalf("old session still valid: %d", w.Code)
	}
}

func TestReplacementPairingClearsPreviousPhoneOutbox(t *testing.T) {
	e := newEnv(t, 0)
	e.pair()
	if _, err := e.s.Publish(store.Item{Kind: "text", Data: []byte("previous phone only")}); err != nil {
		t.Fatal(err)
	}
	// Opening Pair phone and scanning again also replaces the session, even
	// when the user did not explicitly choose Unpair first.
	e.pair()
	if len(e.st.List()) != 0 {
		t.Fatal("replacement phone inherited old clipboard history")
	}
}
