package secret

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalKeyRoundTrip(t *testing.T) {
	d := t.TempDir()
	k1, err := NewLocalKey(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 64 {
		t.Fatalf("want 32 random bytes as hex, got %q", k1)
	}
	got, err := ReadLocalKey(d)
	if err != nil || got != k1 {
		t.Fatalf("read back %q, %v", got, err)
	}
	k2, err := NewLocalKey(d)
	if err != nil {
		t.Fatal(err)
	}
	if k2 == k1 {
		t.Fatal("key must change on every launch")
	}
	if got, _ := ReadLocalKey(d); got != k2 {
		t.Fatal("old key still on disk")
	}
}

func TestReadLocalKeyRejectsGarbage(t *testing.T) {
	d := t.TempDir()
	if _, err := ReadLocalKey(d); err == nil {
		t.Fatal("missing file must fail")
	}
	os.WriteFile(filepath.Join(d, LocalKeyFile), []byte("short"), 0o600)
	if _, err := ReadLocalKey(d); err == nil {
		t.Fatal("short key must fail")
	}
}

func TestEqual(t *testing.T) {
	if Equal("", "") {
		t.Fatal("empty secrets never match")
	}
	if !Equal("abc", "abc") || Equal("abc", "abd") || Equal("abc", "ab") {
		t.Fatal("Equal broken")
	}
}
