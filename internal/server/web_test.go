package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestServiceWorker runs testdata/sw_test.mjs against web/sw.js (share
// stash, waitUntil caching). Skipped when node is not installed.
func TestServiceWorker(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	out, err := exec.Command(node, "testdata/sw_test.mjs", "web/sw.js").CombinedOutput()
	if err != nil {
		t.Fatalf("sw.js check failed: %v\n%s", err, out)
	}
}

// TestI18nJS runs testdata/i18n_test.mjs: he/ru/en key sets match, plural
// forms, and every key index.html and app.js use exists.
func TestI18nJS(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	out, err := exec.Command(node, "testdata/i18n_test.mjs", "web/i18n.js", "web/index.html", "web/app.js").CombinedOutput()
	if err != nil {
		t.Fatalf("i18n.js check failed: %v\n%s", err, out)
	}
}

// The page loads its strings from i18n.js before app.js (CSP: script-src 'self').
func TestIndexLoadsI18nFirst(t *testing.T) {
	b, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i, a := strings.Index(s, `<script src="/i18n.js" defer>`), strings.Index(s, `<script src="/app.js" defer>`)
	if i < 0 || a < 0 || i > a || strings.Contains(s, "<script>") || !strings.Contains(s, `<html lang="he" dir="rtl">`) {
		t.Fatal("index.html must load /i18n.js before /app.js, without inline scripts, Hebrew RTL by default")
	}
	sw, _ := os.ReadFile("web/sw.js")
	if !strings.Contains(string(sw), `'/i18n.js'`) {
		t.Fatal("sw.js must cache /i18n.js")
	}
}

func TestManifestShareTargetUsesServiceWorkerRoute(t *testing.T) {
	b, err := os.ReadFile("web/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		ShareTarget struct {
			Action, Method, Enctype string
			Params                  struct {
				Files []struct{ Name string }
			}
		} `json:"share_target"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	st := m.ShareTarget
	if st.Action != "/share" || st.Method != "POST" || st.Enctype != "multipart/form-data" ||
		len(st.Params.Files) != 1 || st.Params.Files[0].Name != "files" {
		t.Fatalf("share_target: %+v", st)
	}
}

// Without a service worker the share POST reaches the server, which handles
// it like /send.
func TestShareFallbackRoute(t *testing.T) {
	e := newEnv(t, 0)
	c := e.pair()
	ct, body := multipartBody(t, part{field: "text", data: []byte("shared text")})
	r := e.authed("POST", "/share", strings.NewReader(string(body)), c)
	r.Header.Set("Content-Type", ct)
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	w := e.do(r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/?sent=text" {
		t.Fatalf("%d %q", w.Code, w.Header().Get("Location"))
	}
	if n, _ := e.clip.calls(); n != 1 {
		t.Fatal("text not copied")
	}
	r = e.req("POST", "/share", strings.NewReader(string(body))) // no cookie
	r.Header.Set("Content-Type", ct)
	if w := e.do(r); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated share: %d", w.Code)
	}
}
