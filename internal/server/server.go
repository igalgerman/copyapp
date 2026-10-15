// Package server is the LAN HTTPS API and PWA host.
//
// Routes (all subject to the private-source-IP filter):
//
//	GET  /, static PWA files     public
//	GET  /pair?t=TOKEN           exchange pairing token -> session cookie, 303 to /
//	GET  /ca.crt                 CA certificate; while pairing is open, or with a session
//	GET  /status                 session check: {"paired":true,"fingerprint":"..."}
//	POST /send                   phone -> PC (JSON {"text":...} or multipart)
//	POST /share                  same as /send; Android share target (sw.js normally
//	                             intercepts it, see web/sw.js)
//	GET  /events                 SSE stream of outbox items (PC -> phone)
//	GET  /item/{id}              outbox item bytes
//	POST /unpair                 drop the session
//	GET  /pairing, /qr, /pairing/status, POST /pairing/open
//	GET  /local/state, POST /local/{settings,choose-folder,open-folder,repair,uninstall,send-clipboard}
//	                             PC-only control window (loopback source, localhost
//	                             Host and the per-launch local secret; see
//	                             Config.LocalSecret)
package server

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/igalgerman/copyapp/internal/auth"
	"github.com/igalgerman/copyapp/internal/i18n"
	"github.com/igalgerman/copyapp/internal/secret"
	"github.com/igalgerman/copyapp/internal/settings"
	"github.com/igalgerman/copyapp/internal/store"
	"github.com/igalgerman/copyapp/internal/tlsutil"
)

//go:embed web
var webFS embed.FS

// CookieName is the session cookie.
const CookieName = "copyapp_session"

// multipartSlack is the allowance over MaxBytes for multipart framing and
// small text fields.
const multipartSlack = 1 << 20

// Clipboard is the PC side the server drives. internal/pc implements it.
type Clipboard interface {
	SetText(string) error
	SetImagePNG([]byte) error // must leave the clipboard unchanged on error
	Notify(title, msg string)
}

type Config struct {
	Auth  *auth.Auth
	Store *store.Store
	Clip  Clipboard
	// Settings holds the receive mode, save folder and size limit. Nil: an
	// in-memory store built from SaveDir and MaxBytes below.
	Settings *settings.Store
	SaveDir  string // used only when Settings is nil
	MaxBytes int64  // used only when Settings is nil; 0 = settings.DefaultMaxBytes
	CAPEM    []byte
	// BaseURL returns the phone-facing origin, e.g. "https://192.168.1.5:8443".
	// It is called each time a pairing URL is built.
	BaseURL func() string
	// Logf receives one line per request (method, path without query, status)
	// and errors. Never secrets. Optional.
	Logf func(format string, args ...any)
	// LocalSecret authorizes the PC-only routes. Loopback is shared by every
	// Windows user on the machine, so peer address alone is not enough: a
	// request must carry it in the X-CopyApp header, or a browser session
	// cookie (LocalCookieName: a random in-memory token, issued once
	// /pairing?k=SECRET was opened). Empty denies those routes.
	LocalSecret string
	// WiFiName returns the SSID this PC is connected to ("" if unknown or
	// wired); the pairing page names it so the phone joins the same network.
	// Optional.
	WiFiName func() string

	// PC-side hooks for the control window (/pairing). All optional.
	ChooseFolder func(current string) (string, error) // "" = cancelled
	OpenFolder   func(path string) error
	Uninstall    func()
	OnSettings   func(old, cur settings.Settings) // after a change from the control window
	Autostart    func() bool
	SetAutostart func(on bool) error
	LogPath      string
	Hotkey       func() string // the active "send clipboard" shortcut
	// SendClipboard sends the PC clipboard to the phone (control window
	// button); source names the trigger for the log. It returns "sent",
	// "empty", "secret" or "failed".
	SendClipboard func(source string) string
	Version       string
	// OnPaired runs after a phone paired (tray flash). Optional.
	OnPaired func(ip string)
}

// LocalCookieName holds the control window's browser session token.
const LocalCookieName = "copyapp_local"

type client struct {
	ch   chan []byte
	quit chan struct{}
	seq  uint64 // subscription order
}

// maxClients caps open event streams; the oldest is closed to make room
// (a phone that reloads leaves dead streams behind until they time out).
const maxClients = 8

// maxUploads caps concurrent uploads from the phone.
const maxUploads = 2

