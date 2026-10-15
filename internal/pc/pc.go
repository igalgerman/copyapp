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
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"golang.design/x/clipboard"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

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
	go func() { _ = toastCmd(title, msg).Run() }()
}

func toastCmd(title, msg string) *exec.Cmd {
	u := utf16.Encode([]rune(toastScript))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	cmd.Env = append(os.Environ(), "COPYAPP_TOAST_TITLE="+title, "COPYAPP_TOAST_MSG="+msg)
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
	return filepath.Join(d, AppName)
}

// OpenFolder creates path if needed and opens it in Explorer.
func OpenFolder(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return exec.Command("explorer.exe", path).Start()
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

// SetAutostart adds or removes CopyApp from HKCU\...\Run.
func SetAutostart(on bool) error {
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
	return k.SetStringValue(AppName, `"`+exe+`"`)
}

// AutostartEnabled reports whether the Run entry exists.
func AutostartEnabled() bool {
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
func WiFiName() string {
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
