package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitize(t *testing.T) {
	for in, want := range map[string]string{
		`..\..\x.exe`: "x.exe", "a/b.txt": "b.txt", "": "file", "CON": "_CON",
		strings.Repeat("a", 500) + ".txt": strings.Repeat("a", 120)[:116] + ".txt",
		"nul.txt":                         "_nul.txt", "com1": "_com1", "LPT9.log": "_LPT9.log", "console.txt": "console.txt",
		"a\x00b\x07.txt": "ab.txt", "..": "file", "dir/": "file",
	} {
		if got := SanitizeName(in); got != want {
			t.Errorf("%q -> %q want %q", in, got, want)
		}
	}
}

func TestSaveNeverOverwrites(t *testing.T) {
	d := t.TempDir()
	p1, _ := SaveFile(d, "a.txt", strings.NewReader("1"), 10)
	p2, _ := SaveFile(d, "a.txt", strings.NewReader("2"), 10)
	if p1 == p2 {
		t.Fatal("overwrote")
	}
	if !strings.HasSuffix(p2, "a (1).txt") {
		t.Fatalf("unexpected name %q", p2)
	}
}

func TestSaveTooBigLeavesNothing(t *testing.T) {
	d := t.TempDir()
	_, err := SaveFile(d, "a.bin", strings.NewReader("12345678901"), 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if es, _ := os.ReadDir(d); len(es) != 0 {
		t.Fatal("partial file left")
	}
}

func TestAddGetListEvict(t *testing.T) {
	s := New()
	now := time.Now()
	s.now = func() time.Time { return now }
	id1, err := s.Add(Item{Kind: "text", Data: []byte("a")})
	if err != nil || id1 == "" {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	id2, _ := s.Add(Item{Kind: "text", Data: []byte("b")})
	l := s.List()
	if len(l) != 2 || l[0].ID != id2 {
		t.Fatalf("bad list %v", l)
	}
	if it, ok := s.Get(id1); !ok || string(it.Data) != "a" {
		t.Fatal("get failed")
	}
	now = now.Add(10 * time.Minute)
	if _, ok := s.Get(id1); ok {
		t.Fatal("expired item returned")
	}
	if len(s.List()) != 0 {
		t.Fatal("expired items listed")
	}
}

func TestAddTooBig(t *testing.T) {
	s := New()
	if _, err := s.Add(Item{Data: make([]byte, MaxItemSize+1)}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSanitizeStripsBidiAndInvisible(t *testing.T) {
	r := func(c rune) string { return string(c) }
	for in, want := range map[string]string{
		"invoice" + r(0x202E) + "txt.exe":                            "invoicetxt.exe",
		"a" + r(0x200B) + "b" + r(0x200F) + "c" + r(0xFEFF) + ".txt": "abc.txt",
		"x" + r(0x2066) + "y" + r(0x2069) + r(0x202A) + ".pdf":       "xy.pdf",
		r(0x200E) + r(0x200D):                                        "file",
		"CONIN$":                                                     "_CONIN$",
		"conout$.txt":                                                "_conout$.txt",
		"COM" + r(0xB9) + ".txt":                                     "_COM" + r(0xB9) + ".txt",
		"lpt" + r(0xB3):                                              "_lpt" + r(0xB3),
		"COM" + r(0xB2) + r(0xB3):                                    "COM" + r(0xB2) + r(0xB3),
		"COM0":                                                       "_COM0",
	} {
		if got := SanitizeName(in); got != want {
			t.Errorf("%q -> %q want %q", in, got, want)
		}
	}
}

func TestOutboxTotalCap(t *testing.T) {
	s := New()
	s.maxTotal = 10
	if _, err := s.Add(Item{Data: make([]byte, 6)}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Add(Item{Data: make([]byte, 5)})
	if !errors.Is(err, ErrTooLarge) || !errors.Is(err, ErrOutboxFull) {
		t.Fatalf("want outbox full, got %v", err)
	}
	if _, err := s.Add(Item{Data: make([]byte, 4)}); err != nil {
		t.Fatalf("fits exactly: %v", err)
	}
}

func TestOutboxCapFreedByExpiry(t *testing.T) {
	s := New()
	s.maxTotal = 10
	now := time.Now()
	s.now = func() time.Time { return now }
	s.Add(Item{Data: make([]byte, 10)})
	now = now.Add(Expiry + time.Second)
	if _, err := s.Add(Item{Data: make([]byte, 10)}); err != nil {
		t.Fatalf("expired items must free space: %v", err)
	}
}

func TestPathItemsDoNotCountAndKeepTheFile(t *testing.T) {
	s := New()
	s.maxTotal = 10
	now := time.Now()
	s.now = func() time.Time { return now }
	p := filepath.Join(t.TempDir(), "big.bin")
	os.WriteFile(p, make([]byte, 1000), 0o644)
	id, err := s.Add(Item{Kind: "file", Name: "big.bin", Path: p})
	if err != nil {
		t.Fatalf("path items are not in memory, the cap must not apply: %v", err)
	}
	if it, _ := s.Get(id); it.Size != 1000 || it.Data != nil {
		t.Fatalf("size %d", it.Size)
	}
	if _, err := s.Add(Item{Data: make([]byte, 10)}); err != nil {
		t.Fatalf("memory cap must ignore path items: %v", err)
	}
	if id2, _ := s.Add(Item{Data: []byte("abc")}); id2 != "" {
		if it, _ := s.Get(id2); it.Size != 3 {
			t.Fatal("size of memory item")
		}
	}
	now = now.Add(Expiry + time.Second)
	if _, ok := s.Get(id); ok {
		t.Fatal("not expired")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("expiry must not delete the user's file")
	}
	if _, err := s.Add(Item{Path: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("missing file accepted")
	}
}
