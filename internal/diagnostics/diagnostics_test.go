package diagnostics

import (
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
  if _, err := ValidateDSN(dsn); err == nil { t.Errorf("accepted unsafe DSN %q", dsn) }
 }
 for _, dsn := range []string{"https://abc@o1.ingest.sentry.io/123", "https://abc@o1.ingest.us.sentry.io/123", "https://abc@o1.ingest.de.sentry.io/123"} {
  origin, err := ValidateDSN(dsn)
  if err != nil || strings.Contains(origin, "@") || strings.Contains(origin, "/123") { t.Errorf("unexpected origin %q: %v", origin, err) }
 }
}

func TestStableSupportIDAndFreshSessionOffline(t *testing.T) {
 t.Setenv("COPYAPP_SENTRY_DSN", "")
 oldDSN := BuildDSN; BuildDSN = ""; defer func() { BuildDSN = oldDSN; current.Store(nil) }()
 dir := t.TempDir()
 a, err := Start(dir, "test", nil); if err != nil { t.Fatal(err) }
 b, err := Start(dir, "test", nil); if err != nil { t.Fatal(err) }
 if a.id != b.id || a.session == b.session || !idRE.MatchString(a.id) { t.Fatal("installation/session identity lifetime incorrect") }
 if b.logger != nil || BrowserConfig()["dsn"] != "" { t.Fatal("offline diagnostics unexpectedly enabled") }
}
