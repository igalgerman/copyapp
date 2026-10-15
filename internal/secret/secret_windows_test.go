package secret

import (
	"bytes"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestLocalKeyACLCurrentUserOnly(t *testing.T) {
	d := t.TempDir()
	if _, err := NewLocalKey(d); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(filepath.Join(d, LocalKeyFile), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	ctl, _, _ := sd.Control()
	if ctl&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("DACL must not inherit")
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("no DACL: %v", err)
	}
	if dacl.AceCount != 1 {
		t.Fatalf("want exactly one ACE, got %d", dacl.AceCount)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if !sid.Equals(u.User.Sid) {
		t.Fatalf("ACE is for %v, want current user %v", sid, u.User.Sid)
	}
}

func TestDPAPIRoundTrip(t *testing.T) {
	plain := []byte("-----BEGIN EC PRIVATE KEY-----")
	enc, err := DPAPI{}.Protect(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(enc, plain) {
		t.Fatal("not encrypted")
	}
	dec, err := DPAPI{}.Unprotect(enc)
	if err != nil || !bytes.Equal(dec, plain) {
		t.Fatalf("round trip: %q %v", dec, err)
	}
	enc[len(enc)-1] ^= 1
	if _, err := (DPAPI{}).Unprotect(enc); err == nil {
		t.Fatal("tampered blob must fail")
	}
}
