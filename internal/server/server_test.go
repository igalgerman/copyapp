package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/igalgerman/copyapp/internal/auth"
	"github.com/igalgerman/copyapp/internal/store"
)

// ---- fakes and helpers ----

type fakeClip struct {
	mu     sync.Mutex
	texts  []string
	images [][]byte
	notes  []string
}

func (f *fakeClip) SetText(s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.texts = append(f.texts, s)
	return nil
}
func (f *fakeClip) SetImagePNG(b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.images = append(f.images, b)
	return nil
}
func (f *fakeClip) Notify(title, msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notes = append(f.notes, title+": "+msg)
}
func (f *fakeClip) calls() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.texts), len(f.images)
}

type env struct {
	t    *testing.T
	s    *Server
	a    *auth.Auth
	st   *store.Store
	clip *fakeClip
	dir  string
	logs *bytes.Buffer
	lmu  sync.Mutex
}

const lanAddr = "192.168.1.20:5555"

func newEnv(t *testing.T, max int64) *env {
	t.Helper()
	e := &env{t: t, a: auth.New(), st: store.New(), clip: &fakeClip{}, logs: &bytes.Buffer{}}
	e.dir = filepath.Join(t.TempDir(), "recv")
	e.s = New(Config{
		Auth: e.a, Store: e.st, Clip: e.clip, SaveDir: e.dir,
		CAPEM:    []byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"),
		MaxBytes: max,
		BaseURL:  func() string { return "https://192.168.1.5:8443" },
		Logf: func(f string, a ...any) {
			e.lmu.Lock()
			defer e.lmu.Unlock()
			fmt.Fprintf(e.logs, f+"\n", a...)
		},
	})
	return e
}

func (e *env) req(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.RemoteAddr = lanAddr
	return r
}

func (e *env) do(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.s.ServeHTTP(w, r)
	return w
}

// pair performs the real /pair flow and returns the session cookie.
func (e *env) pair() *http.Cookie {
	e.t.Helper()
	tok := e.a.OpenPairing()
	w := e.do(e.req("GET", "/pair?t="+tok, nil))
	if w.Code != http.StatusSeeOther {
		e.t.Fatalf("pair: %d %s", w.Code, w.Body)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			return c
		}
	}
	e.t.Fatal("no session cookie")
	return nil
}

func (e *env) authed(method, target string, body io.Reader, c *http.Cookie) *http.Request {
	r := e.req(method, target, body)
	r.AddCookie(c)
	return r
}

func (e *env) sendJSON(c *http.Cookie, v any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(v)
	r := e.authed("POST", "/send", bytes.NewReader(b), c)
	r.Header.Set("Content-Type", "application/json")
	return e.do(r)
}

type part struct {
	field, filename, ctype string
	data                   []byte
	isFile                 bool
}

