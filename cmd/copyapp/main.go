// Command copyapp is the Windows tray app that shares the clipboard with a
// paired phone over the LAN.
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"golang.design/x/hotkey"
	"golang.org/x/sys/windows"

	"github.com/igalgerman/copyapp/internal/auth"
	"github.com/igalgerman/copyapp/internal/pc"
	"github.com/igalgerman/copyapp/internal/server"
	"github.com/igalgerman/copyapp/internal/store"
	"github.com/igalgerman/copyapp/internal/tlsutil"
)

const (
	defaultPort    = 8443
	sendFilePath   = "/local/send-file"
	sendToLinkName = "Phone (CopyApp).lnk"
)

var (
	dataDir  string
	srv      *server.Server
	sess     *auth.Auth
	machine  *pc.PC
	firstRun bool
)

func main() {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		h, _ := os.UserHomeDir()
		appdata = filepath.Join(h, "AppData", "Roaming")
	}
	dataDir = filepath.Join(appdata, "CopyApp")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatal(err)
	}
	if lf, err := os.OpenFile(filepath.Join(dataDir, "copyapp.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
		log.SetOutput(lf)
	}

	var fileArg string
	if len(os.Args) > 1 {
		fileArg = os.Args[1]
	}

	// Single instance.
	h, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(`Local\CopyAppSingleton`))
	if err == syscall.ERROR_ALREADY_EXISTS {
		if h != 0 {
			windows.CloseHandle(h)
		}
		secondInstance(fileArg)
		return
	} else if err != nil {
		log.Fatal(err)
	}

	_, statErr := os.Stat(filepath.Join(dataDir, "port.txt"))
	firstRun = statErr != nil

	machine, err = pc.New()
	if err != nil {
		log.Fatal(err)
	}
	tlsCfg, caPEM, err := tlsutil.Load(dataDir, pc.LanIPs())
	if err != nil {
		log.Fatal(err)
	}
	sess = auth.New()
	if err := sess.Persist(filepath.Join(dataDir, "session.json")); err != nil {
		log.Printf("persist session: %v", err)
	}

	httpsLn, err := listenRemembered("port.txt", "0.0.0.0", defaultPort, true)
	if err != nil {
		log.Fatal(err)
	}
	loopLn, err := listenRemembered("local-port.txt", "127.0.0.1", 0, false)
	if err != nil {
		log.Fatal(err)
	}
	httpsPort := httpsLn.Addr().(*net.TCPAddr).Port

	srv = server.New(server.Config{
		Auth:    sess,
		Store:   store.New(),
		Clip:    machine,
		SaveDir: pc.ReceivedDir(),
		CAPEM:   caPEM,
		BaseURL: func() string {
			ip := "localhost"
			if p := pc.PrimaryIP(); p != nil {
				ip = p.String()
			}
			return "https://" + net.JoinHostPort(ip, strconv.Itoa(httpsPort))
		},
		Logf: log.Printf,
	})

	go func() {
		s := &http.Server{Handler: srv, ReadHeaderTimeout: 10 * time.Second, ErrorLog: log.Default()}
		log.Printf("https: %v", s.Serve(tls.NewListener(httpsLn, tlsCfg)))
	}()
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("POST "+sendFilePath, handleSendFile)
		mux.Handle("/", srv)
		s := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, ErrorLog: log.Default()}
		log.Printf("loopback: %v", s.Serve(loopLn))
	}()

	if firstRun {
		if err := pc.SetAutostart(true); err != nil {
			log.Printf("autostart: %v", err)
		}
		go createSendToShortcut()
	}
	if fileArg != "" {
		go sendFile(fileArg)
	}
	go hotkeyLoop()
	systray.Run(onReady, func() {})
}

// listenRemembered listens on the port saved in dir\name if free, otherwise on
// the preferred port (or the next free ones when scan is set; any free port
// otherwise) and saves the result.
func listenRemembered(name, host string, preferred int, scan bool) (net.Listener, error) {
	path := filepath.Join(dataDir, name)
	var tries []int
	if p := readPort(name); p > 0 {
		tries = append(tries, p)
	}
	if scan {
		for p := preferred; p < preferred+50; p++ {
			tries = append(tries, p)
		}
	} else {
		tries = append(tries, 0)
	}
	for _, p := range tries {
		ln, err := net.Listen("tcp4", net.JoinHostPort(host, strconv.Itoa(p)))
		if err != nil {
			continue
		}
		port := ln.Addr().(*net.TCPAddr).Port
		os.WriteFile(path, []byte(strconv.Itoa(port)), 0o600)
		return ln, nil
	}
	return nil, fmt.Errorf("no free port for %s", name)
}

func readPort(name string) int {
	b, err := os.ReadFile(filepath.Join(dataDir, name))
	if err != nil {
		return 0
	}
	p, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if p < 1 || p > 65535 {
		return 0
	}
	return p
}

func pairingURL() string {
	return "http://127.0.0.1:" + strconv.Itoa(readPort("local-port.txt")) + "/pairing"
}

