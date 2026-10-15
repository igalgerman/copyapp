package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/igalgerman/copyapp/internal/settings"
	"github.com/igalgerman/copyapp/internal/store"
)

type sendReply struct {
	OK     bool     `json:"ok"`
	Kind   string   `json:"kind"`
	To     string   `json:"where"`
	Saved  []string `json:"saved"`
	Folder string   `json:"folder"`
	Code   string   `json:"code"`
	Error  string   `json:"error"`
}

func decodeReply(t *testing.T, b *bytes.Buffer) sendReply {
	t.Helper()
	var r sendReply
	if err := json.Unmarshal(b.Bytes(), &r); err != nil {
		t.Fatalf("reply %q: %v", b, err)
	}
	return r
}

func (e *env) setMode(m string) {
	e.s.cfg.Settings.Update(func(s *settings.Settings) { s.ReceiveMode = m })
}

func TestClipboardSettingRoutesItems(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	w := e.sendJSON(c, map[string]string{"text": "hi"})
	if r := decodeReply(t, w.Body); w.Code != 200 || r.Kind != "text" || r.To != "clipboard" || len(r.Saved) != 0 {
		t.Fatalf("text: %d %s", w.Code, w.Body)
	}
	w = e.sendMultipart(c, part{field: "files", filename: "p.png", ctype: "image/png", data: pngBytes(), isFile: true})
	if r := decodeReply(t, w.Body); r.Kind != "image" || r.To != "clipboard" {
		t.Fatalf("png: %s", w.Body)
	}
	w = e.sendMultipart(c, part{field: "files", filename: "doc.pdf", ctype: "application/pdf", data: []byte("%PDF"), isFile: true})
	if r := decodeReply(t, w.Body); r.Kind != "file" || r.To != "folder" || len(r.Saved) != 1 || r.Saved[0] != "doc.pdf" || r.Folder != e.dir {
		t.Fatalf("pdf: %s", w.Body)
	}
	if n, m := e.clip.calls(); n != 1 || m != 1 {
		t.Fatalf("clipboard calls %d %d", n, m)
	}
}

func TestFolderSettingSavesEverything(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	e.setMode(settings.ModeFolder)
	w := e.sendJSON(c, map[string]string{"text": "note from phone"})
	r := decodeReply(t, w.Body)
	if w.Code != 200 || r.To != "folder" || len(r.Saved) != 1 || !strings.HasSuffix(r.Saved[0], ".txt") {
		t.Fatalf("text: %d %s", w.Code, w.Body)
	}
	if b, _ := os.ReadFile(filepath.Join(e.dir, r.Saved[0])); string(b) != "note from phone" {
		t.Fatalf("saved text %q", b)
	}
	w = e.sendMultipart(c, part{field: "file", filename: "shot.png", ctype: "image/png", data: pngBytes(), isFile: true})
	if r := decodeReply(t, w.Body); r.To != "folder" || len(r.Saved) != 1 || r.Saved[0] != "shot.png" {
		t.Fatalf("png: %s", w.Body)
	}
	// Share-target text field in folder mode.
	w = e.sendMultipart(c, part{field: "title", data: []byte("T")}, part{field: "url", data: []byte("https://x.example")})
	if r := decodeReply(t, w.Body); r.To != "folder" || len(r.Saved) != 1 {
		t.Fatalf("shared link: %s", w.Body)
	}
	if n, m := e.clip.calls(); n+m != 0 {
		t.Fatal("folder mode touched the clipboard")
	}
	// The phone can still ask for the clipboard explicitly.
	w = e.sendMultipart(c, part{field: "mode", data: []byte("clipboard")}, part{field: "file", filename: "c.png", ctype: "image/png", data: pngBytes(), isFile: true})
	if r := decodeReply(t, w.Body); r.To != "clipboard" || r.Kind != "image" {
		t.Fatalf("explicit clipboard: %s", w.Body)
	}
}

func TestExplicitModes(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	// mode=file overrides the clipboard setting, also for JSON text.
	w := e.sendJSON(c, map[string]string{"text": "x", "mode": "file"})
	if r := decodeReply(t, w.Body); r.To != "folder" {
		t.Fatalf("json mode=file: %s", w.Body)
	}
	e.setMode(settings.ModeFolder)
	w = e.sendMultipart(c, part{field: "mode", data: []byte("auto")}, part{field: "file", filename: "a.png", ctype: "image/png", data: pngBytes(), isFile: true})
	if r := decodeReply(t, w.Body); r.To != "folder" {
		t.Fatalf("auto follows the PC setting: %s", w.Body)
	}
	w = e.sendJSON(c, map[string]string{"text": "x", "mode": "bogus"})
	if w.Code != 400 {
		t.Fatalf("bad mode: %d", w.Code)
	}
	// Explicit clipboard is also valid for text and Android share-target URLs.
	w = e.sendMultipart(c, part{field: "mode", data: []byte("clipboard")}, part{field: "text", data: []byte("clipboard note")})
	if r := decodeReply(t, w.Body); w.Code != 200 || r.To != "clipboard" || r.Kind != "text" {
		t.Fatalf("multipart clipboard text: %d %s", w.Code, w.Body)
	}
	w = e.sendMultipart(c, part{field: "mode", data: []byte("clipboard")}, part{field: "url", data: []byte("https://example.com")})
	if r := decodeReply(t, w.Body); w.Code != 200 || r.To != "clipboard" || r.Kind != "text" {
		t.Fatalf("multipart clipboard URL: %d %s", w.Code, w.Body)
	}
}