func multipartBody(t *testing.T, parts ...part) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		if p.isFile {
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, p.field, p.filename))
		} else {
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q`, p.field))
		}
		if p.ctype != "" {
			h.Set("Content-Type", p.ctype)
		}
		w, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(p.data)
	}
	mw.Close()
	return mw.FormDataContentType(), buf.Bytes()
}

func (e *env) sendMultipart(c *http.Cookie, parts ...part) *httptest.ResponseRecorder {
	ct, body := multipartBody(e.t, parts...)
	r := e.authed("POST", "/send", bytes.NewReader(body), c)
	r.Header.Set("Content-Type", ct)
	return e.do(r)
}

func pngBytes() []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	return b.Bytes()
}

func (e *env) savedFiles() []string {
	var out []string
	filepath.WalkDir(filepath.Dir(e.dir), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// ---- Review focus: public IP / missing or forged cookie ----

func TestProtectedRoutesNeedCookie(t *testing.T) {
	e := newEnv(t, 0)
	e.pair() // a session exists, but these requests do not carry it
	for _, rt := range [][2]string{
		{"POST", "/send"}, {"GET", "/events"}, {"GET", "/item/0123456789abcdef0123456789abcdef"},
		{"POST", "/unpair"}, {"GET", "/status"},
	} {
		if w := e.do(e.req(rt[0], rt[1], strings.NewReader(`{"text":"x"}`))); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without cookie: %d", rt[0], rt[1], w.Code)
		}
		forged := &http.Cookie{Name: CookieName, Value: strings.Repeat("a", 64)}
		if w := e.do(e.authed(rt[0], rt[1], strings.NewReader(`{"text":"x"}`), forged)); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s forged cookie: %d", rt[0], rt[1], w.Code)
		}
	}
	if n, _ := e.clip.calls(); n != 0 {
		t.Fatal("clipboard touched without auth")
	}
}

func TestPublicIPForbidden(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	for _, addr := range []string{"8.8.8.8:1", "[2001:4860:4860::8888]:443", "[::ffff:8.8.8.8]:1", "100.64.0.1:1", "garbage"} {
		for _, path := range []string{"/", "/send", "/pair?t=x", "/ca.crt", "/events"} {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second) // /events would stream if let through
			r := e.authed("GET", path, nil, c).WithContext(ctx)
			r.RemoteAddr = addr
			if w := e.do(r); w.Code != http.StatusForbidden {
				t.Errorf("%s from %s: %d", path, addr, w.Code)
			}
			cancel()
		}
	}
	for _, addr := range []string{"10.1.2.3:1", "172.20.0.1:1", "192.168.0.9:1", "127.0.0.1:1", "[::1]:1", "169.254.3.4:1", "[fe80::1]:1", "[fd00::5]:1"} {
		r := e.authed("GET", "/status", nil, c)
		r.RemoteAddr = addr
		if w := e.do(r); w.Code != http.StatusOK {
			t.Errorf("private %s: %d", addr, w.Code)
		}
	}
}

// ---- Review focus: pairing token single use / expiry ----

func TestPairSetsSecureCookieAndSendTextWorks(t *testing.T) {
	e := newEnv(t, 0)
	tok := e.a.OpenPairing()
	w := e.do(e.req("GET", "/pair?t="+tok, nil))
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("pair: %d loc=%q", w.Code, w.Header().Get("Location"))
	}
	raw := w.Header().Get("Set-Cookie")
	for _, attr := range []string{"HttpOnly", "Secure", "SameSite=Strict", "Path=/"} {
		if !strings.Contains(raw, attr) {
			t.Errorf("cookie lacks %s: %s", attr, raw)
		}
	}
	if w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Error("pair response must not leak the token via Referer")
	}
	c := w.Result().Cookies()[0]
	if w := e.sendJSON(c, map[string]string{"text": "hello from phone"}); w.Code != http.StatusOK {
		t.Fatalf("send: %d %s", w.Code, w.Body)
	}
	if len(e.clip.texts) != 1 || e.clip.texts[0] != "hello from phone" {
		t.Fatalf("clipboard: %v", e.clip.texts)
	}
	if len(e.clip.notes) == 0 {
		t.Fatal("no toast")
	}
}

func TestPairTokenReuseRejected(t *testing.T) {
	e := newEnv(t, 0)
	tok := e.a.OpenPairing()
	if w := e.do(e.req("GET", "/pair?t="+tok, nil)); w.Code != http.StatusSeeOther {
		t.Fatal("first pair failed")
	}
	w := e.do(e.req("GET", "/pair?t="+tok, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("second pair: %d", w.Code)
	}
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("cookie set on failed pairing")
	}
}

func TestPairTokenExpiredRejected(t *testing.T) {
	e := newEnv(t, 0)
	base := time.Now()
	e.a.SetClock(func() time.Time { return base })
	tok := e.a.OpenPairing()
	e.a.SetClock(func() time.Time { return base.Add(auth.PairWindow + time.Second) })
	if w := e.do(e.req("GET", "/pair?t="+tok, nil)); w.Code != http.StatusUnauthorized {
		t.Fatalf("expired token: %d", w.Code)
	}
}

func TestPairBadOrMissingToken(t *testing.T) {
	e := newEnv(t, 0)
	e.a.OpenPairing()
	for _, q := range []string{"/pair", "/pair?t=", "/pair?t=abc", "/pair?t=" + strings.Repeat("0", 64)} {
		if w := e.do(e.req("GET", q, nil)); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: %d", q, w.Code)
		}
	}
}

func TestHeadPairDoesNotConsume(t *testing.T) {
	e := newEnv(t, 0)
	tok := e.a.OpenPairing()
	e.do(e.req("HEAD", "/pair?t="+tok, nil))
	if w := e.do(e.req("GET", "/pair?t="+tok, nil)); w.Code != http.StatusSeeOther {
		t.Fatalf("HEAD consumed the token: %d", w.Code)
	}
}

func TestTokensNeverLogged(t *testing.T) {
	e := newEnv(t, 0)
	tok := e.a.OpenPairing()
	e.do(e.req("GET", "/pair?t="+tok+"x", nil)) // failure path
	w := e.do(e.req("GET", "/pair?t="+tok, nil))
	c := w.Result().Cookies()[0]
	e.do(e.req("GET", "/pair?t="+tok, nil)) // reuse
	e.do(e.authed("GET", "/nope", nil, c))
	r := e.authed("POST", "/send", strings.NewReader("{"), c)
	r.Header.Set("Content-Type", "application/json")
	e.do(r)
	e.lmu.Lock()
	defer e.lmu.Unlock()
	logs := e.logs.String()
	if strings.Contains(logs, tok) || strings.Contains(logs, c.Value) {
		t.Fatalf("secret in logs:\n%s", logs)
	}
}

// ---- Review focus: empty text / non-PNG clipboard image ----

func TestEmptyTextRejected(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	for _, txt := range []string{"", "   \n\t"} {
		w := e.sendJSON(c, map[string]string{"text": txt})
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "empty") {
			t.Errorf("text %q: %d %s", txt, w.Code, w.Body)
		}
	}
	w := e.sendMultipart(c, part{field: "text", data: nil})
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty multipart: %d", w.Code)
	}
	r := e.authed("POST", "/send", strings.NewReader("{bad json"), c)
	r.Header.Set("Content-Type", "application/json")
	if w := e.do(r); w.Code != http.StatusBadRequest {
		t.Errorf("bad json: %d", w.Code)
	}
	r = e.authed("POST", "/send", strings.NewReader("hi"), c)
	r.Header.Set("Content-Type", "text/plain")
	if w := e.do(r); w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain body: %d", w.Code)
	}
	if n, m := e.clip.calls(); n+m != 0 {
		t.Fatal("clipboard changed")
	}
}

func TestNonPNGClipboardImageRejected(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	jpeg := []byte("\xff\xd8\xff\xe0 fake jpeg")
	cases := [][]part{
		{{field: "mode", data: []byte("clipboard")}, {field: "file", filename: "a.jpg", ctype: "image/jpeg", data: jpeg, isFile: true}},
		{{field: "file", filename: "a.png", ctype: "image/png", data: []byte("not a png at all"), isFile: true}},
		{{field: "mode", data: []byte("clipboard")}, {field: "file", filename: "a.png", ctype: "image/png", data: jpeg, isFile: true}},
	}
	for i, ps := range cases {
		w := e.sendMultipart(c, ps...)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "PNG") {
			t.Errorf("case %d: %d %s", i, w.Code, w.Body)
		}
	}
	if n, m := e.clip.calls(); n+m != 0 {
		t.Fatal("clipboard changed")
	}
	if f := e.savedFiles(); len(f) != 0 {
		t.Fatalf("files saved: %v", f)
	}
}

func TestPNGGoesToClipboard(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	p := pngBytes()
	w := e.sendMultipart(c, part{field: "file", filename: "shot.png", ctype: "image/png", data: p, isFile: true})
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if len(e.clip.images) != 1 || !bytes.Equal(e.clip.images[0], p) {
		t.Fatal("SetImagePNG not called with the PNG")
	}
	if f := e.savedFiles(); len(f) != 0 {
		t.Fatalf("PNG for clipboard should not be saved: %v", f)
	}
}

func TestModeFileSavesPNG(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	w := e.sendMultipart(c, part{field: "mode", data: []byte("file")},
		part{field: "files", filename: "shot.png", ctype: "image/png", data: pngBytes(), isFile: true})
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if _, m := e.clip.calls(); m != 0 {
		t.Fatal("clipboard used in file mode")
	}
	if f := e.savedFiles(); len(f) != 1 || filepath.Base(f[0]) != "shot.png" {
		t.Fatalf("saved: %v", f)
	}
}

func TestShareTargetTextAndNavigateRedirect(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	ct, body := multipartBody(t, part{field: "title", data: []byte("Title")}, part{field: "text", data: []byte("look")}, part{field: "url", data: []byte("https://x.test/a")})
	r := e.authed("POST", "/send", bytes.NewReader(body), c)
	r.Header.Set("Content-Type", ct)
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	w := e.do(r)
	if w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), "/?sent=") {
		t.Fatalf("navigate: %d %q", w.Code, w.Header().Get("Location"))
	}
	if len(e.clip.texts) != 1 || e.clip.texts[0] != "look\nhttps://x.test/a" {
		t.Fatalf("text: %q", e.clip.texts)
	}
}

// ---- files, filenames (Review focus) ----

func TestNonImageFileSaved(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	w := e.sendMultipart(c, part{field: "file", filename: "notes.txt", ctype: "text/plain", data: []byte("abc"), isFile: true})
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if n, m := e.clip.calls(); n+m != 0 {
		t.Fatal("file placed on clipboard")
	}
	b, err := os.ReadFile(filepath.Join(e.dir, "notes.txt"))
	if err != nil || string(b) != "abc" {
		t.Fatalf("saved file: %v %q", err, b)
	}
}

func TestHostileFilenamesStayInFolder(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	names := []string{`..\..\x.exe`, "CON", "a/b", "", strings.Repeat("n", 500), "../../../etc/passwd", `C:\Windows\evil.dll`, "..", "nul.txt"}
	for _, n := range names {
		w := e.sendMultipart(c, part{field: "file", filename: n, ctype: "application/octet-stream", data: []byte("x"), isFile: true})
		if w.Code != http.StatusOK {
			t.Fatalf("%q: %d %s", n, w.Code, w.Body)
		}
	}
	files := e.savedFiles()
	if len(files) != len(names) {
		t.Fatalf("want %d files, got %v", len(names), files)
	}
	for _, f := range files {
		base := filepath.Base(f)
		if filepath.Dir(f) != e.dir {
			t.Errorf("escaped folder: %s", f)
		}
		if len(base) > 130 || strings.EqualFold(strings.Split(base, ".")[0], "CON") || strings.EqualFold(strings.Split(base, ".")[0], "nul") {
			t.Errorf("bad name: %s", base)
		}
	}
}

// ---- Review focus: >200 MB rejected early, nothing on disk ----

type failReader struct{ t *testing.T }

func (f failReader) Read([]byte) (int, error) {
	f.t.Error("body read despite oversized Content-Length")
	return 0, io.EOF
}

func TestTooLargeRejected(t *testing.T) {
	e := newEnv(t, 1024)
	c := e.pair()
	// Declared too large: rejected before reading.
	r := e.authed("POST", "/send", failReader{t}, c)
	r.ContentLength = 300 << 20
	r.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	if w := e.do(r); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("declared size: %d", w.Code)
	}
	// Streamed too large (no/unknown length): rejected, partial file removed.
	ct, body := multipartBody(t, part{field: "file", filename: "big.bin", data: bytes.Repeat([]byte("z"), 4096), isFile: true})
	r = e.authed("POST", "/send", io.MultiReader(bytes.NewReader(body)), c)
	r.ContentLength = -1
	r.Header.Set("Content-Type", ct)
	if w := e.do(r); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("streamed: %d %s", w.Code, w.Body)
	}
	// File just over the limit even though the body fits the multipart slack.
	ct, body = multipartBody(t, part{field: "file", filename: "big2.bin", data: bytes.Repeat([]byte("z"), 1025), isFile: true})
	r = e.authed("POST", "/send", bytes.NewReader(body), c)
	r.Header.Set("Content-Type", ct)
	if w := e.do(r); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("file over limit: %d %s", w.Code, w.Body)
	}
	// Large PNG for clipboard and large JSON text.
	big := e.sendJSON(c, map[string]string{"text": strings.Repeat("t", 4096)})
	if big.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("json: %d", big.Code)
	}
	if f := e.savedFiles(); len(f) != 0 {
		t.Fatalf("left on disk: %v", f)
	}
	if n, m := e.clip.calls(); n+m != 0 {
		t.Fatal("clipboard changed")
	}
}

func TestDefaultLimitIs200MB(t *testing.T) {
	e := newEnv(t, 0)
	if e.s.cfg.MaxBytes != store.MaxItemSize || store.MaxItemSize != 200<<20 {
		t.Fatalf("limit %d", e.s.cfg.MaxBytes)
	}
}

// ---- outbox: /item, /events, Broadcast ----

func TestItemDownload(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	id, err := e.s.Publish(store.Item{Kind: "file", Name: `evil".html`, Mime: "text/html", Data: []byte("<script>x</script>")})
	if err != nil {
		t.Fatal(err)
	}
	w := e.do(e.authed("GET", "/item/"+id, nil, c))
	if w.Code != 200 || w.Body.String() != "<script>x</script>" {
		t.Fatalf("%d %q", w.Code, w.Body)
	}
	h := w.Header()
	if h.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") ||
		!strings.Contains(h.Get("Content-Security-Policy"), "sandbox") || h.Get("Cache-Control") != "no-store" {
		t.Fatalf("unsafe headers: %v", h)
	}
	if strings.Contains(h.Get("Content-Disposition"), `evil".html`) {
		t.Fatalf("unescaped filename: %s", h.Get("Content-Disposition"))
	}
	for _, bad := range []string{"/item/" + strings.Repeat("0", 32), "/item/..%2f..%2fserver.go", "/item/"} {
		if w := e.do(e.authed("GET", bad, nil, c)); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}
}

