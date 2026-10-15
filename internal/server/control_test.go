package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/igalgerman/copyapp/internal/i18n"
	"github.com/igalgerman/copyapp/internal/settings"
)

func localJSON(e *env, method, target, body string) *http.Request {
	r := local(e, method, target)
	if body != "" {
		r.Body = http.NoBody
		r = r.Clone(r.Context())
		r.Body = ioNop(body)
		r.ContentLength = int64(len(body))
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func ioNop(s string) *nopCloser { return &nopCloser{bytes.NewReader([]byte(s))} }

type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }

func getState(t *testing.T, e *env) localState {
	t.Helper()
	w := e.do(local(e, "GET", "/local/state"))
	if w.Code != 200 {
		t.Fatalf("state: %d", w.Code)
	}
	var st localState
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestControlRoutesNeedLocalSecret(t *testing.T) {
	e := newEnv(t, 0)
	for _, rt := range [][2]string{{"GET", "/local/state"}, {"POST", "/local/settings"}, {"POST", "/local/choose-folder"},
		{"POST", "/local/open-folder"}, {"POST", "/local/repair"}, {"POST", "/local/uninstall"}} {
		if w := e.do(bareLocal(e, rt[0], rt[1])); w.Code != http.StatusForbidden {
			t.Errorf("%s without secret: %d", rt[1], w.Code)
		}
		r := e.req(rt[0], rt[1], nil) // from the LAN
		r.Header.Set("X-CopyApp", testLocalKey)
		if w := e.do(r); w.Code != http.StatusForbidden {
			t.Errorf("%s from LAN: %d", rt[1], w.Code)
		}
	}
}

func TestLocalStateReportsStatus(t *testing.T) {
	e := newEnv(t, 0)
	e.s.cfg.WiFiName = func() string { return "Home Wi-Fi" }
	e.s.cfg.LogPath = `C:\x\copyapp.log`
	e.s.cfg.Autostart = func() bool { return true }
	st := getState(t, e)
	if st.Phone.Paired || st.Phone.LastSeen != nil || st.WiFi != "Home Wi-Fi" || st.LogPath != `C:\x\copyapp.log` ||
		!st.Autostart || st.Settings.MaxBytes != 2<<30 || len(st.SizeChoices) != 4 || st.Hotkey != "Ctrl+Alt+C" {
		t.Fatalf("%+v", st)
	}
	c := e.pair()
	e.do(e.authed("GET", "/status", nil, c))
	st = getState(t, e)
	if !st.Phone.Paired || st.Phone.LastSeen == nil || time.Since(*st.Phone.LastSeen) > time.Minute || !st.Pairing.Paired {
		t.Fatalf("after pairing: %+v", st)
	}
}

func TestLocalSettings(t *testing.T) {
	e := newEnv(t, 0)
	defer i18n.Set(i18n.Default)
	var calls int
	var auto bool
	e.s.cfg.OnSettings = func(old, cur settings.Settings) { calls++ }
	e.s.cfg.SetAutostart = func(on bool) error { auto = on; return nil }
	e.s.cfg.Autostart = func() bool { return auto }
	dir := filepath.Join(t.TempDir(), "elsewhere")
	body := `{"language":"ru","receiveMode":"folder","maxBytes":524288000,"autostart":true,"saveDir":` + jsonString(dir) + `}`
	w := e.do(localJSON(e, "POST", "/local/settings", body))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	s := e.s.cfg.Settings.Get()
	if s.Language != "ru" || s.ReceiveMode != "folder" || s.MaxBytes != 500<<20 || s.SaveDir != dir || !auto || calls != 1 || i18n.Lang() != "ru" {
		t.Fatalf("%+v auto=%v calls=%d", s, auto, calls)
	}
	// Uploads now follow the new folder.
	c := e.pair()
	e.sendJSON(c, map[string]string{"text": "x"})
	if m, _ := filepath.Glob(filepath.Join(dir, "*.txt")); len(m) != 1 {
		t.Fatal("not saved to the new folder")
	}
	for _, bad := range []string{`{"maxBytes":123}`, `{"receiveMode":"zip"}`, `{"language":"xx"}`, `{"saveDir":"relative"}`, `{`} {
		if w := e.do(localJSON(e, "POST", "/local/settings", bad)); w.Code != 400 {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}
	// Reset the folder.
	e.do(localJSON(e, "POST", "/local/settings", `{"saveDir":""}`))
	if e.s.cfg.Settings.Get().SaveDir != e.dir {
		t.Fatal("reset")
	}
	// Other web pages cannot change settings.
	r := localJSON(e, "POST", "/local/settings", `{"receiveMode":"clipboard"}`)
	r.Header.Set("Origin", "http://evil.example")
	if w := e.do(r); w.Code != 403 {
		t.Fatalf("cross-origin: %d", w.Code)
	}
	// Scripts with the key and no Origin may.
	r = localJSON(e, "POST", "/local/settings", `{"receiveMode":"clipboard"}`)
	r.Header.Del("Origin")
	r.Header.Set("X-CopyApp", testLocalKey)
	if w := e.do(r); w.Code != 200 {
		t.Fatalf("script: %d", w.Code)
	}
	// Cookie only, no Origin (e.g. a form from a page that strips it): refused.
	r = localJSON(e, "POST", "/local/settings", `{"receiveMode":"clipboard"}`)
	r.Header.Del("Origin")
	if w := e.do(r); w.Code != 403 {
		t.Fatalf("cookie without origin: %d", w.Code)
	}
}

func jsonString(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestChooseFolderRepairUninstall(t *testing.T) {
	e := newEnv(t, 0)
	picked := filepath.Join(t.TempDir(), "picked")
	e.s.cfg.ChooseFolder = func(cur string) (string, error) {
		if cur != e.dir {
			t.Errorf("current %q", cur)
		}
		return picked, nil
	}
	w := e.do(local(e, "POST", "/local/choose-folder"))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"changed":true`) || e.s.cfg.Settings.Get().SaveDir != picked {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	e.s.cfg.ChooseFolder = func(string) (string, error) { return "", nil } // cancelled
	if w := e.do(local(e, "POST", "/local/choose-folder")); !strings.Contains(w.Body.String(), `"changed":false`) || e.s.cfg.Settings.Get().SaveDir != picked {
		t.Fatalf("cancel: %s", w.Body)
	}
	c := e.pair()
	if w := e.do(local(e, "POST", "/local/repair")); w.Code != 204 || e.a.Valid(c.Value) || !e.a.PairingOpen() {
		t.Fatalf("repair: %d", w.Code)
	}
	done := make(chan bool, 1)
	e.s.cfg.Uninstall = func() { done <- true }
	if w := e.do(local(e, "POST", "/local/uninstall")); w.Code != 202 {
		t.Fatalf("uninstall: %d", w.Code)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("uninstall hook not called")
	}
}

func TestControlWindowPage(t *testing.T) {
	e := newEnv(t, 0)
	e.s.cfg.LogPath = `C:\data\copyapp.log`
	e.s.StartPairing()
	body := e.do(local(e, "GET", "/pairing")).Body.String()
	for _, want := range []string{`id="status"`, `id="pair"`, `id="transfers"`, `id="settings"`, `name="mode" value="clipboard" checked`,
		`id="choose"`, `id="maxSel"`, `value="2147483648" selected`, `id="auto"`, `id="uninstall"`, `id="repair"`, `C:\data\copyapp.log`,
		i18n.T("home.mode")} {
		if !strings.Contains(body, want) {
			t.Errorf("control window lacks %q", want)
		}
	}
	if strings.Count(body, "<option value=") != 4+3 {
		t.Errorf("want 4 size and 3 language options")
	}
}

func TestLocalAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://127.0.0.1:5000/local/quit", nil)
	r.RemoteAddr = "127.0.0.1:40000"
	r.Header.Set("X-CopyApp", testLocalKey)
	if !LocalAllowed(r, testLocalKey) {
		t.Fatal("valid request refused")
	}
	r.Header.Set("Origin", "http://evil")
	if LocalAllowed(r, testLocalKey) {
		t.Fatal("origin accepted")
	}
	r.Header.Del("Origin")
	r.RemoteAddr = "192.168.1.9:4000"
	if LocalAllowed(r, testLocalKey) {
		t.Fatal("LAN accepted")
	}
}

func TestSettingsSaveFailureIsReported(t *testing.T) {
	e := newEnv(t, 0)
	dir := t.TempDir()
	st, err := settings.Load(dir, e.dir)
	if err != nil {
		t.Fatal(err)
	}
	e.s.cfg.Settings = st
	// A directory where the settings file belongs simulates a failed replace.
	if err := os.Mkdir(filepath.Join(dir, settings.FileName), 0o700); err != nil {
		t.Fatal(err)
	}
	w := e.do(localJSON(e, "POST", "/local/settings", `{"receiveMode":"folder"}`))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), `"code":"settings"`) {
		t.Fatalf("save failure not reported: %d %s", w.Code, w.Body)
	}
}

func TestSaveDirRejectsNetworkDeviceAndSystemFolders(t *testing.T) {
	e := newEnv(t, 0)
	sys := os.Getenv("SystemRoot")
	if sys == "" {
		sys = `C:\Windows`
	}
	for _, bad := range []string{`\\server\share`, `\\?\C:\x`, `\\.\PhysicalDrive0`, `C:\x\NUL`, filepath.Join(sys, "Temp")} {
		if w := e.do(localJSON(e, "POST", "/local/settings", `{"saveDir":`+jsonString(bad)+`}`)); w.Code != 400 {
			t.Errorf("settings accepted %q: %d", bad, w.Code)
		}
		e.s.cfg.ChooseFolder = func(string) (string, error) { return bad, nil }
		if w := e.do(local(e, "POST", "/local/choose-folder")); w.Code != 400 || !strings.Contains(w.Body.String(), "bad_folder") {
			t.Errorf("picker accepted %q: %d %s", bad, w.Code, w.Body)
		}
		if got := e.s.cfg.Settings.Get().SaveDir; got != e.dir {
			t.Fatalf("save folder changed to %q", got)
		}
	}
}

func TestSendClipboardButton(t *testing.T) {
	e := newEnv(t, 0)
	if w := e.do(local(e, "POST", "/local/send-clipboard")); w.Code != http.StatusNotImplemented {
		t.Fatalf("without hook: %d", w.Code)
	}
	var sources []string
	e.s.cfg.SendClipboard = func(src string) string { sources = append(sources, src); return "empty" }
	w := e.do(local(e, "POST", "/local/send-clipboard"))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"result":"empty"`) || len(sources) != 1 || sources[0] != "window" {
		t.Fatalf("%d %s %v", w.Code, w.Body, sources)
	}
	r := local(e, "POST", "/local/send-clipboard")
	r.Header.Set("Origin", "http://evil.example")
	if w := e.do(r); w.Code != 403 || len(sources) != 1 {
		t.Fatalf("cross-origin: %d", w.Code)
	}
	if w := e.do(bareLocal(e, "POST", "/local/send-clipboard")); w.Code != 403 {
		t.Fatalf("no secret: %d", w.Code)
	}
	if body := e.do(local(e, "GET", "/pairing")).Body.String(); !strings.Contains(body, `id="sendNow"`) {
		t.Fatal("control window lacks the send button")
	}
}
