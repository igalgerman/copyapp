//go:build windows

package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func tempPaths(t *testing.T) Paths {
	d := t.TempDir()
	return Paths{
		InstallDir: filepath.Join(d, "Programs", "CopyApp"),
		Desktop:    filepath.Join(d, "Desktop"),
		StartMenu:  filepath.Join(d, "Start"),
		SendTo:     filepath.Join(d, "SendTo"),
	}
}

func TestDefaultHonoursOverrides(t *testing.T) {
	t.Setenv("COPYAPP_INSTALL_DIR", `C:\x\inst`)
	t.Setenv("COPYAPP_DESKTOP_DIR", `C:\x\desk`)
	t.Setenv("COPYAPP_STARTMENU_DIR", `C:\x\start`)
	t.Setenv("COPYAPP_SENDTO_DIR", `C:\x\sendto`)
	p := Default()
	if p.Exe() != `C:\x\inst\copyapp.exe` || p.Desktop != `C:\x\desk` || p.StartMenu != `C:\x\start` || p.SendTo != `C:\x\sendto` {
		t.Fatalf("%+v", p)
	}
	if l := p.Links(); len(l) != 3 || l[2] != `C:\x\sendto\Phone (CopyApp).lnk` {
		t.Fatalf("%v", l)
	}
}

func TestRealDefaultIsLocalAppData(t *testing.T) {
	for _, k := range []string{"COPYAPP_INSTALL_DIR", "COPYAPP_DESKTOP_DIR", "COPYAPP_STARTMENU_DIR", "COPYAPP_SENDTO_DIR"} {
		t.Setenv(k, "")
	}
	p := Default()
	want := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "CopyApp")
	if !SamePath(p.InstallDir, want) {
		t.Fatalf("%s want %s", p.InstallDir, want)
	}
}

func TestSamePathAndInside(t *testing.T) {
	if !SamePath(`C:\A\b\..\B\x.EXE`, `c:\a\B\x.exe`) || SamePath(`C:\a\x.exe`, `C:\a\y.exe`) {
		t.Fatal("SamePath")
	}
	if !Inside(`C:\Prog\CopyApp\copyapp.exe`, `c:\prog\copyapp`) || Inside(`C:\Prog\CopyAppX\a`, `C:\Prog\CopyApp`) {
		t.Fatal("Inside")
	}
}

func TestInstallCopiesAndCreatesShortcuts(t *testing.T) {
	p := tempPaths(t)
	src := filepath.Join(t.TempDir(), "copyapp.exe")
	os.WriteFile(src, []byte("version 1"), 0o755)
	copied, err := Install(p, src)
	if err != nil || !copied {
		t.Fatalf("%v %v", copied, err)
	}
	if !SameFile(src, p.Exe()) {
		t.Fatal("not copied")
	}
	for _, l := range p.Links() {
		if fi, err := os.Stat(l); err != nil || fi.Size() == 0 {
			t.Fatalf("shortcut %s: %v", l, err)
		}
	}
	// Same content: no copy. New content: replaced.
	if copied, _ := Install(p, src); copied {
		t.Fatal("identical exe copied again")
	}
	os.WriteFile(src, []byte("version 2"), 0o755)
	if copied, err := Install(p, src); err != nil || !copied || !SameFile(src, p.Exe()) {
		t.Fatalf("upgrade: %v %v", copied, err)
	}
	if err := RemoveShortcuts(p); err != nil {
		t.Fatal(err)
	}
	for _, l := range p.Links() {
		if _, err := os.Stat(l); !os.IsNotExist(err) {
			t.Fatalf("%s still there", l)
		}
	}
	if err := RemoveShortcuts(p); err != nil {
		t.Fatal("removing twice must be fine")
	}
}

func TestSafeToDelete(t *testing.T) {
	h, _ := os.UserHomeDir()
	for _, bad := range []string{`C:\`, `C:`, `relative\dir`, h, os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"), `C:\a`} {
		if SafeToDelete(bad) {
			t.Errorf("%q considered safe", bad)
		}
	}
	for _, ok := range []string{filepath.Join(os.Getenv("APPDATA"), "CopyApp"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "CopyApp"), `D:\data\CopyApp`} {
		if !SafeToDelete(ok) {
			t.Errorf("%q refused", ok)
		}
	}
}

func TestScheduleDeleteWaitsForProcess(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "inst")
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "f.txt"), []byte("x"), 0o644)
	cmd := exec.Command("ping", "-n", "3", "127.0.0.1") // ~2 s
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := ScheduleDelete(cmd.Process.Pid, []string{dir}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("deleted before the process exited")
	}
	cmd.Wait()
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("not deleted after exit")
}

func TestScheduleDeleteRefusesUnsafe(t *testing.T) {
	if err := ScheduleDelete(os.Getpid(), []string{`C:\`}); err == nil {
		t.Fatal("root accepted")
	}
}

func TestUninstallPreservesUnrelatedAndReceivedFiles(t *testing.T) {
	p := tempPaths(t)
	d := filepath.Join(t.TempDir(), "CopyApp")
	for _, dir := range []string{p.InstallDir, d} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{p.Exe(), filepath.Join(p.InstallDir, "personal.txt"), filepath.Join(d, "settings.json"), filepath.Join(d, "received.txt")} {
		if err := os.WriteFile(f, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("ping", "-n", "2", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := ScheduleUninstall(cmd.Process.Pid, p, d, true); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()
	for i := 0; i < 50; i++ {
		_, exeErr := os.Stat(p.Exe())
		_, settingsErr := os.Stat(filepath.Join(d, "settings.json"))
		if os.IsNotExist(exeErr) && os.IsNotExist(settingsErr) {
			for _, f := range []string{filepath.Join(p.InstallDir, "personal.txt"), filepath.Join(d, "received.txt")} {
				if _, err := os.Stat(f); err != nil {
					t.Fatalf("user file deleted: %s: %v", f, err)
				}
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("uninstall did not remove application files")
}