// A PNG over the in-memory clipboard cap is streamed to disk, never buffered.
func TestBigPNGWithoutModeIsSaved(t *testing.T) {
	old := clipboardCap
	clipboardCap = 64
	defer func() { clipboardCap = old }()
	e := newEnv(t, 0)
	c := e.pair()
	big := append(pngBytes(), bytes.Repeat([]byte{0}, 200)...)
	w := e.sendMultipart(c, part{field: "file", filename: "big.png", ctype: "image/png", data: big, isFile: true})
	if r := decodeReply(t, w.Body); w.Code != 200 || r.To != "folder" || r.Saved[0] != "big.png" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if b, _ := os.ReadFile(filepath.Join(e.dir, "big.png")); !bytes.Equal(b, big) {
		t.Fatal("content")
	}
	w = e.sendMultipart(c, part{field: "mode", data: []byte("clipboard")}, part{field: "file", filename: "b.png", ctype: "image/png", data: big, isFile: true})
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("explicit clipboard over cap: %d", w.Code)
	}
}

// zeroReader yields n zero bytes without allocating them.
type zeroReader struct{ n int64 }

func (z *zeroReader) Read(p []byte) (int, error) {
	if z.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > z.n {
		p = p[:z.n]
	}
	clear(p)
	z.n -= int64(len(p))
	return len(p), nil
}

// streamBody builds a multipart body on the fly from a generator.
func streamBody(t *testing.T, name string, src io.Reader, failAfter bool) (string, io.Reader) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		fw, err := mw.CreateFormFile("file", name)
		if err == nil {
			_, err = io.Copy(fw, src)
		}
		if failAfter {
			pw.CloseWithError(errors.New("client went away"))
			return
		}
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()
	return mw.FormDataContentType(), pr
}

func TestHugeUploadStreamsToDisk(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 300 MB")
	}
	e := newEnv(t, 0) // default limit: 2 GiB
	c := e.pair()
	const size = 300 << 20
	ct, body := streamBody(t, "huge.bin", &zeroReader{n: size}, false)
	r := e.authed("POST", "/send", body, c)
	r.Header.Set("Content-Type", ct)
	r.ContentLength = -1
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	w := e.do(r)
	runtime.ReadMemStats(&after)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 64<<20 {
		t.Fatalf("upload allocated %d MB: it is being buffered", alloc>>20)
	}
	fi, err := os.Stat(filepath.Join(e.dir, "huge.bin"))
	if err != nil || fi.Size() != size {
		t.Fatalf("saved: %v %v", fi, err)
	}
	if len(e.s.Transfers()) != 0 {
		t.Fatal("finished transfer still listed")
	}
}

func TestCancelledUploadLeavesNoFile(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	ct, body := streamBody(t, "half.bin", &zeroReader{n: 3 << 20}, true)
	r := e.authed("POST", "/send", body, c)
	r.Header.Set("Content-Type", ct)
	w := e.do(r)
	if w.Code == 200 {
		t.Fatalf("cancelled upload reported ok: %s", w.Body)
	}
	if f := e.savedFiles(); len(f) != 0 {
		t.Fatalf("partial file left: %v", f)
	}
	if len(e.s.Transfers()) != 0 {
		t.Fatal("cancelled transfer still listed")
	}
}

// slowReader lets the test look at the registry mid-upload.
type gateReader struct {
	r    io.Reader
	gate chan struct{}
	half int64
	n    int64
}

func (g *gateReader) Read(p []byte) (int, error) {
	if g.n >= g.half && g.gate != nil {
		<-g.gate
		g.gate = nil
	}
	if g.n < g.half && int64(len(p)) > g.half-g.n {
		p = p[:g.half-g.n]
	}
	n, err := g.r.Read(p)
	g.n += int64(n)
	return n, err
}