// Server is an http.Handler.
type Server struct {
	cfg     Config
	handler http.Handler

	mu         sync.Mutex
	clients    map[*client]struct{}
	clientSeq  uint64
	pairToken  string
	openedAt   time.Time
	pairedAt   time.Time
	pairedFrom string // phone address at pairing (this run)
	lastSeen   time.Time

	transfers transfers
	uploads   atomic.Int32

	localMu       sync.Mutex
	localSessions map[string]time.Time // control-window cookie tokens -> last use

	wifiMu   sync.Mutex
	wifi     string
	wifiTime time.Time
}

func New(cfg Config) *Server {
	if cfg.Settings == nil {
		cfg.Settings = settings.NewMemory(settings.Settings{SaveDir: cfg.SaveDir, MaxBytes: cfg.MaxBytes})
	}
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	if cfg.BaseURL == nil {
		cfg.BaseURL = func() string { return "https://localhost" }
	}
	s := &Server{cfg: cfg, clients: map[*client]struct{}{}, localSessions: map[string]time.Time{}}

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", staticHandler(static))
	mux.HandleFunc("GET /pair", s.pair)
	mux.HandleFunc("GET /ca.crt", s.caCert)
	mux.HandleFunc("GET /status", s.requireAuth(s.status))
	mux.HandleFunc("POST /send", sameSite(s.requireAuth(s.send)))
	mux.HandleFunc("POST /share", sameSite(s.requireAuth(s.send))) // share target fallback when sw.js is absent
	mux.HandleFunc("GET /events", s.requireAuth(s.events))
	mux.HandleFunc("GET /item/{id}", s.requireAuth(s.item))
	mux.HandleFunc("POST /unpair", sameSite(s.requireAuth(s.unpair)))
	mux.HandleFunc("GET /pairing", s.localOnly(s.pairingPage))
	mux.HandleFunc("GET /pairing/status", s.localOnly(s.pairingStatus))
	mux.HandleFunc("POST /pairing/open", s.localOnly(s.pairingOpen))
	mux.HandleFunc("GET /qr", s.localOnly(s.qr))
	mux.HandleFunc("GET /local/state", s.localOnly(s.localState))
	mux.HandleFunc("POST /local/settings", s.localOnly(localPost(s.localSettings)))
	mux.HandleFunc("POST /local/choose-folder", s.localOnly(localPost(s.localChooseFolder)))
	mux.HandleFunc("POST /local/open-folder", s.localOnly(localPost(s.localOpenFolder)))
	mux.HandleFunc("POST /local/repair", s.localOnly(localPost(s.localRepair)))
	mux.HandleFunc("POST /local/uninstall", s.localOnly(localPost(s.localUninstall)))
	mux.HandleFunc("POST /local/send-clipboard", s.localOnly(localPost(s.localSendClipboard)))
	s.handler = s.logged(securityHeaders(privateOnly(mux)))
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

// StartPairing opens a 10-minute pairing window and returns the URL to put in
// the QR code. Use this (not Auth.OpenPairing) so /qr shows the right token.
func (s *Server) StartPairing() string {
	tok := s.cfg.Auth.OpenPairing()
	s.mu.Lock()
	s.pairToken = tok
	s.openedAt = time.Now()
	s.mu.Unlock()
	return s.pairURL(tok)
}

func (s *Server) pairURL(tok string) string {
	return strings.TrimRight(s.cfg.BaseURL(), "/") + "/pair?t=" + tok
}

// Publish adds an item to the outbox and pushes it to connected phones.
func (s *Server) Publish(it store.Item) (string, error) {
	size := int64(len(it.Data))
	if it.Path != "" {
		fi, err := os.Stat(it.Path)
		if err != nil {
			return "", err
		}
		if !fi.Mode().IsRegular() {
			return "", fmt.Errorf("only regular files can be sent")
		}
		size = fi.Size()
	}
	if size > s.cfg.Settings.Get().MaxBytes {
		return "", store.ErrTooLarge
	}
	id, err := s.cfg.Store.Add(it)
	if err != nil {
		return "", err
	}
	if it, ok := s.cfg.Store.Get(id); ok {
		s.Broadcast(it)
	}
	return id, nil
}

// Broadcast pushes an item (already in the store) to all SSE clients. Slow
// clients are disconnected; they reconnect and receive the full list.
func (s *Server) Broadcast(it store.Item) {
	msg := sseMessage(it)
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		select {
		case c.ch <- msg:
		default:
			close(c.quit)
			delete(s.clients, c)
		}
	}
}

// Unpair drops the session and pairing token and closes all event streams
// (tray "Re-pair").
func (s *Server) Unpair() {
	s.cfg.Auth.Revoke()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pairToken, s.pairedFrom = "", ""
	for c := range s.clients {
		close(c.quit)
		delete(s.clients, c)
	}
}

