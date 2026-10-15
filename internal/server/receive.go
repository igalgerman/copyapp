package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/igalgerman/copyapp/internal/i18n"
	"github.com/igalgerman/copyapp/internal/settings"
	"github.com/igalgerman/copyapp/internal/store"
)

// ---------- send (phone -> PC) ----------
//
// Where an item goes is decided by the "mode" the phone sends (multipart
// field before any file, JSON field, or ?mode=):
//
//	auto / missing  the PC setting (settings.ReceiveMode)
//	clipboard       text and one PNG to the clipboard; anything else is an error
//	file / folder   everything saved to the save folder
//
// With the "clipboard" setting, auto puts text and a single PNG on the
// clipboard and saves other files. With the "folder" setting everything is
// saved: text as .txt, images under their own name.
// Files stream to disk; only text and clipboard PNGs are held in memory, up
// to clipboardCap.

// clipboardCap bounds what is held in memory: text and clipboard PNGs.
var clipboardCap int64 = store.MaxItemSize

const (
	effClipboard = "clipboard" // strict, explicit
	effFolder    = "folder"
	effAuto      = "auto" // clipboard setting: text/PNG to clipboard, files saved
)

func parseMode(m string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "", "auto":
		return effAuto, nil
	case "clipboard":
		return effClipboard, nil
	case "file", "folder":
		return effFolder, nil
	}
	return "", badRequest("bad_request", "mode must be auto, clipboard or file")
}

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

var errTooLarge = &sendError{status: http.StatusRequestEntityTooLarge, code: "too_large", msg: "item is larger than the size limit"}

// humanSize formats a limit for messages ("200 MB", "2 GB").
func humanSize(n int64) string {
	switch {
	case n >= 1<<30 && n%(1<<30) == 0:
		return fmt.Sprintf("%d GB", n>>30)
	case n >= 1<<20:
		return fmt.Sprintf("%d MB", n>>20)
	default:
		return fmt.Sprintf("%d KB", (n+1023)>>10)
	}
}

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

// countReader reports progress to the transfer registry.
type countReader struct {
	r  io.Reader
	tr *transfer
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.tr.add(int64(n))
	return n, err
}

// sendResult is the success reply. Where is "clipboard" or "folder"; Saved
// lists the file names written to the save folder.
type sendResult struct {
	Kind  string   `json:"kind"` // text | image | file
	Where string   `json:"where"`
	Saved []string `json:"saved,omitempty"`
}

func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	if s.uploads.Add(1) > maxUploads {
		s.uploads.Add(-1)
		s.sendDone(w, r, s.cfg.Settings.Get(), nil, &sendError{status: http.StatusTooManyRequests, code: "busy", msg: "two transfers are already running; wait for one to finish"})
		return
	}
	defer s.uploads.Add(-1)
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(time.Hour)) // big uploads on slow Wi-Fi

	set := s.cfg.Settings.Get()
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	limit := min(set.MaxBytes, clipboardCap) + 64<<10
	if ct == "multipart/form-data" {
		limit = set.MaxBytes + multipartSlack
	}
	if r.ContentLength > limit {
		s.sendDone(w, r, set, nil, errTooLarge)
		return
	}
	body := &limitReader{r: http.MaxBytesReader(w, r.Body, limit+1), n: limit}

	var res *sendResult
	var err error
	switch ct {
	case "application/json":
		res, err = s.sendJSON(body, set, r.URL.Query().Get("mode"))
	case "multipart/form-data":
		tr := s.transfers.start("in", "", r.ContentLength)
		defer tr.end()
		r.Body = io.NopCloser(&countReader{r: body, tr: tr})
		res, err = s.sendMultipart(r, set, tr)
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
	s.sendDone(w, r, set, res, err)
}

