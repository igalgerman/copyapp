//go:build windows

// Package install puts copyapp.exe in %LOCALAPPDATA%\Programs\CopyApp with
// Desktop, Start menu and Explorer "Send to" shortcuts, and removes them
// again. Every location can be redirected with an environment variable so
// tests never touch the user's real folders:
//
//	COPYAPP_INSTALL_DIR, COPYAPP_DESKTOP_DIR, COPYAPP_STARTMENU_DIR, COPYAPP_SENDTO_DIR
package install

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"encoding/base64"

	"golang.org/x/sys/windows"
)

const (
	ExeName    = "copyapp.exe"
	LinkName   = "CopyApp.lnk"         // Desktop and Start menu
	SendToName = "Phone (CopyApp).lnk" // Explorer "Send to"
)

// Paths are the install locations.
type Paths struct {
	InstallDir, Desktop, StartMenu, SendTo string
}

func known(id *windows.KNOWNFOLDERID, fallback string) string {
	if d, err := windows.KnownFolderPath(id, 0); err == nil && d != "" {
		return d
	}
	return fallback
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// Default returns the real locations, or the COPYAPP_*_DIR overrides.
func Default() Paths {
	h, _ := os.UserHomeDir()
	lad := envOr("LOCALAPPDATA", filepath.Join(h, "AppData", "Local"))
	return Paths{
		InstallDir: envOr("COPYAPP_INSTALL_DIR", filepath.Join(known(windows.FOLDERID_UserProgramFiles, filepath.Join(lad, "Programs")), "CopyApp")),
		Desktop:    envOr("COPYAPP_DESKTOP_DIR", known(windows.FOLDERID_Desktop, filepath.Join(h, "Desktop"))),
		StartMenu:  envOr("COPYAPP_STARTMENU_DIR", known(windows.FOLDERID_Programs, "")),
		SendTo:     envOr("COPYAPP_SENDTO_DIR", known(windows.FOLDERID_SendTo, "")),
	}
}

// Exe is the installed program.
func (p Paths) Exe() string { return filepath.Join(p.InstallDir, ExeName) }

// Links lists the shortcut files CopyApp creates.
func (p Paths) Links() []string {
	var out []string
	for _, l := range [][2]string{{p.Desktop, LinkName}, {p.StartMenu, LinkName}, {p.SendTo, SendToName}} {
		if l[0] != "" {
			out = append(out, filepath.Join(l[0], l[1]))
		}
	}
	return out
}

func norm(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		p = a
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return strings.ToLower(filepath.Clean(p))
}

// SamePath compares Windows paths (case-insensitive, cleaned, links resolved).
func SamePath(a, b string) bool { return norm(a) == norm(b) }

// Inside reports whether path is dir or below it.
func Inside(path, dir string) bool {
	p, d := norm(path), norm(dir)
	return p == d || strings.HasPrefix(p, strings.TrimRight(d, `\`)+`\`)
}

// FileHash is the SHA-256 of a file, hex.
func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SameFile reports whether both files exist with identical content.
func SameFile(a, b string) bool {
	ha, err1 := FileHash(a)
	hb, err2 := FileHash(b)
	return err1 == nil && err2 == nil && ha == hb
}

// copyFile writes dst atomically (temp file + rename). A running dst is
// locked, so it retries for up to wait.
func copyFile(src, dst string, wait time.Duration) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	deadline := time.Now().Add(wait)
	for {
		err = os.Rename(tmp, dst) // MoveFileEx with REPLACE_EXISTING
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// Install copies src to p.Exe() (unless identical) and (re)creates the
// shortcuts. It reports whether the exe was replaced.
func Install(p Paths, src string) (copied bool, err error) {
	if copied, err = CopyExe(p, src); err != nil {
		return copied, err
	}
	return copied, CreateShortcuts(p)
}

// CopyExe puts src at p.Exe() (unless identical). It is the fast part of
// Install; the installed copy then creates the shortcuts itself, so the user
// sees the window sooner.
func CopyExe(p Paths, src string) (copied bool, err error) {
	if err := os.MkdirAll(p.InstallDir, 0o755); err != nil {
		return false, err
	}
	if SameFile(src, p.Exe()) {
		return false, nil
	}
	if err := copyFile(src, p.Exe(), 15*time.Second); err != nil {
		return false, fmt.Errorf("copy to %s: %w", p.InstallDir, err)
	}
	return true, nil
}

const shortcutScript = `$ErrorActionPreference='Stop'
$sh=New-Object -ComObject WScript.Shell
foreach($l in ($env:CA_LINKS -split '\|')){
 if(-not $l){continue}
 [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($l)) | Out-Null
 $s=$sh.CreateShortcut($l)
 $s.TargetPath=$env:CA_EXE
 $s.WorkingDirectory=[IO.Path]::GetDirectoryName($env:CA_EXE)
 $s.IconLocation=$env:CA_EXE+',0'
 $s.Description='CopyApp'
 if([IO.Path]::GetFileName($l) -eq 'CopyApp.lnk'){$s.Arguments='--open'}else{$s.Arguments=''}
 $s.Save()
}`

func psCmd(script string) *exec.Cmd {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	cmd.Dir = os.TempDir() // never keep the install folder busy
	return cmd
}

// CreateShortcuts writes the Desktop, Start menu and "Send to" shortcuts,
// all pointing at the installed exe.
func CreateShortcuts(p Paths) error {
	cmd := psCmd(shortcutScript)
	cmd.Env = append(os.Environ(), "CA_EXE="+p.Exe(), "CA_LINKS="+strings.Join(p.Links(), "|"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("shortcuts: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveShortcuts deletes the shortcuts (missing ones are fine).
func RemoveShortcuts(p Paths) error {
	var errs []error
	for _, l := range p.Links() {
		if err := os.Remove(l); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// SafeToDelete refuses drive roots, very short paths and well-known user
// folders, so a bad setting can never wipe something important.
func SafeToDelete(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	c := filepath.Clean(path)
	vol := filepath.VolumeName(c)
	rest := strings.Trim(strings.TrimPrefix(c, vol), `\/`)
	if rest == "" || !strings.Contains(rest, `\`) && len(rest) < 3 {
		return false
	}
	h, _ := os.UserHomeDir()
	for _, bad := range []string{h, filepath.Dir(h), os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"), os.Getenv("USERPROFILE"),
		os.Getenv("SystemRoot"), os.Getenv("ProgramFiles"), os.Getenv("TEMP"),
		known(windows.FOLDERID_Desktop, ""), known(windows.FOLDERID_Downloads, ""), known(windows.FOLDERID_Documents, ""),
		known(windows.FOLDERID_UserProgramFiles, "")} {
		if bad != "" && SamePath(c, bad) {
			return false
		}
	}
	return true
}

const deleteScript = `$ErrorActionPreference='SilentlyContinue'
try{Wait-Process -Id ([int]$env:CA_PID) -Timeout 60}catch{}
if(Get-Process -Id ([int]$env:CA_PID) -ErrorAction SilentlyContinue){exit}
Start-Sleep -Milliseconds 700
foreach($p in ($env:CA_DEL -split '\|')){
 if(-not $p){continue}
 for($i=0;$i -lt 20 -and (Test-Path -LiteralPath $p);$i++){
  Remove-Item -LiteralPath $p -Recurse -Force
  if(Test-Path -LiteralPath $p){Start-Sleep -Milliseconds 500}
 }
}`

// ScheduleDelete starts a hidden helper that waits for process pid to exit
// and then deletes paths (recursively). Unsafe paths are refused.
func ScheduleDelete(pid int, paths []string) error {
	var ok []string
	for _, p := range paths {
		if !SafeToDelete(p) {
			return fmt.Errorf("refusing to delete %q", p)
		}
		ok = append(ok, filepath.Clean(p))
	}
	if len(ok) == 0 {
		return nil
	}
	cmd := psCmd(deleteScript)
	cmd.Env = append(os.Environ(), "CA_PID="+strconv.Itoa(pid), "CA_DEL="+strings.Join(ok, "|"))
	return cmd.Start()
}

// ScheduleUninstall removes only known CopyApp files, then removes the two
// directories only if empty. Even a custom install/data location must never
// cause unrelated user files or received files to be recursively deleted.
func ScheduleUninstall(pid int, p Paths, dataDir string, deleteData bool) error {
	dirs := []string{p.InstallDir}
	files := []string{p.Exe(), p.Exe() + ".new"}
	if deleteData {
		dirs = append(dirs, dataDir)
		for _, name := range []string{"settings.json", "lang.txt", "session.json", "local.key", "port.txt", "local-port.txt", "copyapp.log", "copyapp.log.1", "ca.crt", "ca.key", "leaf.crt", "leaf.key"} {
			files = append(files, filepath.Join(dataDir, name))
		}
	}
	for _, dir := range dirs {
		if !SafeToDelete(dir) {
			return fmt.Errorf("refusing to uninstall from %q", dir)
		}
	}
	const script = `$ErrorActionPreference='SilentlyContinue'
try{Wait-Process -Id ([int]$env:CA_PID) -Timeout 60}catch{}
if(Get-Process -Id ([int]$env:CA_PID) -ErrorAction SilentlyContinue){exit}
Start-Sleep -Milliseconds 700
foreach($p in ($env:CA_FILES -split '\|')){
 for($i=0;$i -lt 20 -and [IO.File]::Exists($p);$i++){
  [IO.File]::Delete($p)
  if([IO.File]::Exists($p)){Start-Sleep -Milliseconds 500}
 }
}
foreach($d in ($env:CA_DIRS -split '\|')){try{[IO.Directory]::Delete($d,$false)}catch{}}`
	cmd := psCmd(script)
	cmd.Env = append(os.Environ(), "CA_PID="+strconv.Itoa(pid), "CA_FILES="+strings.Join(files, "|"), "CA_DIRS="+strings.Join(dirs, "|"))
	return cmd.Start()
}
