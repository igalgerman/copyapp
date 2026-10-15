// Package store holds the short-lived outbox (PC -> phone) and safe file saving.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	// MaxItemSize is the per-item cap (200 MB).
	MaxItemSize = 200 << 20
	// Expiry is how long items stay in the outbox.
	Expiry = 10 * time.Minute

	maxNameLen = 120
)

// ErrTooLarge is returned when data exceeds the allowed size.
var ErrTooLarge = errors.New("store: item too large")

type Item struct {
	ID, Kind, Name, Mime string
	Data                 []byte
	Created              time.Time
} // Kind: "text"|"image"|"file"

type Store struct {
	mu    sync.Mutex
	items map[string]Item
	now   func() time.Time
}

func New() *Store {
	return &Store{items: map[string]Item{}, now: time.Now}
}

func (s *Store) evictLocked() {
	cut := s.now().Add(-Expiry)
	for id, it := range s.items {
		if !it.Created.After(cut) {
			delete(s.items, id)
		}
	}
}

// Add stores the item and returns its new ID. Rejects items over 200 MB and
// evicts items older than 10 minutes.
func (s *Store) Add(it Item) (string, error) {
	if len(it.Data) > MaxItemSize {
		return "", ErrTooLarge
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	it.ID = hex.EncodeToString(b[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	it.Created = s.now()
	s.evictLocked()
	s.items[it.ID] = it
	return it.ID, nil
}

func (s *Store) Get(id string) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	it, ok := s.items[id]
	return it, ok
}

// List returns items newest first.
func (s *Store) List() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

func isReserved(name string) bool {
	stem := name
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	stem = strings.ToUpper(strings.TrimRight(stem, " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) {
		return stem[3] >= '1' && stem[3] <= '9'
	}
	return false
}

// SanitizeName turns an untrusted client file name into a safe Windows file name.
func SanitizeName(name string) string {
	if i := strings.LastIndexAny(name, `\/`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"|?*`, r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, name)
	name = strings.TrimRight(name, ". ")
	name = strings.TrimLeft(name, " ")
	if name == "" {
		return "file"
	}
	if isReserved(name) {
		name = "_" + name
	}
	if len(name) > maxNameLen {
		ext := filepath.Ext(name)
		if len(ext) > 20 {
			ext = ""
		}
		stem := name[:maxNameLen-len(ext)]
		for !utf8.ValidString(stem) {
			stem = stem[:len(stem)-1]
		}
		name = stem + ext
	}
	return name
}

// SaveFile copies r into dir under a sanitized, unique name. It reads at most
// max bytes; on any error (including exceeding max) the partial file is removed.
func SaveFile(dir, name string, r io.Reader, max int64) (path string, err error) {
	name = SanitizeName(name)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 0; ; i++ {
		cand := name
		if i > 0 {
			cand = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		p := filepath.Join(dir, cand)
		f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		n, e := io.Copy(f, io.LimitReader(r, max+1))
		if ce := f.Close(); e == nil {
			e = ce
		}
		if e == nil && n > max {
			e = ErrTooLarge
		}
		if e != nil {
			os.Remove(p)
			return "", e
		}
		return p, nil
	}
}