func openBrowser(u string) {
	exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

// secondInstance forwards to the running instance.
func secondInstance(fileArg string) {
	port := readPort("local-port.txt")
	if port == 0 {
		return
	}
	if fileArg == "" {
		openBrowser(pairingURL())
		return
	}
	abs, err := filepath.Abs(fileArg)
	if err != nil {
		return
	}
	req, _ := http.NewRequest("POST", "http://127.0.0.1:"+strconv.Itoa(port)+sendFilePath+"?path="+url.QueryEscape(abs), nil)
	req.Header.Set("X-CopyApp", "1")
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// handleSendFile lets a second launch (Explorer "Send to") hand a file path to
// this instance. Loopback peer, 127.0.0.1 Host, and a custom header (browsers
// cannot send it cross-site without a CORS preflight, which is never granted).
func handleSendFile(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	hh, _, _ := net.SplitHostPort(r.Host)
	if ip == nil || !ip.IsLoopback() || hh != "127.0.0.1" || r.Header.Get("X-CopyApp") != "1" || r.Header.Get("Origin") != "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	sendFile(r.URL.Query().Get("path"))
	w.WriteHeader(http.StatusNoContent)
}

func sendFile(path string) {
	name := filepath.Base(path)
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		machine.Notify("Not sent", "Can't send "+name)
		return
	}
	if fi.Size() > store.MaxItemSize {
		machine.Notify("Not sent", name+" is larger than 200 MB")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		machine.Notify("Not sent", "Can't read "+name)
		return
	}
	mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if mt == "" {
		mt = "application/octet-stream"
	}
	if _, err := srv.Publish(store.Item{Kind: "file", Name: name, Mime: mt, Data: data}); err != nil {
		machine.Notify("Not sent", err.Error())
		return
	}
	machine.Notify("Sent to phone", name)
}

func sendClipboard() {
	if machine.Sensitive() {
		machine.Notify("Not sent", "Clipboard looks like a secret; skipped")
		return
	}
	var it store.Item
	what := "Text"
	if img := machine.ReadImagePNG(); len(img) > 0 {
		it = store.Item{Kind: "image", Name: "clipboard.png", Mime: "image/png", Data: img}
		what = "Image"
	} else if t := machine.ReadText(); t != "" {
		it = store.Item{Kind: "text", Name: "clipboard.txt", Mime: "text/plain; charset=utf-8", Data: []byte(t)}
	} else {
		machine.Notify("Nothing to send", "The clipboard is empty")
		return
	}
	if _, err := srv.Publish(it); err != nil {
		machine.Notify("Not sent", err.Error())
		return
	}
	machine.Notify("Sent to phone", what+" from clipboard")
}

func hotkeyLoop() {
	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModAlt}, hotkey.KeyC)
	if err := hk.Register(); err != nil {
		log.Printf("hotkey: %v", err)
		return
	}
	for range hk.Keydown() {
		go sendClipboard()
	}
}

func startPairing() {
	srv.StartPairing()
	openBrowser(pairingURL())
}

func onReady() {
	systray.SetIcon(makeICO())
	systray.SetTitle("CopyApp")
	systray.SetTooltip("CopyApp - Ctrl+Alt+C sends the clipboard to your phone")
	mPair := systray.AddMenuItem("Pair phone…", "Show the QR code")
	mSend := systray.AddMenuItem("Send clipboard to phone", "Ctrl+Alt+C")
	mOpen := systray.AddMenuItem("Open received folder", "")
	systray.AddSeparator()
	mAuto := systray.AddMenuItemCheckbox("Start with Windows", "", pc.AutostartEnabled())
	mRepair := systray.AddMenuItem("Re-pair phone", "Forget the current phone")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "")

	if firstRun || !sess.Paired() {
		go startPairing()
	}
	go func() {
		for {
			select {
			case <-mPair.ClickedCh:
				startPairing()
			case <-mSend.ClickedCh:
				go sendClipboard()
			case <-mOpen.ClickedCh:
				if err := pc.OpenFolder(pc.ReceivedDir()); err != nil {
					machine.Notify("CopyApp", err.Error())
				}
			case <-mAuto.ClickedCh:
				on := !mAuto.Checked()
				if err := pc.SetAutostart(on); err != nil {
					machine.Notify("CopyApp", err.Error())
					continue
				}
				if on {
					mAuto.Check()
				} else {
					mAuto.Uncheck()
				}
			case <-mRepair.ClickedCh:
				srv.Unpair()
				startPairing()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// createSendToShortcut adds "Send to > Phone (CopyApp)" (best effort).
func createSendToShortcut() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	ps := `$d=[Environment]::GetFolderPath('SendTo');$s=(New-Object -ComObject WScript.Shell).CreateShortcut((Join-Path $d $env:CA_LNK));` +
		`$s.TargetPath=$env:CA_EXE;$s.WorkingDirectory=Split-Path $env:CA_EXE;$s.Save()`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.Env = append(os.Environ(), "CA_EXE="+exe, "CA_LNK="+sendToLinkName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	if err := cmd.Run(); err != nil {
		log.Printf("sendto shortcut: %v", err)
	}
}
