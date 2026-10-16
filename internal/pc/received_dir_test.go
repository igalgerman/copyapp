//go:build windows

package pc

import (
	"path/filepath"
	"testing"

	"github.com/igalgerman/copyapp/internal/settings"
)

func TestRedirectedDownloadsUsesSafeLocalFallback(t *testing.T) {
	local := t.TempDir()
	want := filepath.Join(local, AppName, "Received")
	if got := chooseReceivedDir(`\\server\redirected-downloads`, local); got != want {
		t.Fatalf("network Downloads fallback = %q, want %q", got, want)
	}
	denied := t.TempDir()
	old := settings.ExtraDenied
	settings.ExtraDenied = append(append([]string{}, old...), denied)
	t.Cleanup(func() { settings.ExtraDenied = old })
	if got := chooseReceivedDir(denied, local); got != want {
		t.Fatalf("Startup Downloads fallback = %q, want %q", got, want)
	}
	if got := chooseReceivedDir(denied, denied); got != "" {
		t.Fatalf("unsafe fallback accepted: %q", got)
	}
}
