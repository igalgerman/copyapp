// Command copyapp is the Windows tray app that shares the clipboard with a
// paired phone over the LAN.
package main

import (
	"crypto/tls"
	"errors"
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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"golang.design/x/clipboard"
	"golang.design/x/hotkey"
	"golang.org/x/sys/windows"

	"github.com/igalgerman/copyapp/internal/applog"
	"github.com/igalgerman/copyapp/internal/auth"
	"github.com/igalgerman/copyapp/internal/diagnostics"
	"github.com/igalgerman/copyapp/internal/i18n"
	"github.com/igalgerman/copyapp/internal/install"
	"github.com/igalgerman/copyapp/internal/pc"
	"github.com/igalgerman/copyapp/internal/secret"
	"github.com/igalgerman/copyapp/internal/server"
	"github.com/igalgerman/copyapp/internal/settings"
	"github.com/igalgerman/copyapp/internal/store"
	"github.com/igalgerman/copyapp/internal/tlsutil"
)

const (
	defaultPort  = 8443
	sendFilePath = "/local/send-file"

	// idleTimeout closes idle keep-alive connections. It does not cut active
	// requests: SSE (/events) and big uploads/downloads run inside a handler
	// and manage their own deadlines.
	idleTimeout = 2 * time.Minute
)

// version is set at build time: -ldflags "-X main.version=1.0.0" (build.ps1).
var version = "dev"

var clipboardAttempt atomic.Uint64
var lastClipboardStatus atomic.Value

var (
	quietStart    bool // started by Windows at sign-in (--autostart)
	setupLinks    bool // just installed by the downloaded exe (--setup): create the shortcuts
	dataDir       string
	srv           *server.Server
	sess          *auth.Auth
	machine       *pc.PC
	firstRun      bool
	localKey      string // per-launch secret for the loopback routes (see secret.LocalKeyFile)
	prefs         *settings.Store
	activeHotkey  atomic.Value
	trayReady     atomic.Bool
	flashMu       sync.Mutex
	refreshTray   = make(chan struct{}, 1)
	sendMu        sync.Mutex
	openOnLaunch  bool
	shortcutSetup sync.WaitGroup // the --setup helper must finish before exit or shortcut removal
)

