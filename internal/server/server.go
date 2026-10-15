// Package server is the LAN HTTPS API and PWA host.
//
// Routes (all subject to the private-source-IP filter):
//
//	GET  /, static PWA files     public
//	GET  /pair?t=TOKEN           exchange pairing token -> session cookie, 303 to /
//	GET  /ca.crt                 CA certificate; while pairing is open, or with a session
//	GET  /status                 session check: {"paired":true,"fingerprint":"..."}
//	POST /send                   phone -> PC (JSON {"text":...} or multipart)
//	GET  /events                 SSE stream of outbox items (PC -> phone)
//	GET  /item/{id}              outbox item bytes
//	POST /unpair                 drop the session
//	GET  /pairing, /qr, /pairing/status, POST /pairing/open
//	                             PC-only pages (loopback source and localhost Host)
package server

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
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
	"time"
	"unicode/utf8"

	"github.com/igalgerman/copyapp/internal/auth"
	"github.com/igalgerman/copyapp/internal/store"
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
	Auth    *auth.Auth
	Store   *store.Store
	Clip    Clipboard
	SaveDir string // received files go here
	CAPEM   []byte
	// BaseURL returns the phone-facing origin, e.g. "https://192.168.1.5:8443".
	// It is called each time a pairing URL is built.
	BaseURL func() string
	// MaxBytes caps one item; 0 means store.MaxItemSize (200 MB).
	MaxBytes int64
	// Logf receives one line per request (method, path without query, status)
	// and errors. Never secrets. Optional.
	Logf func(format string, args ...any)
}

type client struct {
	ch   chan []byte
	quit chan struct{}
}

// Server is an http.Handler.
type Server struct {
	cfg     Config
	handler http.Handler

	mu        sync.Mutex
	clients   map[*client]struct{}
	pairToken string
	openedAt  time.Time
	pairedAt  time.Time
}

func New(cfg Config) *Server {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = store.MaxItemSize
	}
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	if cfg.BaseURL == nil {
		cfg.BaseURL = func() string { return "https://localhost" }
	}
	s := &Server{cfg: cfg, clients: map[*client]struct{}{}}

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", staticHandler(static))
	mux.HandleFunc("GET /pair", s.pair)
	mux.HandleFunc("GET /ca.crt", s.caCert)
	mux.HandleFunc("GET /status", s.requireAuth(s.status))
	mux.HandleFunc("POST /send", s.requireAuth(s.send))
	mux.HandleFunc("GET /events", s.requireAuth(s.events))
	mux.HandleFunc("GET /item/{id}", s.requireAuth(s.item))
	mux.HandleFunc("POST /unpair", s.requireAuth(s.unpair))
	mux.HandleFunc("GET /pairing", s.localOnly(s.pairingPage))
	mux.HandleFunc("GET /pairing/status", s.localOnly(s.pairingStatus))
	mux.HandleFunc("POST /pairing/open", s.localOnly(s.pairingOpen))
	mux.HandleFunc("GET /qr", s.localOnly(s.qr))
	s.handler = s.logged(securityHeaders(privateOnly(mux)))
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

// StartPairing opens a 2-minute pairing window and returns the URL to put in
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
	s.pairToken = ""
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

