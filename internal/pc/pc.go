//go:build windows

// Package pc is the Windows side: clipboard, toasts, folders, autostart.
package pc

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"golang.design/x/clipboard"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/igalgerman/copyapp/internal/settings"
	"github.com/igalgerman/copyapp/internal/store"
)

// AppName is used for the autostart registry value and the received folder.
const AppName = "CopyApp"

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// ErrNotPNG is returned by SetImagePNG for data that is not a PNG image.
var ErrNotPNG = errors.New("pc: image is not a valid PNG")

// PC implements the server's Clipboard interface on the real Windows clipboard.
type PC struct {
	// NotifyFunc, when set, replaces the PowerShell toast (e.g. a tray balloon).
	NotifyFunc func(title, msg string)
	// Logf receives one line per toast (shown or why not). Never the text.
	Logf func(format string, args ...any)
}

// New initializes clipboard access.
func New() (*PC, error) {
	if err := clipboard.Init(); err != nil {
		return nil, err
	}
	return &PC{}, nil
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// SetText replaces the clipboard with text.
func (p *PC) SetText(s string) error {
	c, cancel := ctx()
	defer cancel()
	_, err := clipboard.Write(c, clipboard.FmtText, []byte(s))
	return err
}

// ErrTooManyPixels is returned by SetImagePNG for images over
// store.MaxImagePixels.
var ErrTooManyPixels = errors.New("pc: image is larger than 50 megapixels")

// SetImagePNG validates data as PNG and replaces the clipboard with it. On
// invalid or oversized data the clipboard is left unchanged.
func (p *PC) SetImagePNG(data []byte) error {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ErrNotPNG
	}
	if int64(cfg.Width)*int64(cfg.Height) > store.MaxImagePixels {
		return ErrTooManyPixels
	}
	c, cancel := ctx()
	defer cancel()
	_, err = clipboard.Write(c, clipboard.FmtImage, data)
	return err
}

// ReadText returns the clipboard text, or "" if there is none.
func (p *PC) ReadText() string {
	c, cancel := ctx()
	defer cancel()
	b, err := clipboard.Read(c, clipboard.FmtText)
	if err != nil {
		return ""
	}
	return string(b)
}

// ReadImagePNG returns the clipboard image as PNG, or nil if there is none.
func (p *PC) ReadImagePNG() []byte {
	c, cancel := ctx()
	defer cancel()
	b, err := clipboard.Read(c, clipboard.FmtImage)
	if err != nil {
		return nil
	}
	return b
}

// Sensitive reports whether the clipboard content is marked as a secret by
// e.g. a password manager; such content should not be sent to the phone.
func (p *PC) Sensitive() bool {
	c, cancel := ctx()
	defer cancel()
	s, err := clipboard.Sensitive(c)
	return err == nil && s
}

// toastScript runs in Windows PowerShell 5.1 (WinRT projection). Caller data
// arrives only through environment variables and is XML-escaped.
const toastScript = `$ErrorActionPreference='Stop'
[void][Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
[void][Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]
$t=[Security.SecurityElement]::Escape($env:COPYAPP_TOAST_TITLE)
$m=[Security.SecurityElement]::Escape($env:COPYAPP_TOAST_MSG)
$x=New-Object Windows.Data.Xml.Dom.XmlDocument
$x.LoadXml('<toast><visual><binding template="ToastGeneric"><text>'+$t+'</text><text>'+$m+'</text></binding></visual></toast>')
$n=[Windows.UI.Notifications.ToastNotification]::new($x)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($n)`

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Notify shows a Windows toast without blocking.
func (p *PC) Notify(title, msg string) {
	title, msg = truncate(title, 64), truncate(msg, 200)
	if p.NotifyFunc != nil {
		p.NotifyFunc(title, msg)
		return
	}
	go func() {
		out, err := toastCmd(title, msg).CombinedOutput()
		if p.Logf == nil {
			return
		}
		if err != nil {
			p.Logf("toast: notifier failed: %v: %s", err, truncate(strings.TrimSpace(string(out)), 300))
		} else {
			p.Logf("toast: shown")
		}
	}()
}

func toastCmd(title, msg string) *exec.Cmd {
	cmd := psCmd(toastScript, false)
	cmd.Env = append(os.Environ(), "COPYAPP_TOAST_TITLE="+title, "COPYAPP_TOAST_MSG="+msg)
	return cmd
}

// psCmd runs a Windows PowerShell 5.1 script (passed encoded, so no quoting
// issues) without a window. sta is needed for WinForms dialogs.
func psCmd(script string, sta bool) *exec.Cmd {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass"}
	if sta {
		args = append(args, "-STA")
	}
	cmd := exec.Command("powershell.exe", append(args, "-EncodedCommand", base64.StdEncoding.EncodeToString(b))...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd
}

// ReceivedDir is where files from the phone are saved (Downloads\CopyApp).
func ReceivedDir() string {
	d, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0)
	if err != nil || d == "" {
		h, _ := os.UserHomeDir()
		d = filepath.Join(h, "Downloads")
	}
	local, _ := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	return chooseReceivedDir(d, local)
}