func main() {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		h, _ := os.UserHomeDir()
		appdata = filepath.Join(h, "AppData", "Roaming")
	}
	dataDir = filepath.Join(appdata, "CopyApp")
	if d := os.Getenv("COPYAPP_DATA_DIR"); d != "" {
		dataDir = d
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatal(err)
	}
	if lf, err := applog.Open(filepath.Join(dataDir, "copyapp.log"), applog.DefaultMax); err == nil {
		log.SetOutput(lf)
		defer lf.Close()
	}

	reporter, diagErr := diagnostics.Start(dataDir, version, log.Printf)
	if diagErr != nil {
		log.Printf("diagnostics: initialization unavailable")
	}
	defer reporter.Close()
	defer diagnostics.Recover()
	i18n.Load(dataDir)
	settings.ExtraDenied = pc.AutorunFolders()
	receiveDir := pc.ReceivedDir()
	if d := os.Getenv("COPYAPP_RECEIVE_DIR"); d != "" {
		receiveDir = d
	}
	var err error
	prefs, err = settings.Load(dataDir, receiveDir)
	if err != nil {
		log.Fatal(err)
	}
	i18n.Set(prefs.Get().Language)
	activeHotkey.Store("")
	log.Printf("CopyApp %s starting (pid %d, args %d)", version, os.Getpid(), len(os.Args)-1)

	var fileArg string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--open":
			openOnLaunch = true
		case "--autostart":
			quietStart = true
		case "--setup":
			setupLinks = true
		case "--portable":
			os.Setenv("COPYAPP_PORTABLE", "1")
		case "--no-browser":
			os.Setenv("COPYAPP_NO_BROWSER", "1")
		default:
			if fileArg == "" {
				fileArg = arg
			}
		}
	}

	// Single instance.
	h, err := instanceMutex()
	if err == syscall.ERROR_ALREADY_EXISTS {
		if h != 0 {
			windows.CloseHandle(h)
		}
		secondInstance(fileArg)
		return
	} else if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if h != 0 {
			windows.CloseHandle(h)
		}
	}()

	// Install into a stable per-user location, then run that copy so startup
	// and all shortcuts remain valid after the download is moved or removed.
	if os.Getenv("COPYAPP_PORTABLE") == "" {
		exe, e := os.Executable()
		paths := install.Default()
		if e == nil && !install.SamePath(exe, paths.Exe()) {
			if _, e = install.CopyExe(paths, exe); e == nil {
				log.Printf("self-install: copied to %s, starting it", paths.InstallDir)
				args := append([]string{"--setup"}, os.Args[1:]...)
				if !quietStart {
					args = append([]string{"--open"}, args...)
				}
				cmd := exec.Command(paths.Exe(), args...)
				cmd.Dir = paths.InstallDir
				windows.CloseHandle(h)
				h = 0
				if e = cmd.Start(); e == nil {
					return
				}
			}
			log.Printf("self-install: %v", e)
			pc.Info("CopyApp", i18n.T("inst.fail", e), i18n.Lang() == "he")
		}
	}

	if setupLinks {
		shortcutSetup.Add(1)
		// Keep startup responsive, but retain the singleton until the helper
		// finishes so an upgrade cannot overlap an older shortcut writer.
		defer shortcutSetup.Wait()
		go func() {
			defer shortcutSetup.Done()
			if err := install.CreateShortcuts(install.Default()); err != nil {
				log.Printf("shortcuts: %v", err)
			} else {
				log.Printf("shortcuts created")
			}
		}()
	}

	_, statErr := os.Stat(filepath.Join(dataDir, "port.txt"))
	firstRun = statErr != nil

	machine, err = pc.New()
	if err != nil {
		log.Fatal(err)
	}
	machine.Logf = log.Printf // one line per toast: shown, or why not (never the text)
	pc.OSDLogf = log.Printf
	tlsCfg, caPEM, err := tlsutil.Load(dataDir, pc.LanIPs, secret.DPAPI{})
	if err != nil {
		log.Fatal(err)
	}
	if localKey, err = secret.NewLocalKey(dataDir); err != nil {
		log.Fatal(err)
	}
	sess = auth.New()
	if err := sess.Persist(filepath.Join(dataDir, "session.json")); err != nil {
		log.Printf("persist session: %v", err)
	}

	httpsLn, err := listenRemembered("port.txt", httpsBindHost, defaultPort, true)
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
			if httpsBindHost != "127.0.0.1" {
				if p := pc.PrimaryIP(); p != nil {
					ip = p.String()
				}
			}
			return "https://" + net.JoinHostPort(ip, strconv.Itoa(httpsPort))
		},
		Logf:          log.Printf,
		LocalSecret:   localKey,
		WiFiName:      pc.WiFiName,
		Settings:      prefs,
		ChooseFolder:  func(current string) (string, error) { return pc.ChooseFolder(i18n.T("pick.folder"), current) },
		OpenFolder:    pc.OpenFolder,
		Uninstall:     uninstallApp,
		Autostart:     pc.AutostartEnabled,
		SetAutostart:  pc.SetAutostart,
		OnSettings:    func(old, cur settings.Settings) { signalTrayRefresh() },
		LogPath:       filepath.Join(dataDir, "copyapp.log"),
		Hotkey:        func() string { return activeHotkey.Load().(string) },
		SendStatus:    clipboardStatus,
		Version:       version,
		SendClipboard: sendClipboard,
		OnPaired:      func(string) { pc.Beep(true); flashTray(); signalTrayRefresh() },
	})

	go func() {
		s := &http.Server{Handler: srv, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: idleTimeout, ErrorLog: log.Default()}
		log.Printf("https: %v", s.Serve(tls.NewListener(httpsLn, tlsCfg)))
	}()
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("POST "+sendFilePath, handleSendFile)
		mux.HandleFunc("POST /local/quit", func(w http.ResponseWriter, r *http.Request) {
			if !server.LocalAllowed(r, localKey) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			go func() { time.Sleep(200 * time.Millisecond); systray.Quit() }()
		})
		mux.Handle("/", srv)
		s := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: idleTimeout, ErrorLog: log.Default()}
		log.Printf("loopback: %v", s.Serve(loopLn))
	}()

	if !prefs.Get().AutostartAsked {
		on := pc.Ask("CopyApp", i18n.T("ask.autostart"), i18n.Lang() == "he", false)
		if err := pc.SetAutostart(on); err != nil {
			log.Printf("autostart: %v", err)
		}
		if _, err := prefs.Update(func(st *settings.Settings) { st.AutostartAsked = true }); err != nil {
			log.Printf("save startup choice: %v", err)
		}
	}
	if fileArg != "" {
		go sendFile(fileArg)
	}
	go hotkeyLoop()
	systray.Run(onReady, func() {})
}