// ---------- middleware ----------

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(c int) {
	if w.code == 0 {
		w.code = c
	}
	w.ResponseWriter.WriteHeader(c)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// logged logs method, path (never the query, which may hold a token) and status.
func (s *Server) logged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		s.cfg.Logf("%s %s %d", r.Method, r.URL.Path, sw.code)
	})
}

const shellCSP = "default-src 'self'; img-src 'self' blob: data:; media-src 'self' blob:; " +
	"style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; " +
	"object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", shellCSP)
		next.ServeHTTP(w, r)
	})
}

func remoteAddr(r *http.Request) (netip.Addr, bool) {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, false
	}
	return ap.Addr().Unmap(), true
}

// AllowedSource reports whether a peer address is private, loopback or
// link-local (RFC 1918, fc00::/7, 127/8, ::1, 169.254/16, fe80::/10).
func AllowedSource(a netip.Addr) bool {
	a = a.Unmap()
	return a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast()
}

func privateOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a, ok := remoteAddr(r); !ok || !AllowedSource(a) {
			jsonError(w, http.StatusForbidden, "forbidden", "requests are only accepted from the local network")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isNavigate(r *http.Request) bool { return r.Header.Get("Sec-Fetch-Mode") == "navigate" }

func (s *Server) authed(r *http.Request) bool {
	c, err := r.Cookie(CookieName)
	return err == nil && s.cfg.Auth.Valid(c.Value)
}

func (s *Server) requireAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authed(r) {
			if isNavigate(r) && r.Method == http.MethodPost {
				http.Redirect(w, r, "/?error=unauthorized", http.StatusSeeOther)
				return
			}
			jsonError(w, http.StatusUnauthorized, "unauthorized", "not paired: scan the QR code on your PC")
			return
		}
		s.mu.Lock()
		s.lastSeen = time.Now()
		s.mu.Unlock()
		h(w, r)
	}
}

// sameSite rejects state-changing requests that browsers mark as cross-site
// (defense in depth on top of the SameSite=Strict cookie). Clients that send
// neither header (scripts, old browsers) and opaque "null" origins pass; the
// cookie still has to be valid.
func sameSite(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o := r.Header.Get("Origin")
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" || (o != "" && o != "null" && o != "https://"+r.Host) {
			jsonError(w, http.StatusForbidden, "forbidden", "cross-site request")
			return
		}
		h(w, r)
	}
}

func isLocalHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// LocalAuthorized reports whether r carries the local secret in the
// X-CopyApp header (scripts and the second-instance launcher; browsers cannot
// send it cross-site).
func LocalAuthorized(r *http.Request, key string) bool {
	return secret.Equal(r.Header.Get("X-CopyApp"), key)
}

// localTTL is the sliding lifetime of a control-window browser session.
const localTTL = 12 * time.Hour

// newLocalSession issues a random token for the control window's cookie.
// The long-lived local key itself never goes into a cookie: 127.0.0.1 cookies
// are sent to every port, so another local program could otherwise collect
// it. Tokens live in memory only (a restart logs every browser out).
func (s *Server) newLocalSession() string {
	var b [32]byte
	rand.Read(b[:])
	tok := base64.RawURLEncoding.EncodeToString(b[:])
	s.localMu.Lock()
	defer s.localMu.Unlock()
	now := time.Now()
	for t, seen := range s.localSessions {
		if now.Sub(seen) > localTTL {
			delete(s.localSessions, t)
		}
	}
	if len(s.localSessions) >= 16 { // a handful of browsers at most
		for t := range s.localSessions {
			delete(s.localSessions, t)
			break
		}
	}
	s.localSessions[tok] = now
	return tok
}

// localSessionValid checks and extends a control-window session.
func (s *Server) localSessionValid(r *http.Request) bool {
	c, err := r.Cookie(LocalCookieName)
	if err != nil || c.Value == "" {
		return false
	}
	s.localMu.Lock()
	defer s.localMu.Unlock()
	for tok, seen := range s.localSessions {
		if secret.Equal(c.Value, tok) {
			if time.Since(seen) > localTTL {
				delete(s.localSessions, tok)
				return false
			}
			s.localSessions[tok] = time.Now()
			return true
		}
	}
	return false
}

