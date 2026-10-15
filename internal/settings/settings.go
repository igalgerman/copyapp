// Package settings holds the user's PC-side choices in
// %APPDATA%\CopyApp\settings.json.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/igalgerman/copyapp/internal/i18n"
	"github.com/igalgerman/copyapp/internal/secret"
)

// FileName is the settings file in the data directory.
const FileName = "settings.json"

// Receive modes: what the PC does with items from the phone when the phone
// does not ask for something specific.
const (
	ModeClipboard = "clipboard" // text and PNG to the clipboard, other files saved
	ModeFolder    = "folder"    // everything saved to SaveDir
)

// DefaultMaxBytes is the default per-item limit (2 GiB).
const DefaultMaxBytes int64 = 2 << 30

// SizeChoices are the limits offered in the control window.
var SizeChoices = []int64{200 << 20, 500 << 20, 1 << 30, 2 << 30}

// ValidSize reports whether v is one of SizeChoices.
func ValidSize(v int64) bool {
	for _, c := range SizeChoices {
		if c == v {
			return true
		}
	}
	return false
}

type Settings struct {
	Language          string `json:"language"`
	ReceiveMode       string `json:"receiveMode"`
	SaveDir           string `json:"saveDir"`
	MaxBytes          int64  `json:"maxBytes"`
	AutostartAsked    bool   `json:"autostartAsked"`
	HotkeyNoticeShown bool   `json:"hotkeyNoticeShown,omitempty"`
}

// Defaults returns the settings of a fresh install.
func Defaults(saveDir string) Settings {
	return Settings{Language: i18n.Default, ReceiveMode: ModeClipboard, SaveDir: saveDir, MaxBytes: DefaultMaxBytes}
}

func (s *Settings) normalize(defSave string) {
	if !i18n.Valid(s.Language) {
		s.Language = i18n.Default
	}
	if s.ReceiveMode != ModeFolder {
		s.ReceiveMode = ModeClipboard
	}
	if strings.TrimSpace(s.SaveDir) == "" {
		s.SaveDir = defSave
	}
	if s.MaxBytes <= 0 {
		s.MaxBytes = DefaultMaxBytes
	}
}

// Store is a thread-safe settings holder that saves on every Update.
type Store struct {
	mu      sync.Mutex
	path    string // "" = memory only
	defSave string
	s       Settings
}

// Load reads dir\settings.json. A missing or corrupt file gives defaults
// (the language is taken over from the older lang.txt). Nothing is written
// until Update.
func Load(dir, defaultSaveDir string) (*Store, error) {
	st := &Store{path: filepath.Join(dir, FileName), defSave: defaultSaveDir, s: Defaults(defaultSaveDir)}
	b, err := os.ReadFile(st.path)
	switch {
	case err == nil:
		var s Settings
		if json.Unmarshal(b, &s) == nil {
			st.s = s
		}
	case os.IsNotExist(err):
		if lb, err := os.ReadFile(filepath.Join(dir, i18n.FileName)); err == nil {
			st.s.Language = strings.TrimSpace(string(lb))
		}
	default:
		return nil, err
	}
	st.s.normalize(defaultSaveDir)
	return st, nil
}

// NewMemory returns a store that never touches the disk (tests, fallbacks).
func NewMemory(s Settings) *Store {
	st := &Store{defSave: s.SaveDir, s: s}
	st.s.normalize(s.SaveDir)
	return st
}

// Get returns a copy of the current settings.
func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

// DefaultSaveDir is the folder used when SaveDir is reset.
func (st *Store) DefaultSaveDir() string { return st.defSave }

// Update applies fn, repairs invalid values and saves. On a write error the
// change still applies for this run.
func (st *Store) Update(fn func(*Settings)) (Settings, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	s := st.s
	fn(&s)
	s.normalize(st.defSave)
	st.s = s
	if st.path == "" {
		return s, nil
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return s, secret.WriteOwnerOnly(st.path, b)
}