func readEvent(t *testing.T, br *bufio.Reader) map[string]any {
	t.Helper()
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("stream ended: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			var m map[string]any
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &m); err != nil {
				t.Fatal(err)
			}
			return m
		}
	}
}

func TestEventsStreamAndBroadcast(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	id1, _ := e.s.Publish(store.Item{Kind: "text", Mime: "text/plain", Data: []byte("first")})
	ts := httptest.NewServer(e.s) // RemoteAddr is 127.0.0.1 (allowed)
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/events", nil)
	req.AddCookie(c)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("%d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	br := bufio.NewReader(resp.Body)
	ev := readEvent(t, br)
	if ev["id"] != id1 || ev["kind"] != "text" || ev["text"] != "first" {
		t.Fatalf("initial event: %v", ev)
	}
	id2, _ := e.s.Publish(store.Item{Kind: "image", Name: "a.png", Mime: "image/png", Data: pngBytes()})
	ev = readEvent(t, br)
	if ev["id"] != id2 || ev["kind"] != "image" || ev["size"].(float64) <= 0 {
		t.Fatalf("broadcast event: %v", ev)
	}
	if _, has := ev["data"]; has {
		t.Fatal("binary data must not be inlined")
	}

	// Unpair closes the stream.
	e.s.Unpair()
	done := make(chan struct{})
	go func() { io.Copy(io.Discard, br); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stream not closed after unpair")
	}
}

func TestUnpairRoute(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	w := e.do(e.authed("POST", "/unpair", nil, c))
	if w.Code != http.StatusNoContent {
		t.Fatalf("%d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatal("cookie not cleared")
	}
	if w := e.sendJSON(c, map[string]string{"text": "x"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("session survived unpair: %d", w.Code)
	}
	if w := e.do(e.authed("GET", "/unpair", nil, c)); w.Code == http.StatusNoContent {
		t.Fatal("GET must not unpair")
	}
}

// ---- /ca.crt ----

func TestCACertOnlyWhilePairingOrPaired(t *testing.T) {
	e := newEnv(t, 0)
	if w := e.do(e.req("GET", "/ca.crt", nil)); w.Code != http.StatusNotFound {
		t.Fatalf("closed: %d", w.Code)
	}
	e.a.OpenPairing()
	w := e.do(e.req("GET", "/ca.crt", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "BEGIN CERTIFICATE") || w.Header().Get("Content-Type") != "application/x-x509-ca-cert" {
		t.Fatalf("open: %d %v", w.Code, w.Header())
	}
	c := e.pair() // closes pairing
	if w := e.do(e.req("GET", "/ca.crt", nil)); w.Code != http.StatusNotFound {
		t.Fatalf("after pairing, anonymous: %d", w.Code)
	}
	if w := e.do(e.authed("GET", "/ca.crt", nil, c)); w.Code != 200 {
		t.Fatalf("after pairing, with session: %d", w.Code)
	}
}

// ---- static shell ----

func TestStaticShellPublicWithHeaders(t *testing.T) {
	e := newEnv(t, 0)
	w := e.do(e.req("GET", "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<html") {
		t.Fatalf("%d", w.Code)
	}
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy", "X-Frame-Options"} {
		if w.Header().Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
	for _, p := range []string{"/../server.go", "/..%2fserver.go", "/server.go", "/web/index.html"} {
		w := e.do(e.req("GET", p, nil))
		if w.Code == 200 && strings.Contains(w.Body.String(), "package server") {
			t.Errorf("%s leaked source", p)
		}
	}
}

func TestStatus(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	w := e.do(e.authed("GET", "/status", nil, c))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"paired":true`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

// ---- localhost-only /qr and /pairing ----

func local(e *env, method, target string) *http.Request {
	r := e.req(method, target, nil)
	r.RemoteAddr = "127.0.0.1:50000"
	r.Host = "localhost:8443"
	return r
}

func TestQRAndPairingPageLocalOnly(t *testing.T) {
	e := newEnv(t, 0)
	pairURL := e.s.StartPairing()
	u, err := url.Parse(pairURL)
	if err != nil || u.Host != "192.168.1.5:8443" || u.Path != "/pair" || len(u.Query().Get("t")) != 64 {
		t.Fatalf("pair URL %q", pairURL)
	}
	for _, p := range []string{"/qr", "/pairing", "/pairing/status"} {
		r := e.req("GET", p, nil) // from the LAN
		if w := e.do(r); w.Code != http.StatusForbidden {
			t.Errorf("%s from LAN: %d", p, w.Code)
		}
		r = local(e, "GET", p)
		r.Host = "evil.example:8443" // DNS rebinding
		if w := e.do(r); w.Code != http.StatusForbidden {
			t.Errorf("%s with foreign Host: %d", p, w.Code)
		}
	}
	w := e.do(local(e, "GET", "/qr"))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("qr: %d %v", w.Code, w.Header())
	}
	if _, err := png.Decode(w.Body); err != nil {
		t.Fatal("qr is not a PNG")
	}
	w = e.do(local(e, "GET", "/pairing"))
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, `src="/qr`) || !strings.Contains(w.Header().Get("Content-Security-Policy"), "nonce-") {
		t.Fatalf("pairing page: %d", w.Code)
	}
	w = e.do(local(e, "GET", "/pairing/status"))
	if !strings.Contains(w.Body.String(), `"open":true`) {
		t.Fatalf("status: %s", w.Body)
	}
	// After the phone pairs, the QR is gone.
	e.do(e.req("GET", u.RequestURI(), nil))
	if w := e.do(local(e, "GET", "/qr")); w.Code != http.StatusGone {
		t.Fatalf("qr after pairing: %d", w.Code)
	}
	w = e.do(local(e, "GET", "/pairing/status"))
	if !strings.Contains(w.Body.String(), `"paired":true`) {
		t.Fatalf("status: %s", w.Body)
	}
}

func TestPairingOpenRequiresSameOrigin(t *testing.T) {
	e := newEnv(t, 0)
	r := local(e, "POST", "/pairing/open")
	r.Header.Set("Origin", "https://evil.example")
	if w := e.do(r); w.Code != http.StatusForbidden || e.a.PairingOpen() {
		t.Fatalf("cross-origin open: %d", w.Code)
	}
	r = local(e, "POST", "/pairing/open")
	r.Header.Set("Origin", "https://localhost:8443")
	if w := e.do(r); w.Code != http.StatusNoContent || !e.a.PairingOpen() {
		t.Fatalf("same-origin open: %d", w.Code)
	}
}

// ---- static: favicon and font MIME ----

func TestFaviconAndWoff2(t *testing.T) {
	e := newEnv(t, 0)
	w := e.do(e.req("GET", "/favicon.ico", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Body.Len() == 0 {
		t.Fatalf("favicon: %d %q len=%d", w.Code, w.Header().Get("Content-Type"), w.Body.Len())
	}
	w = e.do(e.req("GET", "/bricolage.woff2", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "font/woff2" {
		t.Fatalf("woff2: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
}
