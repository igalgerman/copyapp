// Package applog is a size-capped log file: when it would grow past the cap
// it is renamed to NAME.1 (replacing the previous one) and started anew.
// Callers must never log clipboard content, tokens or keys.
package applog

import (
	"os"
	"sync"
)

// DefaultMax is the size at which the log rotates (about 1 MB).
const DefaultMax = 1 << 20

type Writer struct {
	mu   sync.Mutex
	path string
	max  int64
	f    *os.File
	size int64
}

// Open appends to path.
func Open(path string, max int64) (*Writer, error) {
	w := &Writer{path: path, max: max}
	return w, w.open()
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f, w.size = f, fi.Size()
	return nil
}

// clean replaces control characters (newlines inside a record included) with
// '?', so a value logged from a request can never forge extra log lines.
// The final newline of the record is kept.
func clean(p []byte) []byte {
	n := len(p)
	if n > 0 && p[n-1] == '\n' {
		n--
	}
	var out []byte
	for i := 0; i < n; i++ {
		if c := p[i]; c < 0x20 && c != '\t' || c == 0x7f {
			if out == nil {
				out = append([]byte(nil), p...)
			}
			out[i] = '?'
		}
	}
	if out == nil {
		return p
	}
	return out
}

func (w *Writer) Write(p []byte) (int, error) {
	orig := len(p)
	p = clean(p)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f != nil && w.size > 0 && w.size+int64(len(p)) > w.max {
		w.f.Close()
		w.f = nil
		os.Remove(w.path + ".1")
		os.Rename(w.path, w.path+".1")
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.f == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	if err == nil {
		n = orig
	}
	return n, err
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