func chooseReceivedDir(downloads, local string) string {
	preferred := filepath.Join(downloads, AppName)
	if settings.ValidSaveDir(preferred) {
		return preferred
	}
	// Downloads may be redirected to a network share or a protected folder.
	// Keep the default receive destination local even in that configuration.
	fallback := filepath.Join(local, AppName, "Received")
	if settings.ValidSaveDir(fallback) {
		return fallback
	}
	return "" // settings.Load fails closed if both destinations are unsafe
}

// OpenFolder creates path if needed and opens it in Explorer.
// The path is passed to the shell as one string (no command line is built,
// so commas or quotes in a folder name cannot become Explorer switches).
func OpenFolder(path string) error {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\"\x00") {
		return errors.New("pc: not an absolute folder path")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
		return errors.New("pc: not a folder")
	}
	verb, _ := windows.UTF16PtrFromString("open")
	p, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, p, nil, nil, windows.SW_SHOWNORMAL)
}

// LanIPs returns the machine's private, non-loopback IPv4 addresses, the one
// used for outbound traffic first.
func LanIPs() []net.IP {
	var out []net.IP
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := n.IP.To4()
			if ip != nil && ip.IsPrivate() && !ip.IsLoopback() {
				out = append(out, ip)
			}
		}
	}
	pri := PrimaryIP()
	sort.SliceStable(out, func(i, j int) bool { return pri != nil && out[i].Equal(pri) && !out[j].Equal(pri) })
	return out
}

// PrimaryIP returns the local IPv4 address used for outbound traffic, or nil.
// No packets are sent.
func PrimaryIP() net.IP {
	c, err := net.Dial("udp4", "192.0.2.1:9") // TEST-NET-1, never contacted
	if err != nil {
		return nil
	}
	defer c.Close()
	ip := c.LocalAddr().(*net.UDPAddr).IP.To4()
	if ip == nil || !ip.IsPrivate() {
		return nil
	}
	return ip
}

// noRunWrite: COPYAPP_NO_AUTOSTART_WRITE=1 keeps the Run entry in memory
// (tests and the e2e script must not touch the user's registry).
var (
	noRunWrite = os.Getenv("COPYAPP_NO_AUTOSTART_WRITE") != ""
	fakeRun    atomic.Bool
)

// SetAutostart adds or removes CopyApp (this exe) from HKCU\...\Run.
// COPYAPP_NO_AUTOSTART_WRITE keeps the choice in memory only.
func SetAutostart(on bool) error {
	if noRunWrite {
		fakeRun.Store(on)
		return nil
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		err := k.DeleteValue(AppName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// --autostart: start quietly in the tray, without opening the window.
	return k.SetStringValue(AppName, `"`+exe+`" --autostart`)
}

// AutostartEnabled reports whether the Run entry exists.
func AutostartEnabled() bool {
	if noRunWrite {
		return fakeRun.Load()
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(AppName)
	return err == nil
}

var procGetOEMCP = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetOEMCP")

// WiFiName returns the SSID of the Wi-Fi network this PC is connected to, or
// "" when it is unknown, disconnected or the PC is wired only. It runs
// `netsh wlan show interfaces` (about 50 ms, no window).
// COPYAPP_WIFI_NAME overrides it (screenshots, videos and tests must not show
// the real network name).
func WiFiName() string {
	if v := os.Getenv("COPYAPP_WIFI_NAME"); v != "" {
		return v
	}
	c, cancel := ctx()
	defer cancel()
	cmd := exec.CommandContext(c, "netsh.exe", "wlan", "show", "interfaces")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return ParseSSID(decodeOEM(out))
}

// decodeOEM turns netsh output into a Go string. Without a console netsh
// writes in the OEM code page; UTF-8 output is passed through.
func decodeOEM(b []byte) string {
	if utf8.Valid(b) || len(b) == 0 {
		return string(b)
	}
	cp, _, _ := procGetOEMCP.Call()
	n, err := windows.MultiByteToWideChar(uint32(cp), 0, &b[0], int32(len(b)), nil, 0)
	if err != nil || n <= 0 {
		return string(b)
	}
	w := make([]uint16, n)
	if _, err := windows.MultiByteToWideChar(uint32(cp), 0, &b[0], int32(len(b)), &w[0], n); err != nil {
		return string(b)
	}
	return string(utf16.Decode(w))
}

// AutorunFolders lists the Start menu and Startup folders (this user and all
// users): received files must never land there.
func AutorunFolders() []string {
	var out []string
	for _, id := range []*windows.KNOWNFOLDERID{windows.FOLDERID_Startup, windows.FOLDERID_CommonStartup,
		windows.FOLDERID_StartMenu, windows.FOLDERID_CommonStartMenu, windows.FOLDERID_System, windows.FOLDERID_SystemX86} {
		if d, err := windows.KnownFolderPath(id, 0); err == nil && d != "" {
			out = append(out, d)
		}
	}
	return out
}
