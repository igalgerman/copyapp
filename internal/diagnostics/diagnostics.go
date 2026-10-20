// Package diagnostics reports bounded, content-free stages directly to hosted Sentry.
// It deliberately does not forward the application's general-purpose local log.
package diagnostics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getsentry/sentry-go"
)

// BuildDSN may be set with -ldflags. The DSN is a public ingestion identifier.
var BuildDSN string
var current atomic.Pointer[Reporter]
var codeRE = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,64}$`)
var projectRE = regexp.MustCompile(`^/[0-9]+$`)
var idRE = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Reporter is safe to call concurrently. Missing/invalid configuration never blocks transfers.
type Reporter struct {
	id, session, version, dsn string
	hub                       *sentry.Hub
	logger                    sentry.Logger
	local                     func(string, ...any)
	mu                        sync.Mutex
	minute                    time.Time
	count                     int
}

func NewOperation() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(b[:])
}

// ValidateDSN accepts hosted Sentry HTTPS ingestion only, with no password/query/fragment.
// Restricting this public setting also makes it safe to include its origin in CSP.
func ValidateDSN(dsn string) (string, error) {
	if dsn == "" {
		return "", nil
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "https" || u.User == nil || u.User.Username() == "" || u.RawQuery != "" || u.Fragment != "" || !projectRE.MatchString(u.Path) {
		return "", errors.New("invalid Sentry DSN")
	}
	if _, ok := u.User.Password(); ok {
		return "", errors.New("Sentry DSN must not contain a secret key")
	}
	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".ingest.sentry.io") && !strings.HasSuffix(host, ".ingest.us.sentry.io") && !strings.HasSuffix(host, ".ingest.de.sentry.io") {
		return "", errors.New("Sentry DSN must use hosted ingestion")
	}
	if u.Port() != "" && u.Port() != "443" {
		return "", errors.New("invalid Sentry port")
	}
	return "https://" + u.Host, nil
}

// Start preserves a random installation ID across launches and creates a new session ID.
// A failed SDK/configuration initialization leaves a usable local-only reporter installed.
func Start(dir, version string, logf func(string, ...any)) (*Reporter, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	r := &Reporter{id: NewOperation(), session: NewOperation(), version: version, local: logf}
	var startupErr error
	if err := os.MkdirAll(dir, 0700); err != nil {
		startupErr = errors.New("diagnostic ID directory unavailable")
	} else {
		path := filepath.Join(dir, "diagnostic-id")
		if b, err := os.ReadFile(path); err == nil && idRE.MatchString(strings.TrimSpace(string(b))) {
			r.id = strings.TrimSpace(string(b))
		} else if err := os.WriteFile(path, []byte(r.id+"\n"), 0600); err != nil {
			startupErr = errors.New("diagnostic ID could not be saved")
		}
	}
	current.Store(r)
	dsn := os.Getenv("COPYAPP_SENTRY_DSN")
	if dsn == "" {
		dsn = BuildDSN
	}
	if _, err := ValidateDSN(dsn); err != nil {
		return r, err
	}
	r.dsn = dsn
	if dsn == "" {
		return r, startupErr
	}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn: dsn, Release: "copyapp@" + version, Environment: "production",
		ServerName: "copyapp", SendDefaultPII: false, MaxBreadcrumbs: -1,
		HTTPClient:    &http.Client{Timeout: 3 * time.Second},
		BeforeSend:    sanitizeEvent,
		BeforeSendLog: func(l *sentry.Log) *sentry.Log { delete(l.Attributes, "sentry.server.address"); return l },
	})
	if err != nil {
		r.dsn = ""
		return r, errors.New("Sentry initialization failed")
	}
	r.hub = sentry.NewHub(client, sentry.NewScope())
	r.logger = sentry.NewLogger(sentry.SetHubOnContext(context.Background(), r.hub))
	Emit("app", "launch", "ok", "")
	return r, startupErr
}

// sanitizeEvent uses the v0.50 Event schema: arbitrary extra values live in
// Contexts; the older Event.Extra field no longer exists. Attachments and
// thread metadata are unnecessary for our content-free crash marker.
func sanitizeEvent(e *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	e.Request = nil
	e.User = sentry.User{}
	e.Contexts = nil
	e.Breadcrumbs = nil
	e.Attachments = nil
	e.Threads = nil
	e.DebugMeta = nil
	e.ServerName = "copyapp"
	for i := range e.Exception {
		e.Exception[i].Value = "redacted"
	}
	return e
}

func SupportID() string {
	if r := current.Load(); r != nil {
		return r.id
	}
	return ""
}
func BrowserConfig() map[string]string {
	if r := current.Load(); r != nil {
		return map[string]string{"dsn": r.dsn, "version": r.version}
	}
	return map[string]string{"dsn": "", "version": ""}
}
func IngestionOrigin() string {
	if r := current.Load(); r != nil {
		origin, _ := ValidateDSN(r.dsn)
		return origin
	}
	return ""
}

// Emit accepts fixed action/stage/result codes, never clipboard contents, errors or URLs.
// Local stages remain available offline. Remote ingestion is capped at 60 stages/minute;
// the SDK has a bounded 1000-log ring buffer and drops overflow/offline data.
func Emit(action, stage, result, operation string) {
	r := current.Load()
	if r == nil {
		return
	}
	if !codeRE.MatchString(action) || !codeRE.MatchString(stage) || !codeRE.MatchString(result) {
		return
	}
	if !idRE.MatchString(operation) {
		operation = ""
	}
	r.local("diagnostic support=%s session=%s operation=%s action=%s stage=%s result=%s", r.id, r.session, operation, action, stage, result)
	if r.logger == nil {
		return
	}
	r.mu.Lock()
	if time.Since(r.minute) >= time.Minute {
		r.minute = time.Now()
		r.count = 0
	}
	if r.count >= 60 {
		r.mu.Unlock()
		return
	}
	r.count++
	r.mu.Unlock()
	entry := r.logger.Info()
	if result != "ok" && result != "queued" && result != "start" && !strings.HasPrefix(result, "http_2") && !strings.HasPrefix(result, "http_3") {
		entry = r.logger.Warn()
	}
	entry.String("app", "copyapp").String("side", "windows").String("version", r.version).
		String("installation_id", r.id).String("support_id", r.id).String("session_id", r.session).
		String("operation_id", operation).String("action", action).String("stage", stage).String("result", result).
		Emit(action + "." + stage + "." + result)
}

// Recover reports a content-free panic marker, flushes briefly, then re-panics.
// Call with defer in main and long-lived application goroutines.
func Recover() {
	if p := recover(); p != nil {
		Emit("app", "panic", "failed", "")
		if r := current.Load(); r != nil && r.hub != nil {
			r.hub.CaptureMessage("application panic (details retained locally)")
			r.hub.Flush(time.Second)
		}
		panic(p)
	}
}
func (r *Reporter) Close() {
	if r != nil && r.hub != nil {
		r.hub.Flush(time.Second)
	}
}
