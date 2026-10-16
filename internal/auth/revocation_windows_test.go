//go:build windows

package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/igalgerman/copyapp/internal/secret"
	"golang.org/x/sys/windows"
)

func lockSession(t *testing.T, path string) func() {
	t.Helper()
	p, _ := windows.UTF16PtrFromString(path)
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return func() { windows.CloseHandle(h) }
}

func persisted(t *testing.T) (*Auth, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	a := New()
	if err := a.Persist(path); err != nil {
		t.Fatal(err)
	}
	sid, ok, err := a.ExchangeWithError(a.OpenPairing())
	if err != nil || !ok {
		t.Fatalf("pair: %v %v", ok, err)
	}
	return a, path, sid
}

func TestRevocationSurvivesLockedOrReadOnlySession(t *testing.T) {
	for _, kind := range []string{"locked", "read-only"} {
		t.Run(kind, func(t *testing.T) {
			a, path, old := persisted(t)
			var release func()
			if kind == "locked" {
				release = lockSession(t, path)
			} else {
				if err := os.Chmod(path, 0o400); err != nil {
					t.Fatal(err)
				}
				release = func() { os.Chmod(path, 0o600) }
			}
			defer func() { release() }()
			if err := a.Revoke(); err != nil {
				t.Fatalf("durable revoke: %v", err)
			}
			if a.Valid(old) {
				t.Fatal("revoked session valid in memory")
			}
			if _, err := os.Stat(path); kind == "locked" && err != nil {
				t.Fatalf("fixture should prevent deletion: %v", err)
			}
			b := New()
			if err := b.Persist(path); err != nil {
				t.Fatal(err)
			}
			if b.Valid(old) || b.Paired() {
				t.Fatal("revoked session restored after restart")
			}
			if kind == "locked" {
				if _, ok, err := b.ExchangeWithError(b.OpenPairing()); err == nil || ok || b.Paired() {
					t.Fatalf("pairing reported success despite unsavable session: ok=%v err=%v", ok, err)
				}
			}
			release()
			// Release is idempotent for chmod; do not close a handle twice.
			release = func() {}
			sid, ok, err := b.ExchangeWithError(b.OpenPairing())
			if err != nil || !ok {
				t.Fatalf("pair after releasing file: %v %v", ok, err)
			}
			c := New()
			if err := c.Persist(path); err != nil || !c.Valid(sid) || c.Valid(old) {
				t.Fatalf("new durable pairing failed: %v", err)
			}
		})
	}
}

func TestRevokeReportsFailureWhenNeitherMarkerNorDeleteCanCommit(t *testing.T) {
	a, path, old := persisted(t)
	release := lockSession(t, path)
	defer release()
	dir := filepath.Dir(path)
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	trustee := windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(u.User.Sid)}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{AccessPermissions: windows.FILE_WRITE_DATA, AccessMode: windows.DENY_ACCESS, Trustee: trustee}, // FILE_ADD_FILE for a directory
		{AccessPermissions: windows.GENERIC_ALL, AccessMode: windows.GRANT_ACCESS, Trustee: trustee},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	defer secret.Restrict(dir)
	if err := a.Revoke(); err == nil {
		t.Fatal("revocation falsely reported durable success")
	}
	if a.Valid(old) {
		t.Fatal("failed persistence must still revoke this process")
	}
}
