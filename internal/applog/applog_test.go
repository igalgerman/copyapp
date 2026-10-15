package applog

import (
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
