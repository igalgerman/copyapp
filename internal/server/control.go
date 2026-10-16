package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/igalgerman/copyapp/internal/i18n"
	"github.com/igalgerman/copyapp/internal/secret"
	"github.com/igalgerman/copyapp/internal/settings"
	"github.com/igalgerman/copyapp/internal/tlsutil"
)

// localPost guards the control window's state-changing routes against other
// web pages: the Origin must be this page, or (scripts, the e2e test) no
// Origin with the local secret in X-CopyApp, which browsers cannot send
// cross-site without a preflight that is never granted.
func localPost(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o := r.Header.Get("Origin")
		sameOrigin := o == "http://"+r.Host || o == "https://"+r.Host
		if !sameOrigin && (o != "" || r.Header.Get("X-CopyApp") == "") {
			jsonError(w, http.StatusForbidden, "forbidden", "cross-origin request")
			return
		}
		h(w, r)
	}
}

// wifiName caches the SSID lookup (netsh takes ~50 ms) for the 1 s poll.
func (s *Server) wifiName() string {
	if s.cfg.WiFiName == nil {
		return ""
	}
	s.wifiMu.Lock()
	defer s.wifiMu.Unlock()
	if time.Since(s.wifiTime) > 10*time.Second {
		s.wifi, s.wifiTime = s.cfg.WiFiName(), time.Now()
	}
	return s.wifi
}

type phoneState struct {
	Paired    bool       `json:"paired"`
	Connected bool       `json:"connected"` // an event stream is open right now
	LastSeen  *time.Time `json:"lastSeen"`  // last authenticated request (this run)
	// Where and when the current phone paired (this run; empty after a restart).
	PairedFrom string     `json:"pairedFrom,omitempty"`
	PairedAt   *time.Time `json:"pairedAt,omitempty"`
}

type localState struct {
	Pairing          pairState         `json:"pairing"`
	Phone            phoneState        `json:"phone"`
	WiFi             string            `json:"wifi"`
	Address          string            `json:"address"`
	Settings         settings.Settings `json:"settings"`
	Autostart        bool              `json:"autostart"`
	Transfers        []Transfer        `json:"transfers"`
	SizeChoices      []int64           `json:"sizeChoices"`
	LogPath          string            `json:"logPath"`
	Hotkey           string            `json:"hotkey"`
	Version          string            `json:"version"`
	Fingerprint      string            `json:"fingerprint"`
	ShortFingerprint string            `json:"shortFingerprint"`
}

func (s *Server) phone() phoneState {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := phoneState{Paired: s.cfg.Auth.Paired(), Connected: len(s.clients) > 0}
	if !s.lastSeen.IsZero() && p.Paired {
		t := s.lastSeen
		p.LastSeen = &t
	}
	if p.Paired && s.pairedFrom != "" {
		t := s.pairedAt
		p.PairedFrom, p.PairedAt = s.pairedFrom, &t
	}
	return p
}

func (s *Server) snapshot() localState {
	st := localState{
		Pairing:          s.state(),
		Phone:            s.phone(),
		WiFi:             s.wifiName(),
		Address:          s.cfg.BaseURL(),
		Settings:         s.cfg.Settings.Get(),
		Transfers:        s.Transfers(),
		SizeChoices:      settings.SizeChoices,
		LogPath:          s.cfg.LogPath,
		Hotkey:           "Ctrl+Alt+C",
		Version:          s.cfg.Version,
		Fingerprint:      fingerprint(s.cfg.CAPEM),
		ShortFingerprint: tlsutil.ShortFingerprint(s.cfg.CAPEM),
	}
	if s.cfg.Autostart != nil {
		st.Autostart = s.cfg.Autostart()
	}
	if s.cfg.Hotkey != nil {
		st.Hotkey = s.cfg.Hotkey()
	}
	return st
}

func (s *Server) localState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.snapshot())
}

func (s *Server) changeSettings(fn func(*settings.Settings)) error {
	old := s.cfg.Settings.Get()
	cur, err := s.cfg.Settings.Update(fn)
	if cur.Language != old.Language {
		i18n.Set(cur.Language)
	}
	if s.cfg.OnSettings != nil {
		s.cfg.OnSettings(old, cur)
	}
	if err != nil {
		s.cfg.Logf("settings: save: %v", err)
	}
	return err
}

