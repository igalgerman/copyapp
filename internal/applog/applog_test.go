package applog

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotates(t *testing.T) {
	p := filepath.Join(t.TempDir(), "copyapp.log")
	w, err := Open(p, 100)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Repeat("a", 39) + "\n" // 40 bytes
	for i := 0; i < 3; i++ {
		w.Write([]byte(line))
	}
	w.Write([]byte("last\n"))
	w.Close()
	cur, _ := os.ReadFile(p)
	old, err := os.ReadFile(p + ".1")
	if err != nil {
		t.Fatal("no rotated file")
	}
	if len(old) != 80 || string(cur) != line+"last\n" {
		t.Fatalf("old=%d cur=%q", len(old), cur)
	}
}

func TestAppendsAcrossOpens(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.log")
	w, _ := Open(p, 1<<20)
	w.Write([]byte("one\n"))
	w.Close()
	w, _ = Open(p, 1<<20)
	w.Write([]byte("two\n"))
	w.Close()
	if b, _ := os.ReadFile(p); string(b) != "one\ntwo\n" {
		t.Fatalf("%q", b)
	}
}

func TestControlCharactersCannotForgeLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	w, err := Open(path, DefaultMax)
	if err != nil {
		t.Fatal(err)
	}
	l := log.New(w, "", 0)
	l.Printf("GET %s 404", "/a\n2026/01/01 fake line\r\x1b[31m")
	w.Close()
	b, _ := os.ReadFile(path)
	if got := string(b); strings.Count(got, "\n") != 1 || !strings.HasSuffix(got, "404\n") || strings.ContainsAny(got, "\r\x1b") {
		t.Fatalf("%q", got)
	}
}