// localOnly allows only loopback peers that address us as localhost (the
// Host check blocks DNS-rebinding pages) and hold the local secret (blocks
// other users of this PC). GET /pairing?k=SECRET trades the secret for the
// cookie.
func (s *Server) localOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := remoteAddr(r)
		if !ok || !a.IsLoopback() || !isLocalHost(r.Host) {
			jsonError(w, http.StatusForbidden, "forbidden", "only available on this PC")
			return
		}
		if k := r.URL.Query().Get("k"); k != "" && r.Method == http.MethodGet && r.URL.Path == "/pairing" {
			if !secret.Equal(k, s.cfg.LocalSecret) {
				jsonError(w, http.StatusForbidden, "forbidden", "open this page from the CopyApp tray menu")
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name: LocalCookieName, Value: s.newLocalSession(), Path: "/",
				HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode,
			})
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/pairing", http.StatusSeeOther) // drop the key from the address bar
			return
		}
		if s.cfg.LocalSecret == "" || !s.localSessionValid(r) && !LocalAuthorized(r, s.cfg.LocalSecret) {
			jsonError(w, http.StatusForbidden, "forbidden", "open this page from the CopyApp tray menu")
			return
		}
		h(w, r)
	}
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]string{"error": msg, "code": errCode})
}

// ---------- static ----------

func staticHandler(fsys fs.FS) http.Handler {
	// Windows registry MIME mappings may lack this; never rely on them.
	mime.AddExtensionType(".woff2", "font/woff2")
	favicon, _ := fs.ReadFile(fsys, "icon-192.png")
	fileServer := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p != "/" && strings.HasSuffix(p, "/") { // no directory listings
			http.NotFound(w, r)
			return
		}
		switch {
		case p == "/favicon.ico":
			if len(favicon) == 0 {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			w.Write(favicon)
			return
		case p == "/" || p == "/index.html" || p == "/sw.js":
			w.Header().Set("Cache-Control", "no-cache")
		case strings.HasSuffix(p, ".webmanifest") || p == "/manifest.json":
			w.Header().Set("Content-Type", "application/manifest+json")
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}

// ---------- pairing ----------

func (s *Server) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: CookieName, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	}
}

// pairFailHTML is shown on the phone, whose language the PC does not know:
// the PC's language first, then the other two.
func pairFailHTML() string {
	var b strings.Builder
	b.WriteString(`<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>CopyApp</title><body style="font:16px system-ui;margin:2rem;max-width:30rem">`)
	langs := []string{i18n.Lang()}
	for _, l := range i18n.Langs {
		if l != langs[0] {
			langs = append(langs, l)
		}
	}
	for _, l := range langs {
		fmt.Fprintf(&b, `<section lang="%s" dir="%s" style="margin-bottom:2rem"><h1>%s</h1><p>%s</p></section>`,
			l, i18n.Dir(l), i18n.In(l, "fail.title"), i18n.In(l, "fail.text")) // our own strings
	}
	b.WriteString(`</body>`)
	return b.String()
}

func (s *Server) pair(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead { // link previews must not burn the token
		return
	}
	sid, ok := s.cfg.Auth.Exchange(r.URL.Query().Get("t"))
	if !ok {
		s.cfg.Logf("pair: rejected token from %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, pairFailHTML())
		return
	}
	ip := "?"
	if a, ok := remoteAddr(r); ok {
		ip = a.String()
	}
	s.mu.Lock()
	s.pairToken = ""
	s.pairedAt = time.Now()
	s.pairedFrom = ip
	s.mu.Unlock()
	http.SetCookie(w, s.cookie(sid, int(auth.SessionTTL/time.Second)))
	// Always visible on the PC, so a pairing the user did not make stands out.
	s.cfg.Logf("pair: phone paired from %s", ip)
	s.cfg.Clip.Notify("CopyApp", i18n.T("toast.pairedFrom", ip))
	if s.cfg.OnPaired != nil {
		s.cfg.OnPaired(ip)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) caCert(w http.ResponseWriter, r *http.Request) {
	if len(s.cfg.CAPEM) == 0 || !(s.cfg.Auth.PairingOpen() || s.authed(r)) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="copyapp-ca.crt"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(s.cfg.CAPEM)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	// The session slides: renew the cookie's lifetime with the server's.
	if c, err := r.Cookie(CookieName); err == nil {
		http.SetCookie(w, s.cookie(c.Value, int(auth.SessionTTL/time.Second)))
	}
	set := s.cfg.Settings.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"paired": true, "fingerprint": fingerprint(s.cfg.CAPEM), "shortFingerprint": tlsutil.ShortFingerprint(s.cfg.CAPEM),
		// What the PC does with items sent with mode=auto, and the per-item limit.
		"receiveMode": set.ReceiveMode, "maxBytes": set.MaxBytes,
	})
}