func TestTransferProgressIsVisible(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	gate := make(chan struct{})
	ct, body := streamBody(t, "movie.mp4", &zeroReader{n: 4 << 20}, false)
	r := e.authed("POST", "/send", &gateReader{r: body, gate: gate, half: 2 << 20}, c)
	r.Header.Set("Content-Type", ct)
	r.ContentLength = 4<<20 + 300
	done := make(chan int)
	go func() { done <- e.do(r).Code }()
	var tr []Transfer
	for i := 0; i < 200; i++ {
		if tr = e.s.Transfers(); len(tr) == 1 && tr[0].Done >= 2<<20 && tr[0].Name == "movie.mp4" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(tr) != 1 || tr[0].Dir != "in" || tr[0].Name != "movie.mp4" || tr[0].Total != 4<<20+300 || tr[0].Done < 2<<20 {
		t.Fatalf("mid-upload registry: %+v", tr)
	}
	close(gate)
	if code := <-done; code != 200 {
		t.Fatalf("%d", code)
	}
	if len(e.s.Transfers()) != 0 {
		t.Fatal("still listed")
	}
}

func TestTooLargeMessageNamesLimit(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	e.s.cfg.Settings.Update(func(s *settings.Settings) { s.MaxBytes = 200 << 20 })
	r := e.authed("POST", "/send", failReader{t}, c)
	r.ContentLength = 300 << 20
	r.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	w := e.do(r)
	if rep := decodeReply(t, w.Body); w.Code != 413 || rep.Code != "too_large" || !strings.Contains(rep.Error, "200 MB") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestPathItemServedWithRange(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	p := filepath.Join(t.TempDir(), "report.pdf")
	os.WriteFile(p, []byte("0123456789"), 0o644)
	id, err := e.s.Publish(store.Item{Kind: "file", Name: "report.pdf", Mime: "application/pdf", Path: p})
	if err != nil {
		t.Fatal(err)
	}
	w := e.do(e.authed("GET", "/item/"+id, nil, c))
	if w.Code != 200 || w.Body.String() != "0123456789" || w.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("full: %d %q %v", w.Code, w.Body, w.Header())
	}
	r := e.authed("GET", "/item/"+id, nil, c)
	r.Header.Set("Range", "bytes=2-5")
	w = e.do(r)
	if w.Code != http.StatusPartialContent || w.Body.String() != "2345" || w.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range: %d %q %v", w.Code, w.Body, w.Header())
	}
	if !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("headers: %v", w.Header())
	}
	// SSE reports the file size.
	if msg := string(sseMessage(mustGet(t, e, id))); !strings.Contains(msg, `"size":10`) {
		t.Fatalf("sse: %s", msg)
	}
	// The user's file disappears: 404, not a crash.
	os.Remove(p)
	if w := e.do(e.authed("GET", "/item/"+id, nil, c)); w.Code != 404 {
		t.Fatalf("gone: %d", w.Code)
	}
}

func mustGet(t *testing.T, e *env, id string) store.Item {
	it, ok := e.st.Get(id)
	if !ok {
		t.Fatal("no item " + id)
	}
	return it
}

func TestStatusAdvertisesLimits(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	var st map[string]any
	json.Unmarshal(e.do(e.authed("GET", "/status", nil, c)).Body.Bytes(), &st)
	if st["maxBytes"] != float64(2<<30) || st["receiveMode"] != "clipboard" {
		t.Fatalf("%v", st)
	}
}

func TestPublishHonorsConfiguredLimit(t *testing.T) {
	e := newEnv(t, 0)
	e.s.cfg.Settings.Update(func(s *settings.Settings) { s.MaxBytes = 8 })
	if _, err := e.s.Publish(store.Item{Kind: "text", Data: []byte("123456789")}); !errors.Is(err, store.ErrTooLarge) {
		t.Fatalf("memory limit: %v", err)
	}
	p := filepath.Join(t.TempDir(), "large.bin")
	os.WriteFile(p, []byte("123456789"), 0o600)
	if _, err := e.s.Publish(store.Item{Kind: "file", Path: p}); !errors.Is(err, store.ErrTooLarge) {
		t.Fatalf("disk limit: %v", err)
	}
	os.WriteFile(p, []byte("12345678"), 0o600)
	id, err := e.s.Publish(store.Item{Kind: "file", Path: p})
	if err != nil {
		t.Fatal(err)
	}
	// A file that grows after being queued must not bypass the current limit.
	os.WriteFile(p, []byte("123456789"), 0o600)
	c := e.pair()
	if w := e.do(e.authed("GET", "/item/"+id, nil, c)); w.Code != 413 {
		t.Fatalf("grown file: %d", w.Code)
	}
}

func TestHumanSize(t *testing.T) {
	for in, want := range map[int64]string{200 << 20: "200 MB", 500 << 20: "500 MB", 1 << 30: "1 GB", 2 << 30: "2 GB", 1024: "1 KB"} {
		if got := humanSize(in); got != want {
			t.Errorf("%d: %q want %q", in, got, want)
		}
	}
	_ = fmt.Sprint
}
