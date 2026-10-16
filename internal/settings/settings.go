// Package settings holds the user's PC-side choices in
// %APPDATA%\CopyApp\settings.json.
package settings

import (
	"encoding/json"
	"fmt"
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
	if strings.TrimSpace(s.SaveDir) == "" || s.SaveDir != defSave && !ValidSaveDir(s.SaveDir) {
		s.SaveDir = defSave
	}
	if s.MaxBytes <= 0 {
		s.MaxBytes = DefaultMaxBytes
	}
}

// ValidSaveDir reports whether dir may hold files received from the phone:
// an absolute path on a drive letter (no network share, no \\?\ or \\.\
// device path, no alternate data stream), without reserved device names,
// and not inside Windows or Program Files. A hand-edited settings file or a
// picked folder that fails this falls back to the default folder.
func ValidSaveDir(dir string) bool {
	if !validLocalPath(dir) {
		return false
	}
	c, err := resolveDestination(dir)
	if err != nil || !validLocalPath(c) {
		return false
	}
	lc := strings.ToLower(c)
	var denied []string
	for _, env := range []string{"SystemRoot", "windir", "ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
		denied = append(denied, os.Getenv(env))
	}
	for _, env := range []string{"APPDATA", "ProgramData", "ALLUSERSPROFILE"} {
		if v := os.Getenv(env); v != "" {
			denied = append(denied, filepath.Join(v, `Microsoft\Windows\Start Menu`))
		}
	}
	denied = append(denied, ExtraDenied...)
	for _, d := range denied {
		if d == "" {
			continue
		}
		resolved, err := resolveDestination(d)
		if err != nil {
			resolved = filepath.Clean(d)
		}
		for _, path := range []string{filepath.Clean(d), resolved} {
			sys := strings.ToLower(path)
			if len(sys) > 3 && (lc == sys || strings.HasPrefix(lc, sys+`\`)) {
				return false
			}
		}
	}
	return true
}

func validLocalPath(dir string) bool {
	if dir == "" || strings.ContainsRune(dir, 0) || !filepath.IsAbs(dir) {
		return false
	}
	c := filepath.Clean(dir)
	vol := filepath.VolumeName(c)
	if len(vol) != 2 || vol[1] != ':' || !(vol[0] >= 'A' && vol[0] <= 'Z' || vol[0] >= 'a' && vol[0] <= 'z') {
		return false
	}
	rest := c[len(vol):]
	if strings.ContainsAny(rest, `:*?"<>|`) {
		return false
	}
	for _, part := range strings.Split(rest, `\`) {
		if reservedName(part) || part != strings.TrimRight(part, " .") {
			return false
		}
	}
	return true
}

// resolveDestination resolves the existing ancestor (including junctions,
// mapped drives and short names), then appends the not-yet-created suffix.
func resolveDestination(dir string) (string, error) {
	path := filepath.Clean(dir)
	var tail []string
	for {
		_, err := os.Lstat(path)
		if err == nil {
			resolved, err := canonicalExisting(path)
			if err != nil {
				return "", err
			}
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		tail = append(tail, filepath.Base(path))
		path = parent
	}
}

// ExtraDenied lists more folders ValidSaveDir refuses (and everything below
// them); the Windows build adds the known Startup and Start menu folders, in
// case they were moved away from their usual place.
var ExtraDenied []string

func reservedName(part string) bool {
	stem := strings.ToUpper(strings.TrimRight(part, " ."))
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	return len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9'
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
	if !ValidSaveDir(defaultSaveDir) {
		return nil, fmt.Errorf("settings: unsafe default receive folder %q", defaultSaveDir)
	}
	st := &Store{path: filepath.Join(dir, FileName), defSave: defaultSaveDir, s: Defaults(defaultSaveDir)}
	b, err := os.ReadFile(st.path)
	switch {
	case err == nil:
		var s Settings
		if json.Unmarshal(b, &s) == nil {
			if !ValidSize(s.MaxBytes) { // hand-edited: only the offered limits
				s.MaxBytes = DefaultMaxBytes
			}
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