func (s *Server) unpair(w http.ResponseWriter, r *http.Request) {
	s.Unpair()
	http.SetCookie(w, s.cookie("", -1))
	s.cfg.Clip.Notify("CopyApp", i18n.T("toast.unpaired"))
	w.WriteHeader(http.StatusNoContent)
}

// ---------- outbox (PC -> phone) ----------

const inlineTextMax = 256 << 10

type itemJSON struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"`
	Name    string    `json:"name,omitempty"`
	Mime    string    `json:"mime,omitempty"`
	Size    int64     `json:"size"`
	Created time.Time `json:"created"`
	Text    string    `json:"text,omitempty"` // text items up to 256 KB
}

func sseMessage(it store.Item) []byte {
	j := itemJSON{ID: it.ID, Kind: it.Kind, Name: it.Name, Mime: it.Mime, Size: it.Size, Created: it.Created}
	if it.Kind == "text" && len(it.Data) <= inlineTextMax {
		j.Text = string(it.Data)
	}
	b, _ := json.Marshal(j)
	return []byte(fmt.Sprintf("id: %s\nevent: item\ndata: %s\n\n", it.ID, b))
}

func (s *Server) subscribe() *client {
	s.mu.Lock()
	s.clientSeq++
	c := &client{ch: make(chan []byte, 16), quit: make(chan struct{}), seq: s.clientSeq}
	for len(s.clients) >= maxClients {
		var oldest *client
		for x := range s.clients {
			if oldest == nil || x.seq < oldest.seq {
				oldest = x
			}
		}
		close(oldest.quit)
		delete(s.clients, oldest)
	}
	s.clients[c] = struct{}{}
	s.mu.Unlock()
	return c
}

func (s *Server) unsubscribe(c *client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clients[c]; ok {
		delete(s.clients, c)
		close(c.quit)
	}
}

// heartbeat is how often /events pings and re-checks the session.
var heartbeat = 15 * time.Second

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")

	c := s.subscribe() // before listing, so nothing published in between is lost
	defer s.unsubscribe(c)
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "retry: 3000\n\n")
	items := s.cfg.Store.List()
	for i := len(items) - 1; i >= 0; i-- { // oldest first
		w.Write(sseMessage(items[i]))
	}
	// Let the phone distinguish the initial history from new transfers, so
	// opening the app does not automatically download the same files again.
	io.WriteString(w, "event: ready\ndata: {}\n\n")
	if rc.Flush() != nil {
		return
	}
	t := time.NewTicker(heartbeat)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-c.quit:
			return
		case msg := <-c.ch:
			if _, err := w.Write(msg); err != nil || rc.Flush() != nil {
				return
			}
		case <-t.C:
			if !s.authed(r) {
				return
			}
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		}
	}
}

var idRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

func (s *Server) item(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idRe.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	it, ok := s.cfg.Store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(time.Hour))

	ct := it.Mime
	if _, _, err := mime.ParseMediaType(ct); err != nil || ct == "" {
		ct = "application/octet-stream"
	}
	if it.Kind == "text" {
		ct = "text/plain; charset=utf-8"
	}
	disp := "attachment"
	if it.Kind == "text" || it.Kind == "image" {
		disp = "inline"
	}
	name := store.SanitizeName(it.Name)
	if it.Name == "" {
		name = it.Kind + extByMime[strings.SplitN(ct, ";", 2)[0]]
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": name}))
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "no-store")

	// ServeContent handles Range (resumable downloads) and Content-Length.
	var content io.ReadSeeker = bytes.NewReader(it.Data)
	mod := it.Created
	if it.Path != "" { // big file picked on the PC: streamed from disk
		f, err := os.Open(it.Path)
		if err != nil {
			s.cfg.Logf("item: file is gone: %v", err)
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil || fi.IsDir() {
			http.NotFound(w, r)
			return
		}
		if !fi.Mode().IsRegular() || fi.Size() > s.cfg.Settings.Get().MaxBytes {
			jsonError(w, http.StatusRequestEntityTooLarge, "too_large", "file exceeds the current size limit")
			return
		}
		content, mod = f, fi.ModTime()
		tr := s.transfers.start("out", it.Name, fi.Size())
		defer tr.end()
		w = &countWriter{ResponseWriter: w, tr: tr}
	}
	http.ServeContent(w, r, name, mod, content)
}

func randomNonce() string {
	var b [16]byte
	rand.Read(b[:])
	return base64.RawStdEncoding.EncodeToString(b[:])
}