func (s *Server) sendDone(w http.ResponseWriter, r *http.Request, set settings.Settings, res *sendResult, err error) {
	if err != nil {
		var se *sendError
		if !errors.As(err, &se) {
			se = &sendError{status: http.StatusBadRequest, code: "bad_request", msg: "could not read the upload", internal: err}
		}
		if se.internal != nil {
			s.cfg.Logf("send: %s: %v", se.code, se.internal)
		}
		msg := se.msg
		if se == errTooLarge {
			msg = "item is larger than the " + humanSize(set.MaxBytes) + " limit"
		}
		s.cfg.Logf("send: failed: %s", se.code)
		if isNavigate(r) {
			http.Redirect(w, r, "/?error="+se.code, http.StatusSeeOther)
			return
		}
		reply := map[string]any{"error": msg, "code": se.code}
		if res != nil && len(res.Saved) > 0 {
			reply["saved"] = res.Saved // partial success: these files are on the PC
			reply["where"] = "folder"
			reply["folder"] = set.SaveDir
		}
		writeJSON(w, se.status, reply)
		return
	}
	s.cfg.Logf("send: received %s -> %s (%d file(s))", res.Kind, res.Where, len(res.Saved))
	if isNavigate(r) { // Android share target posts as a navigation
		http.Redirect(w, r, "/?sent="+res.Kind, http.StatusSeeOther)
		return
	}
	reply := map[string]any{"ok": true, "kind": res.Kind, "where": res.Where}
	if res.Where == "folder" {
		reply["saved"] = res.Saved
		reply["folder"] = set.SaveDir
	}
	writeJSON(w, http.StatusOK, reply)
}

func (s *Server) sendJSON(body io.Reader, set settings.Settings, qmode string) (*sendResult, error) {
	var req struct {
		Text *string `json:"text"`
		Mode string  `json:"mode"`
	}
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		if errors.Is(err, errTooLarge) {
			return nil, errTooLarge
		}
		return nil, badRequest("bad_request", "invalid JSON")
	}
	if req.Mode == "" {
		req.Mode = qmode
	}
	mode, err := parseMode(req.Mode)
	if err != nil {
		return nil, err
	}
	if req.Text == nil {
		return nil, badRequest("empty", "empty text")
	}
	return s.takeText(*req.Text, effective(mode, set), set)
}

func effective(mode string, set settings.Settings) string {
	if mode == effAuto && set.ReceiveMode == settings.ModeFolder {
		return effFolder
	}
	return mode
}

func (s *Server) memCap(set settings.Settings) int64 { return min(set.MaxBytes, clipboardCap) }

// takeText puts text on the clipboard, or saves it as a .txt in folder mode.
func (s *Server) takeText(t string, eff string, set settings.Settings) (*sendResult, error) {
	if strings.TrimSpace(t) == "" {
		return nil, badRequest("empty", "empty text: nothing to copy")
	}
	if int64(len(t)) > s.memCap(set) {
		return nil, errTooLarge
	}
	if !utf8.ValidString(t) {
		return nil, badRequest("bad_text", "text is not valid UTF-8")
	}
	if eff == effFolder {
		name := "text " + time.Now().Format("2006-01-02 15-04-05") + ".txt"
		saved, err := s.saveTo(set, name, strings.NewReader(t))
		if err != nil {
			return nil, err
		}
		s.notifySaved(set, []string{saved})
		return &sendResult{Kind: "file", Where: "folder", Saved: []string{saved}}, nil
	}
	if err := s.cfg.Clip.SetText(t); err != nil {
		return nil, &sendError{status: http.StatusInternalServerError, code: "clipboard", msg: "could not set the PC clipboard", internal: err}
	}
	// Never show the text itself: toasts appear on the lock screen and in history.
	s.cfg.Clip.Notify(i18n.T("toast.textTitle"), i18n.T("toast.textBody", utf8.RuneCountInString(t)))
	return &sendResult{Kind: "text", Where: "clipboard"}, nil
}

// saveTo streams rd into the save folder and returns the file name used.
func (s *Server) saveTo(set settings.Settings, name string, rd io.Reader) (string, error) {
	if err := os.MkdirAll(set.SaveDir, 0o700); err != nil {
		return "", &sendError{status: http.StatusInternalServerError, code: "save", msg: "could not create the receive folder", internal: err}
	}
	path, err := store.SaveFile(set.SaveDir, name, rd, set.MaxBytes)
	if err != nil {
		if errors.Is(err, store.ErrTooLarge) || errors.Is(err, errTooLarge) {
			return "", errTooLarge
		}
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return "", errTooLarge
		}
		return "", &sendError{status: http.StatusInternalServerError, code: "save", msg: "could not save the file", internal: err}
	}
	return fileBase(path), nil
}