// instanceMutex acquires the singleton, optionally stopping an older installed
// version so a newly downloaded executable can replace it.
func instanceMutex() (windows.Handle, error) {
	h, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(`Local\CopyAppSingleton`))
	if err != syscall.ERROR_ALREADY_EXISTS || os.Getenv("COPYAPP_PORTABLE") != "" {
		return h, err
	}
	exe, e := os.Executable()
	installed := install.Default().Exe()
	if e != nil || install.SamePath(exe, installed) || install.SameFile(exe, installed) {
		return h, err
	}
	// An updated download asks the installed instance to stop through the
	// authenticated loopback endpoint, then acquires the singleton before copying.
	key, e := secret.ReadLocalKey(dataDir)
	port := readPort("local-port.txt")
	if e != nil || port == 0 {
		return h, err
	}
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(port)+"/local/quit", nil)
	req.Header.Set("X-CopyApp", key)
	resp, e := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if e != nil {
		return h, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return h, err
	}
	windows.CloseHandle(h)
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		h, err = windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(`Local\CopyAppSingleton`))
		if err != syscall.ERROR_ALREADY_EXISTS {
			return h, err
		}
		windows.CloseHandle(h)
	}
	return 0, syscall.ERROR_ALREADY_EXISTS
}

// listenRemembered reuses a saved port when available and saves the result.
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

// pairingURL carries the local secret once; the page trades it for an
// HttpOnly cookie and redirects to the bare URL.
func pairingURL(key string) string {
	return "http://127.0.0.1:" + strconv.Itoa(readPort("local-port.txt")) + "/pairing?k=" + url.QueryEscape(key)
}