// localOnly allows only loopback peers that address us as localhost (the
// Host check blocks DNS-rebinding pages).
func (s *Server) localOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := remoteAddr(r)
		if !ok || !a.IsLoopback() || !isLocalHost(r.Host) {
			jsonError(w, http.StatusForbidden, "forbidden", "only available on this PC")
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

const pairFailHTML = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>CopyApp</title><body style="font:16px system-ui;margin:2rem;max-width:30rem">
<h1>Pairing link expired</h1><p>This code was already used or is older than 2 minutes.</p>
<p>On your PC, open the CopyApp tray menu and choose <b>Pair phone</b> to get a new code.</p></body>`

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
		io.WriteString(w, pairFailHTML)
		return
	}
	s.mu.Lock()
	s.pairToken = ""
	s.pairedAt = time.Now()
	s.mu.Unlock()
	http.SetCookie(w, s.cookie(sid, 400*24*3600))
	s.cfg.Clip.Notify("CopyApp", "Phone paired")
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
	writeJSON(w, http.StatusOK, map[string]any{"paired": true, "fingerprint": fingerprint(s.cfg.CAPEM)})
}

func (s *Server) unpair(w http.ResponseWriter, r *http.Request) {
	s.Unpair()
	http.SetCookie(w, s.cookie("", -1))
	s.cfg.Clip.Notify("CopyApp", "Phone unpaired")
	w.WriteHeader(http.StatusNoContent)
}

// ---------- send (phone -> PC) ----------

type sendError struct {
	status   int
	code     string
	msg      string
	internal error
}

func (e *sendError) Error() string { return e.msg }

func badRequest(code, msg string) *sendError {
	return &sendError{status: http.StatusBadRequest, code: code, msg: msg}
}

var errTooLarge = &sendError{status: http.StatusRequestEntityTooLarge, code: "too_large", msg: "item is larger than the 200 MB limit"}

// limitReader returns errTooLarge once more than n bytes are read.
type limitReader struct {
	r    io.Reader
	n    int64
	over bool
}

func (l *limitReader) Read(p []byte) (int, error) {
	if l.n < 0 {
		l.over = true
		return 0, errTooLarge
	}
	if int64(len(p)) > l.n+1 {
		p = p[:l.n+1]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	if l.n < 0 {
		l.over = true
		return n, errTooLarge
	}
	return n, err
}

type sendResult struct {
	Kind  string   `json:"kind"` // text | image | file
	Saved []string `json:"saved,omitempty"`
}

func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(time.Hour)) // big uploads on slow Wi-Fi

	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	slack := int64(64 << 10)
	if ct == "multipart/form-data" {
		slack = multipartSlack
	}
	limit := s.cfg.MaxBytes + slack
	if r.ContentLength > limit {
		s.sendDone(w, r, nil, errTooLarge)
		return
	}
	body := &limitReader{r: http.MaxBytesReader(w, r.Body, limit+1), n: limit}

	var res *sendResult
	var err error
	switch ct {
	case "application/json":
		res, err = s.sendJSON(body)
	case "multipart/form-data":
		r.Body = io.NopCloser(body)
		res, err = s.sendMultipart(r)
	default:
		err = &sendError{status: http.StatusUnsupportedMediaType, code: "unsupported", msg: "send JSON or multipart/form-data"}
	}
	if body.over {
		err = errTooLarge
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) || errors.Is(err, store.ErrTooLarge) {
		err = errTooLarge
	}
	s.sendDone(w, r, res, err)
}

func (s *Server) sendDone(w http.ResponseWriter, r *http.Request, res *sendResult, err error) {
	if err != nil {
		var se *sendError
		if !errors.As(err, &se) {
			se = &sendError{status: http.StatusBadRequest, code: "bad_request", msg: "could not read the upload", internal: err}
		}
		if se.internal != nil {
			s.cfg.Logf("send: %s: %v", se.code, se.internal)
		}
		if isNavigate(r) {
			http.Redirect(w, r, "/?error="+se.code, http.StatusSeeOther)
			return
		}
		jsonError(w, se.status, se.code, se.msg)
		return
	}
	if isNavigate(r) { // Android share target posts as a navigation
		http.Redirect(w, r, "/?sent="+res.Kind, http.StatusSeeOther)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "kind": res.Kind, "saved": res.Saved})
}

func (s *Server) sendJSON(body io.Reader) (*sendResult, error) {
	var req struct {
		Text *string `json:"text"`
	}
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		if errors.Is(err, errTooLarge) {
			return nil, errTooLarge
		}
		return nil, badRequest("bad_request", "invalid JSON")
	}
	if req.Text == nil {
		return nil, badRequest("empty", "empty text")
	}
	return s.setText(*req.Text)
}

func (s *Server) setText(t string) (*sendResult, error) {
	if strings.TrimSpace(t) == "" {
		return nil, badRequest("empty", "empty text: nothing to copy")
	}
	if int64(len(t)) > s.cfg.MaxBytes {
		return nil, errTooLarge
	}
	if !utf8.ValidString(t) {
		return nil, badRequest("bad_text", "text is not valid UTF-8")
	}
	if err := s.cfg.Clip.SetText(t); err != nil {
		return nil, &sendError{status: http.StatusInternalServerError, code: "clipboard", msg: "could not set the PC clipboard", internal: err}
	}
	s.cfg.Clip.Notify("Copied from phone", preview(t))
	return &sendResult{Kind: "text"}, nil
}

func preview(t string) string {
	t = strings.Join(strings.Fields(t), " ")
	if r := []rune(t); len(r) > 80 {
		t = string(r[:79]) + "…"
	}
	return t
}

var extByMime = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp",
	"image/heic": ".heic", "text/plain": ".txt", "application/pdf": ".pdf",
	"video/mp4": ".mp4", "audio/mpeg": ".mp3", "application/zip": ".zip",
}

type pendingPNG struct {
	name string
	data []byte
}

// sendMultipart handles the PWA upload and the Android share target.
// Fields: "mode" ("clipboard" | "file", optional, must come before files),
// "text", "title", "url", and one or more files in "file" or "files".
// Without mode, a single image/png file goes to the clipboard and everything
// else is saved to SaveDir.
func (s *Server) sendMultipart(r *http.Request) (*sendResult, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, badRequest("bad_request", "invalid multipart body")
	}
	var (
		mode                 string
		text, title, linkURL string
		saved                []string
		pending              *pendingPNG
		nFiles               int
	)
	readBytes := func(p io.Reader) ([]byte, error) {
		b, err := io.ReadAll(io.LimitReader(p, s.cfg.MaxBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(b)) > s.cfg.MaxBytes {
			return nil, errTooLarge
		}
		return b, nil
	}
	readField := func(p io.Reader) (string, error) {
		b, err := readBytes(p)
		return string(b), err
	}
	save := func(name string, rd io.Reader) error {
		if err := os.MkdirAll(s.cfg.SaveDir, 0o700); err != nil {
			return &sendError{status: http.StatusInternalServerError, code: "save", msg: "could not create the receive folder", internal: err}
		}
		path, err := store.SaveFile(s.cfg.SaveDir, name, rd, s.cfg.MaxBytes)
		if err != nil {
			if errors.Is(err, store.ErrTooLarge) || errors.Is(err, errTooLarge) {
				return errTooLarge
			}
			return &sendError{status: http.StatusInternalServerError, code: "save", msg: "could not save the file", internal: err}
		}
		saved = append(saved, fileBase(path))
		return nil
	}
	flushPending := func() error {
		if pending == nil {
			return nil
		}
		p := pending
		pending = nil
		return save(p.name, bytes.NewReader(p.data))
	}

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		field := p.FormName()
		switch field {
		case "mode", "text", "title", "url":
			v, err := readField(p)
			if err != nil {
				return nil, err
			}
			switch field {
			case "mode":
				mode = strings.TrimSpace(v)
			case "text":
				text = v
			case "title":
				title = v
			case "url":
				linkURL = v
			}
		case "file", "files":
			nFiles++
			ctype, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
			name := p.FileName()
			if strings.Trim(name, ". ") == "" {
				name = "file" + extByMime[ctype]
			}
			switch {
			case mode == "clipboard":
				if nFiles > 1 {
					return nil, badRequest("bad_request", "only one image can go to the clipboard")
				}
				if ctype != "image/png" && ctype != "application/octet-stream" && ctype != "" {
					return nil, badRequest("not_png", "clipboard images must be PNG")
				}
				b, err := readBytes(p)
				if err != nil {
					return nil, err
				}
				pending = &pendingPNG{name: name, data: b}
			case mode != "file" && ctype == "image/png" && nFiles == 1:
				b, err := readBytes(p) // held until we know it is the only file
				if err != nil {
					return nil, err
				}
				pending = &pendingPNG{name: name, data: b}
			default:
				if err := flushPending(); err != nil {
					return nil, err
				}
				if err := save(name, p); err != nil {
					return nil, err
				}
			}
		default:
			io.Copy(io.Discard, io.LimitReader(p, multipartSlack))
		}
		p.Close()
	}

	if pending != nil && nFiles == 1 {
		if _, err := png.DecodeConfig(bytes.NewReader(pending.data)); err != nil {
			return nil, badRequest("not_png", "the image is not a valid PNG; the PC clipboard was not changed")
		}
		if err := s.cfg.Clip.SetImagePNG(pending.data); err != nil {
			return nil, &sendError{status: http.StatusInternalServerError, code: "clipboard", msg: "could not set the PC clipboard", internal: err}
		}
		s.cfg.Clip.Notify("Image copied from phone", "Paste it with Ctrl+V")
		return &sendResult{Kind: "image"}, nil
	}
	if err := flushPending(); err != nil {
		return nil, err
	}
	if len(saved) > 0 {
		msg := saved[0]
		if len(saved) > 1 {
			msg = fmt.Sprintf("%s and %d more", saved[0], len(saved)-1)
		}
		s.cfg.Clip.Notify("Saved from phone", msg)
		return &sendResult{Kind: "file", Saved: saved}, nil
	}
	if mode == "clipboard" {
		return nil, badRequest("not_png", "no PNG image in the request")
	}
	t := text
	if linkURL != "" && !strings.Contains(t, linkURL) {
		if strings.TrimSpace(t) != "" {
			t += "\n"
		}
		t += linkURL
	}
	if strings.TrimSpace(t) == "" {
		t = title
	}
	return s.setText(t)
}

func fileBase(p string) string {
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// ---------- outbox (PC -> phone) ----------

const inlineTextMax = 256 << 10

type itemJSON struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"`
	Name    string    `json:"name,omitempty"`
	Mime    string    `json:"mime,omitempty"`
	Size    int       `json:"size"`
	Created time.Time `json:"created"`
	Text    string    `json:"text,omitempty"` // text items up to 256 KB
}

func sseMessage(it store.Item) []byte {
	j := itemJSON{ID: it.ID, Kind: it.Kind, Name: it.Name, Mime: it.Mime, Size: len(it.Data), Created: it.Created}
	if it.Kind == "text" && len(it.Data) <= inlineTextMax {
		j.Text = string(it.Data)
	}
	b, _ := json.Marshal(j)
	return []byte(fmt.Sprintf("id: %s\nevent: item\ndata: %s\n\n", it.ID, b))
}

func (s *Server) subscribe() *client {
	c := &client{ch: make(chan []byte, 16), quit: make(chan struct{})}
	s.mu.Lock()
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
	h.Set("Content-Length", fmt.Sprint(len(it.Data)))
	w.Write(it.Data)
}

func randomNonce() string {
	var b [16]byte
	rand.Read(b[:])
	return base64.RawStdEncoding.EncodeToString(b[:])
}
