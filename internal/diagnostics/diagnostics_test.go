package diagnostics

import (
	"encoding/json"
	"github.com/getsentry/sentry-go"
	"strings"
	"testing"
)

func TestValidateDSNRejectsCredentialAndCSPInjection(t *testing.T) {
	for _, dsn := range []string{
		"http://abc@o1.ingest.sentry.io/123",
		"https://abc:secret@o1.ingest.sentry.io/123",
		"https://abc@evil.example/123",
		"https://abc@o1.ingest.sentry.io.evil.example/123",
		"https://abc@o1.ingest.sentry.io/123?token=secret",
		"https://abc@o1.ingest.sentry.io/123#secret",
		"https://abc@o1.ingest.sentry.io:8443/123",
		"https://abc@o1.ingest.sentry.io/project",
	} {
		if _, err := ValidateDSN(dsn); err == nil {
			t.Errorf("accepted unsafe DSN %q", dsn)
		}
	}
	for _, dsn := range []string{"https://abc@o1.ingest.sentry.io/123", "https://abc@o1.ingest.us.sentry.io/123", "https://abc@o1.ingest.de.sentry.io/123"} {
		origin, err := ValidateDSN(dsn)
		if err != nil || strings.Contains(origin, "@") || strings.Contains(origin, "/123") {
			t.Errorf("unexpected origin %q: %v", origin, err)
		}
	}
}

func TestStableSupportIDAndFreshSessionOffline(t *testing.T) {
	t.Setenv("COPYAPP_SENTRY_DSN", "")
	oldDSN := BuildDSN
	BuildDSN = ""
	defer func() { BuildDSN = oldDSN; current.Store(nil) }()
	dir := t.TempDir()
	a, err := Start(dir, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Start(dir, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.id != b.id || a.session == b.session || !idRE.MatchString(a.id) {
		t.Fatal("installation/session identity lifetime incorrect")
	}
	if b.logger != nil || BrowserConfig()["dsn"] != "" {
		t.Fatal("offline diagnostics unexpectedly enabled")
	}
}

func TestSanitizeEventUsesCurrentSDKPrivacyFields(t *testing.T) {
	e := &sentry.Event{
		Message:     "application panic (details retained locally)",
		Request:     &sentry.Request{URL: "https://example.invalid/?token=SECRET"},
		User:        sentry.User{Email: "SECRET@example.invalid"},
		Contexts:    map[string]sentry.Context{"extra": {"clipboard": "SECRET"}},
		Breadcrumbs: []*sentry.Breadcrumb{{Message: "SECRET"}},
		Attachments: []*sentry.Attachment{{Filename: "SECRET"}},
		Threads:     []sentry.Thread{{Name: "SECRET"}},
		Exception:   []sentry.Exception{{Value: "SECRET"}},
		ServerName:  "SECRET",
	}
	got := sanitizeEvent(e, nil)
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "SECRET") {
		t.Fatal("event retained sensitive metadata")
	}
	if len(got.Attachments) != 0 {
		t.Fatal("event retained binary attachments")
	}
	if got.Exception[0].Value != "redacted" || got.ServerName != "copyapp" {
		t.Fatal("event sanitization incomplete")
	}
}