func openBrowser(u string) {
	if os.Getenv("COPYAPP_NO_BROWSER") != "" {
		return
	}
	exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

// secondInstance forwards to the running instance.
func secondInstance(fileArg string) {
	port := readPort("local-port.txt")
	key, err := secret.ReadLocalKey(dataDir)
	if port == 0 || err != nil {
		return
	}
	if fileArg == "" {
		openBrowser(pairingURL(key))
		return
	}
	abs, err := filepath.Abs(fileArg)
	if err != nil {
		return
	}
	req, _ := http.NewRequest("POST", "http://127.0.0.1:"+strconv.Itoa(port)+sendFilePath+"?path="+url.QueryEscape(abs), nil)
	req.Header.Set("X-CopyApp", key)
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// handleSendFile lets a second launch (Explorer "Send to") hand a file path to
// this instance. Loopback peer, 127.0.0.1 Host, no Origin, and the per-launch
// secret in X-CopyApp (only this Windows user can read it from %APPDATA%;
// browsers cannot send the header cross-site without a CORS preflight, which
// is never granted).
func handleSendFile(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	hh, _, _ := net.SplitHostPort(r.Host)
	if ip == nil || !ip.IsLoopback() || hh != "127.0.0.1" || !secret.Equal(r.Header.Get("X-CopyApp"), localKey) || r.Header.Get("Origin") != "" {
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
		machine.Notify(i18n.T("toast.notSent"), i18n.T("toast.cantSend", name))
		return
	}
	if !fi.Mode().IsRegular() || fi.Size() > prefs.Get().MaxBytes {
		log.Printf("file send refused: %d bytes, limit %d", fi.Size(), prefs.Get().MaxBytes)
		pc.Beep(false)
		machine.Notify(i18n.T("toast.notSent"), i18n.T("toast.tooLarge", name, humanSize(prefs.Get().MaxBytes)))
		return
	}
	mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if mt == "" {
		mt = "application/octet-stream"
	}
	if _, err := srv.Publish(store.Item{Kind: "file", Name: name, Mime: mt, Path: path}); err != nil {
		machine.Notify(i18n.T("toast.notSent"), publishErrText(err))
		return
	}
	result := sentFeedback("file", fi.Size())
	machine.Notify(i18n.T("home.res."+result), name)
}

// humanSize formats a size limit ("500 MB", "2 GB").
func humanSize(n int64) string {
	if n >= 1<<30 && n%(1<<30) == 0 {
		return strconv.FormatInt(n>>30, 10) + " GB"
	}
	return strconv.FormatInt(n>>20, 10) + " MB"
}

// sendClipboard publishes the PC clipboard to the phone. source ("hotkey",
// "tray", "window") is only logged. Results distinguish queued/published from
// empty, sensitive, busy, read or publish failures. Logs exclude clipboard content.
func sendClipboard(source string) (result string) {
	operation := diagnostics.NewOperation()
	attempt := clipboardAttempt.Add(1)
	started := time.Now()
	log.Printf("clipboard attempt=%d operation=%s source=%s requested", attempt, operation, source)
	if source == "hotkey" {
		diagnostics.Emit("hotkey", "received", "ok", operation)
	}
	diagnostics.Emit("clipboard", "requested", source, operation)
	defer func() {
		log.Printf("clipboard attempt=%d source=%s result=%s duration_ms=%d", attempt, source, result, time.Since(started).Milliseconds())
		diagnostics.Emit("clipboard", "complete", result, operation)
		// A busy trigger must not overwrite the status of an in-flight attempt.
		if result != "busy" {
			lastClipboardStatus.Store(server.ClipboardSendStatus{At: time.Now(), Source: source, Result: result})
		}
	}()
	if !sendMu.TryLock() {
		return "busy"
	}
	defer sendMu.Unlock()
	it, err := machine.ReadClipboard()
	if errors.Is(err, pc.ErrSensitiveClipboard) {
		pc.Beep(false)
		machine.Notify(i18n.T("toast.notSent"), i18n.T("toast.secret"))
		return "secret"
	}
	if errors.Is(err, clipboard.ErrNoData) {
		pc.Beep(false)
		machine.Notify(i18n.T("toast.nothing"), i18n.T("toast.clipEmpty"))
		return "empty"
	}
	if err != nil {
		log.Printf("clipboard attempt=%d read failed: %v", attempt, err)
		pc.Beep(false)
		machine.Notify(i18n.T("toast.notSent"), i18n.T("home.res.readFailed"))
		return "readFailed"
	}
	diagnostics.Emit("clipboard", "read", it.Kind, operation)
	if _, err := srv.Publish(it); err != nil {
		log.Printf("clipboard attempt=%d publish failed: %v", attempt, err)
		pc.Beep(false)
		machine.Notify(i18n.T("toast.notSent"), publishErrText(err))
		return "failed"
	}
	// Publish stores an outbox item and signals SSE; it is not a phone ACK.
	result = "queued"
	if srv.PhoneConnected() {
		result = "published"
	}
	log.Printf("clipboard attempt=%d publish=%s kind=%s bytes=%d", attempt, result, it.Kind, len(it.Data))
	pc.Beep(true)
	pc.ShowSent(i18n.T("home.res."+result), i18n.Lang() == "he")
	machine.Notify("CopyApp", i18n.T("home.res."+result))
	flashTray()
	return result
}

func clipboardStatus() server.ClipboardSendStatus {
	if v := lastClipboardStatus.Load(); v != nil {
		return v.(server.ClipboardSendStatus)
	}
	return server.ClipboardSendStatus{}
}

func hotkeyLoop() {
	delay := time.Second
	warned := false
	for {
		var selected *hotkey.Hotkey
		name := ""
		for i, mods := range [][]hotkey.Modifier{{hotkey.ModCtrl, hotkey.ModAlt}, {hotkey.ModCtrl, hotkey.ModAlt, hotkey.ModShift}} {
			candidate := hotkey.New(mods, hotkey.KeyC)
			if err := candidate.Register(); err != nil {
				log.Printf("hotkey candidate=%d registration failed: %v", i+1, err)
				diagnostics.Emit("hotkey", "registration", "failed", "")
				continue
			}
			selected = candidate
			name = "Ctrl+Alt+C"
			if i == 1 {
				name = "Ctrl+Alt+Shift+C"
			}
			break
		}
		if selected == nil {
			activeHotkey.Store("")
			signalTrayRefresh()
			if !warned {
				machine.Notify("CopyApp", i18n.T("hk.none"))
				warned = true
			}
		} else {
			activeHotkey.Store(name)
			signalTrayRefresh()
			log.Printf("hotkey registered: %s", name)
			diagnostics.Emit("hotkey", "registration", "registered", "")
			if name != "Ctrl+Alt+C" && !warned {
				machine.Notify("CopyApp", i18n.T("hk.fallback", name))
			}
			warned = false
			registered := time.Now()
			consumeHotkey(selected.Keydown(), selected.Keyup(), func() {
				log.Printf("hotkey received: %s", name)
				go sendClipboard("hotkey")
			})
			activeHotkey.Store("")
			signalTrayRefresh()
			log.Printf("hotkey listener ended unexpectedly; re-registering")
			diagnostics.Emit("hotkey", "listener", "closed", "")
			// Unregister before trying again; no accumulating deferred registrations.
			// The supported library owns its Windows registration/message thread.
			unregistered := make(chan error, 1)
			go func() { unregistered <- selected.Unregister() }()
			select {
			case err := <-unregistered:
				if err != nil {
					log.Printf("hotkey unregister: %v", err)
				}
			case <-time.After(2 * time.Second):
				// The library cannot unregister if its internal message pump has died.
				// Stop here rather than leak a new registration goroutine every retry.
				log.Printf("hotkey cleanup timed out; restart CopyApp; manual send remains available")
				machine.Notify("CopyApp", i18n.T("hk.restart"))
				return
			}
			if time.Since(registered) > time.Minute {
				delay = time.Second
			}
		}
		time.Sleep(delay)
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
}

// Drain both streams: the library queues Keyup indefinitely if never consumed.
// The streams have independent queues, so their delivery order cannot be used
// to infer physical held-key state or suppress a following legitimate press.
func consumeHotkey(down, up <-chan hotkey.Event, send func()) {
	for {
		select {
		case _, ok := <-down:
			if !ok {
				return
			}
			send()
		case _, ok := <-up:
			if !ok {
				return
			}
		}
	}
}

func startPairing() {
	srv.StartPairing()
	openBrowser(pairingURL(localKey))
}

// openControl opens the control window (it shows a fresh QR code by itself
// when no phone is paired).
func openControl() { openBrowser(pairingURL(localKey)) }

func onReady() {
	systray.SetIcon(makeICO(false))
	trayReady.Store(true)
	systray.SetTitle("CopyApp")
	mHome := systray.AddMenuItem("", "")
	mPair := systray.AddMenuItem("", "")
	mSend := systray.AddMenuItem("", "")
	mFile := systray.AddMenuItem("", "")
	mOpen := systray.AddMenuItem("", "")
	systray.AddSeparator()
	mAuto := systray.AddMenuItemCheckbox("", "", pc.AutostartEnabled())
	mRepair := systray.AddMenuItem("", "")
	mLang := systray.AddMenuItem(i18n.T("tray.lang"), "")
	langNames := map[string]string{"he": "עברית", "ru": "Русский", "en": "English"}
	langItems := map[string]*systray.MenuItem{}
	langClicked := make(chan string)
	for _, l := range i18n.Langs {
		it := mLang.AddSubMenuItemCheckbox(langNames[l], "", l == i18n.Lang())
		langItems[l] = it
		go func() {
			for range it.ClickedCh {
				langClicked <- l
			}
		}()
	}
	systray.AddSeparator()
	mUninstall := systray.AddMenuItem("", "")
	mQuit := systray.AddMenuItem("", "")
	relabel := func() {
		mHome.SetTitle(i18n.T("tray.openApp"))
		mFile.SetTitle(i18n.T("tray.sendFile"))
		mUninstall.SetTitle(i18n.T("tray.uninstall"))
		mSend.SetTooltip(activeHotkey.Load().(string))
		if pc.AutostartEnabled() {
			mAuto.Check()
		} else {
			mAuto.Uncheck()
		}
		hk := activeHotkey.Load().(string)
		if hk == "" {
			hk = i18n.T("hk.unavailable")
		}
		systray.SetTooltip(i18n.T("tray.tooltip", hk))
		mPair.SetTitle(i18n.T("tray.pair"))
		mPair.SetTooltip(i18n.T("tray.pairTip"))
		mSend.SetTitle(i18n.T("tray.send"))
		mOpen.SetTitle(i18n.T("tray.open"))
		mAuto.SetTitle(i18n.T("tray.autostart"))
		mRepair.SetTitle(i18n.T("tray.repair"))
		mRepair.SetTooltip(i18n.T("tray.repairTip"))
		mQuit.SetTitle(i18n.T("tray.quit"))
		for l, it := range langItems { // radio-like: exactly one checked
			if l == i18n.Lang() {
				it.Check()
			} else {
				it.Uncheck()
			}
		}
	}
	relabel()

	switch {
	case quietStart && !firstRun:
		// Started with Windows: stay quietly in the tray.
	case firstRun || openOnLaunch || !sess.Paired():
		if !sess.Paired() {
			srv.StartPairing()
		}
		go openControl()
	}
	go func() {
		for {
			select {
			case <-refreshTray:
				relabel()
			case <-mHome.ClickedCh:
				openControl()
			case <-mFile.ClickedCh:
				go func() {
					path, err := pc.ChooseFile(i18n.T("pick.file"))
					if err != nil {
						log.Printf("choose file: %v", err)
						return
					}
					if path != "" {
						sendFile(path)
					}
				}()
			case <-mUninstall.ClickedCh:
				go uninstallApp()
			case l := <-langClicked:
				i18n.Set(l)
				if _, err := prefs.Update(func(st *settings.Settings) { st.Language = l }); err != nil {
					log.Printf("save language: %v", err)
				}
				relabel()
			case <-mPair.ClickedCh:
				startPairing()
			case <-mSend.ClickedCh:
				go sendClipboard("tray")
			case <-mOpen.ClickedCh:
				if err := pc.OpenFolder(prefs.Get().SaveDir); err != nil {
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
				if err := srv.Unpair(); err != nil {
					log.Printf("unpair: could not save revocation: %v", err)
					pc.Info("CopyApp", i18n.T("home.revocationFail"), i18n.Lang() == "he")
					continue
				}
				startPairing()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func publishErrText(err error) string {
	if errors.Is(err, store.ErrOutboxFull) {
		return i18n.T("toast.outboxFull")
	}
	return err.Error()
}

func signalTrayRefresh() {
	select {
	case refreshTray <- struct{}{}:
	default:
	}
}

func sentFeedback(kind string, size int64) string {
	result := "queued"
	if srv.PhoneConnected() {
		result = "published"
	}
	log.Printf("outbox publish=%s kind=%s bytes=%d", result, kind, size)
	pc.Beep(true)
	pc.ShowSent(i18n.T("home.res."+result), i18n.Lang() == "he")
	flashTray()
	return result
}

// flashTray shows the tray icon with a green tick for a moment.
func flashTray() {
	if !trayReady.Load() {
		return
	}
	go func() {
		flashMu.Lock()
		defer flashMu.Unlock()
		systray.SetIcon(makeICO(true))
		time.Sleep(1200 * time.Millisecond)
		systray.SetIcon(makeICO(false))
	}()
}

var uninstallMu sync.Mutex

func uninstallApp() {
	if !uninstallMu.TryLock() {
		return
	}
	defer uninstallMu.Unlock()
	rtl := i18n.Lang() == "he"
	if !pc.Ask("CopyApp", i18n.T("un.confirm"), rtl, true) {
		return
	}
	paths := install.Default()
	// Only an installed executable may recursively remove its install folder.
	exe, err := os.Executable()
	if err != nil || !install.SamePath(exe, paths.Exe()) {
		pc.Info("CopyApp", i18n.T("un.notInstalled"), rtl)
		return
	}
	deleteData := pc.Ask("CopyApp", i18n.T("un.data", dataDir), rtl, true)
	// Received files belong to the user, including files in a custom folder.
	// Always preserve them; uninstall only removes application-owned data.
	if !install.SafeToDelete(paths.InstallDir) || deleteData && !install.SafeToDelete(dataDir) {
		log.Printf("uninstall refused unsafe path")
		return
	}
	if err := pc.SetAutostart(false); err != nil {
		log.Printf("uninstall startup: %v", err)
		return
	}
	// --setup creates shortcuts asynchronously. Removing them before its
	// child process exits can recreate shortcuts after a successful uninstall.
	shortcutSetup.Wait()
	if err := install.RemoveShortcuts(paths); err != nil {
		log.Printf("uninstall shortcuts: %v", err)
		return
	}
	if err := install.ScheduleUninstall(os.Getpid(), paths, dataDir, deleteData); err != nil {
		log.Printf("uninstall: %v", err)
		return
	}
	systray.Quit()
}
