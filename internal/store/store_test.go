package store

import (
	"os"
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