func (s *Server) notifySaved(set settings.Settings, saved []string) {
	msg := saved[0]
	if len(saved) > 1 {
		msg = i18n.T("toast.savedMore", saved[0], len(saved)-1)
	}
	s.cfg.Clip.Notify(i18n.T("toast.savedTitle"), i18n.T("toast.savedIn", msg, set.SaveDir))
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

// readUpTo reads all of r if it fits in max bytes; otherwise it returns what
// it read so far and over=true (the rest is still in r).
func readUpTo(r io.Reader, max int64) (b []byte, over bool, err error) {
	b, err = io.ReadAll(io.LimitReader(r, max+1))
	return b, int64(len(b)) > max, err
}

// sendMultipart handles the PWA upload and the Android share target.
// Fields: "mode" (see above; must come before files), "text", "title",
// "url", and one or more files in "file" or "files".
// On failure after some files were saved, the result lists them alongside the
// error.
func (s *Server) sendMultipart(r *http.Request, set settings.Settings, tr *transfer) (res *sendResult, err error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, badRequest("bad_request", "invalid multipart body")
	}
	mode, err := parseMode(r.URL.Query().Get("mode"))
	if err != nil {
		return nil, err
	}
	memCap := s.memCap(set)
	var (
		text, title, linkURL string
		saved                []string
		pending              *pendingPNG
		nFiles               int
	)
	defer func() {
		if err != nil && len(saved) > 0 {
			res = &sendResult{Kind: "file", Where: "folder", Saved: saved}
			s.cfg.Clip.Notify(i18n.T("toast.partTitle"), i18n.T("toast.partBody", len(saved)))
		}
	}()
	readSmall := func(p io.Reader, max int64) ([]byte, error) {
		b, over, err := readUpTo(p, max)
		if err != nil {
			return nil, err
		}
		if over {
			return nil, errTooLarge
		}
		return b, nil
	}
	save := func(name string, rd io.Reader) error {
		n, err := s.saveTo(set, name, rd)
		if err != nil {
			return err
		}
		saved = append(saved, n)
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
			max := memCap
			if field == "mode" {
				max = 64
			}
			b, err := readSmall(p, max)
			if err != nil {
				if field == "mode" && errors.Is(err, errTooLarge) {
					return nil, badRequest("bad_request", "mode must be auto, clipboard or file")
				}
				return nil, err
			}
			v := string(b)
			switch field {
			case "mode":
				if nFiles > 0 {
					return nil, badRequest("bad_request", "mode must come before the files")
				}
				if mode, err = parseMode(v); err != nil {
					return nil, err
				}
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
			tr.setName(name)
			switch eff := effective(mode, set); {
			case eff == effClipboard:
				if nFiles > 1 {
					return nil, badRequest("bad_request", "only one image can go to the clipboard")
				}
				if ctype != "image/png" && ctype != "application/octet-stream" && ctype != "" {
					return nil, badRequest("not_png", "clipboard images must be PNG")
				}
				b, err := readSmall(p, memCap)
				if err != nil {
					return nil, err
				}
				pending = &pendingPNG{name: name, data: b}
			case eff == effAuto && ctype == "image/png" && nFiles == 1:
				// Held until we know it is the only file; too big for memory: saved.
				b, over, err := readUpTo(p, memCap)
				if err != nil {
					return nil, err
				}
				if over {
					if err := save(name, io.MultiReader(bytes.NewReader(b), p)); err != nil {
						return nil, err
					}
				} else {
					pending = &pendingPNG{name: name, data: b}
				}
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

	eff := effective(mode, set)
	if pending != nil && nFiles == 1 {
		cfg, err := png.DecodeConfig(bytes.NewReader(pending.data))
		if err != nil {
			return nil, badRequest("not_png", "the image is not a valid PNG; the PC clipboard was not changed")
		}
		huge := int64(cfg.Width)*int64(cfg.Height) > store.MaxImagePixels
		if huge && eff == effClipboard {
			return nil, &sendError{status: http.StatusRequestEntityTooLarge, code: "too_many_pixels",
				msg: "the image is larger than 50 megapixels; send it as a file"}
		}
		if !huge { // a huge PNG without an explicit mode is saved as a file below
			if err := s.cfg.Clip.SetImagePNG(pending.data); err != nil {
				return nil, &sendError{status: http.StatusInternalServerError, code: "clipboard", msg: "could not set the PC clipboard", internal: err}
			}
			s.cfg.Clip.Notify(i18n.T("toast.imageTitle"), i18n.T("toast.imageBody"))
			return &sendResult{Kind: "image", Where: "clipboard"}, nil
		}
	}
	if err := flushPending(); err != nil {
		return nil, err
	}
	if len(saved) > 0 {
		s.notifySaved(set, saved)
		return &sendResult{Kind: "file", Where: "folder", Saved: saved}, nil
	}
	if eff == effClipboard && nFiles > 0 {
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
	return s.takeText(t, eff, set)
}

func fileBase(p string) string {
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return p[i+1:]
	}
	return p
}
