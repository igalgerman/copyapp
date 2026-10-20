package server

import (
	"encoding/json"
	"net/http"
	"strings"
"time"
"github.com/igalgerman/copyapp/internal/i18n"
)

// The native client reuses send's schema and clipboard implementation but its
// bearer credential cannot authorize browser routes or local PC controls.
func (s *Server) companionSend(w http.ResponseWriter, r *http.Request) {
	value := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if value == r.Header.Get("Authorization") || !s.cfg.Auth.ValidCompanion(value) {
		jsonError(w, http.StatusUnauthorized, "unauthorized", "Pair the companion again on your PC")
		return
	}
	s.cfg.Logf("companion: send accepted")
	s.send(w, r)
}

func (s *Server) companionPair(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct { Token string `json:"token"` }
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body) != nil {
		jsonError(w, http.StatusBadRequest, "invalid_request", "Invalid pairing request")
		return
	}
	s.mu.Lock()
	sid, ok, err := s.cfg.Auth.ExchangeCompanion(body.Token)
	if err == nil && ok {
		s.pairToken = ""
		s.pairedAt = time.Now()
		if a, valid := remoteAddr(r); valid { s.pairedFrom = a.String() }
	}
	s.mu.Unlock()
	if err != nil { jsonError(w, http.StatusInternalServerError, "session_save", "Cannot save pairing"); return }
	if !ok { jsonError(w, http.StatusUnauthorized, "expired_pairing", "Open a new QR code on the PC"); return }
	s.cfg.Logf("companion: paired")
	ip := "?"
	if a, valid := remoteAddr(r); valid { ip = a.String() }
	s.cfg.Clip.Notify("CopyApp", i18n.T("toast.pairedFrom", ip))
	if s.cfg.OnPaired != nil { s.cfg.OnPaired(ip) }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"credential": sid})
}