// localSettings applies a partial change:
// {"language","receiveMode","maxBytes","autostart","saveDir"} (saveDir ""
// resets to Downloads\CopyApp).
func (s *Server) localSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Language    *string `json:"language"`
		ReceiveMode *string `json:"receiveMode"`
		MaxBytes    *int64  `json:"maxBytes"`
		Autostart   *bool   `json:"autostart"`
		SaveDir     *string `json:"saveDir"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	switch {
	case req.Language != nil && !i18n.Valid(*req.Language),
		req.ReceiveMode != nil && *req.ReceiveMode != settings.ModeClipboard && *req.ReceiveMode != settings.ModeFolder,
		req.MaxBytes != nil && !settings.ValidSize(*req.MaxBytes),
		req.SaveDir != nil && *req.SaveDir != "" && !settings.ValidSaveDir(*req.SaveDir):
		jsonError(w, http.StatusBadRequest, "bad_value", "invalid setting")
		return
	}
	if req.SaveDir != nil && *req.SaveDir != "" {
		if err := os.MkdirAll(*req.SaveDir, 0o700); err != nil {
			jsonError(w, http.StatusBadRequest, "bad_folder", "cannot use that folder")
			return
		}
	}
	if req.Autostart != nil {
		if s.cfg.SetAutostart == nil {
			jsonError(w, http.StatusNotImplemented, "unsupported", "not available")
			return
		}
		if err := s.cfg.SetAutostart(*req.Autostart); err != nil {
			s.cfg.Logf("autostart: %v", err)
			jsonError(w, http.StatusInternalServerError, "autostart", "could not change Start with Windows")
			return
		}
	}
	err := s.changeSettings(func(st *settings.Settings) {
		if req.Language != nil {
			st.Language = *req.Language
		}
		if req.ReceiveMode != nil {
			st.ReceiveMode = *req.ReceiveMode
		}
		if req.MaxBytes != nil {
			st.MaxBytes = *req.MaxBytes
		}
		if req.SaveDir != nil {
			st.SaveDir = *req.SaveDir // "" is reset by normalize
		}
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "settings", "changed for this run, but could not save settings")
		return
	}
	writeJSON(w, http.StatusOK, s.snapshot())
}

// localChooseFolder opens the PC's folder picker (blocks until it closes).
func (s *Server) localChooseFolder(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ChooseFolder == nil {
		jsonError(w, http.StatusNotImplemented, "unsupported", "not available")
		return
	}
	cur := s.cfg.Settings.Get().SaveDir
	dir, err := s.cfg.ChooseFolder(cur)
	if err != nil {
		s.cfg.Logf("choose folder: %v", err)
		jsonError(w, http.StatusInternalServerError, "dialog", "could not open the folder picker")
		return
	}
	if dir != "" && !settings.ValidSaveDir(dir) {
		s.cfg.Logf("choose folder: refused a network, device or system folder")
		jsonError(w, http.StatusBadRequest, "bad_folder", "choose a folder on this PC, outside Windows and Program Files")
		return
	}
	changed := dir != "" && dir != cur
	if changed {
		if err := s.changeSettings(func(st *settings.Settings) { st.SaveDir = dir }); err != nil {
			jsonError(w, http.StatusInternalServerError, "settings", "folder changed for this run, but could not save settings")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"changed": changed, "saveDir": s.cfg.Settings.Get().SaveDir})
}

func (s *Server) localOpenFolder(w http.ResponseWriter, r *http.Request) {
	if s.cfg.OpenFolder == nil {
		jsonError(w, http.StatusNotImplemented, "unsupported", "not available")
		return
	}
	if err := s.cfg.OpenFolder(s.cfg.Settings.Get().SaveDir); err != nil {
		s.cfg.Logf("open folder: %v", err)
		jsonError(w, http.StatusInternalServerError, "open", "could not open the folder")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// localRepair forgets the phone and opens a new pairing window.
func (s *Server) localRepair(w http.ResponseWriter, r *http.Request) {
	if err := s.Unpair(); err != nil {
		s.cfg.Logf("unpair: could not persist revocation: %v", err)
		jsonError(w, http.StatusInternalServerError, "session_save", "unpaired for this run, but could not save revocation; the old phone may regain access after restart")
		return
	}
	s.StartPairing()
	w.WriteHeader(http.StatusNoContent)
}

// localSendClipboard sends the PC clipboard to the phone, like the hotkey
// and the tray item. Reply: {"result":"sent"|"empty"|"secret"|"failed"}.
func (s *Server) localSendClipboard(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SendClipboard == nil {
		jsonError(w, http.StatusNotImplemented, "unsupported", "not available")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": s.cfg.SendClipboard("window")})
}

// localUninstall starts the PC's uninstall flow (native confirmation first).
func (s *Server) localUninstall(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Uninstall == nil {
		jsonError(w, http.StatusNotImplemented, "unsupported", "not available")
		return
	}
	go s.cfg.Uninstall()
	w.WriteHeader(http.StatusAccepted)
}

// LocalAllowed reports whether r may use a PC-only route: loopback peer,
// localhost Host, no Origin and the local secret in X-CopyApp. Used by the
// exe's own loopback routes (second instance, upgrade).
func LocalAllowed(r *http.Request, key string) bool {
	a, ok := remoteAddr(r)
	return ok && a.IsLoopback() && isLocalHost(r.Host) && r.Header.Get("Origin") == "" &&
		secret.Equal(r.Header.Get("X-CopyApp"), key)
}
