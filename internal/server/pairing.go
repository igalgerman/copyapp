package server

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/igalgerman/copyapp/internal/i18n"
	"github.com/igalgerman/copyapp/internal/secret"
	"github.com/igalgerman/copyapp/internal/settings"
	"github.com/igalgerman/copyapp/internal/tlsutil"
	qrcode "github.com/skip2/go-qrcode"
)

func fingerprint(caPEM []byte) string { return tlsutil.Fingerprint(caPEM) }

// qr renders the current pairing URL as a PNG. 410 when pairing is closed.
func (s *Server) qr(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	tok := s.pairToken
	s.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	if tok == "" || !s.cfg.Auth.PairingOpen() {
		http.Error(w, "pairing is not open", http.StatusGone)
		return
	}
	img, err := qrcode.Encode(s.pairURL(tok), qrcode.Medium, 360)
	if err != nil {
		s.cfg.Logf("qr: %v", err)
		http.Error(w, "qr failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(img)
}

type pairState struct {
	Open        bool `json:"open"`
	Paired      bool `json:"paired"` // a phone is paired and no new pairing window is open
	SecondsLeft int  `json:"secondsLeft"`
}

func (s *Server) state() pairState {
	s.mu.Lock()
	tok := s.pairToken
	s.mu.Unlock()
	var st pairState
	if d := s.cfg.Auth.PairingDeadline(); tok != "" && !d.IsZero() {
		st.Open = true
		st.SecondsLeft = int(time.Until(d).Round(time.Second) / time.Second)
	}
	st.Paired = !st.Open && s.cfg.Auth.Paired()
	return st
}

func (s *Server) pairingStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state())
}

// pairingOpen restarts the pairing window from the pairing page. Origin must
// match so other web pages cannot trigger it.
func (s *Server) pairingOpen(w http.ResponseWriter, r *http.Request) {
	o := r.Header.Get("Origin")
	if o != "https://"+r.Host && o != "http://"+r.Host {
		jsonError(w, http.StatusForbidden, "forbidden", "cross-origin request")
		return
	}
	s.StartPairing()
	w.WriteHeader(http.StatusNoContent)
}

// minSec formats seconds as m:ss for the pairing countdown.
func minSec(secs int) string {
	if secs < 0 {
		secs = 0
	}
	return strconv.Itoa(secs/60) + ":" + fmt.Sprintf("%02d", secs%60)
}

type sizeOpt struct {
	Value    int64
	Label    string
	Selected bool
}

// pageDict returns the strings the control window needs, per language.
func pageDict() map[string]map[string]string {
	d := i18n.Prefixed("pair.")
	for l, m := range i18n.Prefixed("home.") {
		for k, v := range m {
			d[l][k] = v
		}
	}
	return d
}

// pairingPage is the PC control window: status, pairing QR, active
// transfers and settings. (The path stays /pairing for old shortcuts.)
func (s *Server) localPage(w http.ResponseWriter, r *http.Request) {
	a, ok := remoteAddr(r)
	if !ok || !a.IsLoopback() || !isLocalHost(r.Host) {
		jsonError(w, http.StatusForbidden, "forbidden", "only available on this PC")
		return
	}
	token := ""
	authorized := s.cfg.LocalSecret != "" && (LocalAuthorized(r, s.cfg.LocalSecret) || s.localSessionValid(r))
	if k := r.URL.Query().Get("k"); k != "" {
		if !secret.Equal(k, s.cfg.LocalSecret) || s.cfg.LocalSecret == "" {
			jsonError(w, http.StatusForbidden, "forbidden", "open this page from the CopyApp tray menu")
			return
		}
		token, authorized = s.newLocalSession(), true
	}
	s.renderPairingPage(w, r, token, authorized)
}

func (s *Server) renderPairingPage(w http.ResponseWriter, r *http.Request, token string, authorized bool) {
	nonce := randomNonce()
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self' blob:; connect-src 'self'; "+
		"style-src 'nonce-"+nonce+"'; script-src 'nonce-"+nonce+"'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	// Refreshes load a generic shell. Only explicit credentials may expose PC data.
	snap := localState{Settings: settings.Defaults(""), Hotkey: "Ctrl+Alt+C"}
	if authorized {
		snap = s.snapshot()
	}
	st := snap.Pairing
	lang := snap.Settings.Language
	dict := pageDict()
	str := map[string]template.HTML{}
	for k, v := range dict[lang] {
		str[k] = template.HTML(v) // our own strings; they may hold <b>/<code>
	}
	str["pair.expires"] = template.HTML(strings.Replace(dict[lang]["pair.expires"], "{t}",
		`<b id="left" dir="ltr">`+minSec(st.SecondsLeft)+`</b>`, 1))
	wifi := snap.WiFi
	b1Key := "pair.b1"
	if wifi != "" {
		b1Key = "pair.b1Named"
		str[b1Key] = template.HTML(strings.Replace(dict[lang][b1Key], "{wifi}",
			`<b class="ssid"><bdi>`+html.EscapeString(wifi)+`</bdi></b>`, 1))
	}
	var sizes []sizeOpt
	for _, v := range settings.SizeChoices {
		sizes = append(sizes, sizeOpt{Value: v, Label: humanSize(v), Selected: v == snap.Settings.MaxBytes})
	}
	class := ""
	if st.Paired {
		class = "paired"
	} else if !st.Open {
		class = "expired"
	}
	pairingTmpl.Execute(w, map[string]any{
		"WiFi":        wifi,
		"B1Key":       b1Key,
		"Nonce":       nonce,
		"State":       st,
		"Class":       class,
		"Snap":        snap,
		"Folder":      snap.Settings.ReceiveMode == settings.ModeFolder,
		"Sizes":       sizes,
		"Fingerprint": snap.Fingerprint,
		"Short":       snap.ShortFingerprint,
		"LocalToken":  token,
		"Stamp":       time.Now().UnixNano(),
		"Lang":        lang,
		"Dir":         i18n.Dir(lang),
		"S":           str,
		"Dict":        dict,
	})
}

var pairingTmpl = template.Must(template.New("pairing").Parse(`<!doctype html>
<html lang="{{.Lang}}" dir="{{.Dir}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{index .S "home.htmlTitle"}}</title>
<style nonce="{{.Nonce}}">
:root{--bg:#f4f5f7;--card:#fff;--fg:#1b1d22;--muted:#5d6370;--line:#e3e5ea;--accent:#2f6bff;--accent-bg:#eaf0ff;--ok:#16a34a;--ok-bg:#e7f6ec;--warn:#b45309;--danger:#c0262d;--butter:#FFD447;--butter-bg:#FFF6D6;--butter-line:#E3AE00;--on-butter:#1E1A05}
@media (prefers-color-scheme:dark){:root{--bg:#121318;--card:#1c1e25;--fg:#eceef3;--muted:#a0a6b4;--line:#2c2f39;--accent:#7aa2ff;--accent-bg:#1f2840;--ok:#4ade80;--ok-bg:#15291d;--warn:#fbbf24;--danger:#ff7b80;--butter-bg:#2B250D;--butter-line:#FFD447;--on-butter:#FFF3C4}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;background:var(--bg);color:var(--fg);font:16px/1.5 "Segoe UI",system-ui,sans-serif;padding:24px 16px}
.wrap{max-width:800px;margin:0 auto;display:grid;gap:16px}
.card{background:var(--card);border:1px solid var(--line);border-radius:16px;padding:18px 24px 22px;box-shadow:0 8px 30px rgba(0,0,0,.05);min-width:0}
.card>h2{margin:0 0 12px;font-size:18px;font-weight:700}
.top{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}
.brand{font-weight:750;font-size:22px;display:flex;align-items:center;gap:10px}
.logo{width:30px;height:30px;border-radius:9px;background:var(--accent);display:inline-grid;place-items:center}
.logo svg{width:18px;height:18px;stroke:#fff}
.langs{display:inline-flex;border:1px solid var(--line);border-radius:10px;padding:2px;gap:2px;background:var(--card)}
.langs button{background:transparent;color:var(--muted);padding:4px 12px;border-radius:8px;font-size:14px;font-weight:600}
.langs button[aria-pressed="true"]{background:var(--accent);color:#fff}
button{font:inherit;background:var(--accent);color:#fff;border:0;border-radius:10px;padding:9px 16px;cursor:pointer}
button:focus-visible,select:focus-visible,input:focus-visible+.sw,label.opt:focus-within{outline:3px solid var(--accent);outline-offset:2px}
button.ghost{background:transparent;color:var(--accent);border:1px solid var(--line)}
button.danger{background:transparent;color:var(--danger);border:1px solid var(--line)}
button[disabled]{opacity:.6;cursor:progress}
.muted,.note{color:var(--muted)}
.note{font-size:14px}
/* status */
.facts{display:grid;grid-template-columns:repeat(auto-fit,minmax(210px,1fr));gap:12px 20px;margin:0}
.facts div{min-width:0}
.facts dt{font-size:13px;font-weight:600;color:var(--muted)}
.facts dd{margin:2px 0 0;font-weight:600;overflow-wrap:anywhere}
.dot{display:inline-block;width:10px;height:10px;border-radius:50%;background:var(--line);margin-inline-end:8px;vertical-align:middle}
.dot.on{background:var(--ok)}
.sub2{display:block;font-weight:400;font-size:13px;color:var(--muted)}
.statusbar{display:flex;gap:10px;flex-wrap:wrap;margin-top:14px}
kbd{font:600 13px Consolas,monospace;border:1px solid var(--line);border-bottom-width:2px;border-radius:6px;padding:1px 6px}
/* pairing (unchanged look) */
.pair{display:grid;grid-template-columns:auto 1fr;gap:20px 32px;padding:20px 32px 32px}
@media (max-width:640px){.pair{grid-template-columns:1fr;padding:16px 18px 22px}body{padding:12px}.before{padding:12px 14px 14px}}
.pair>section{min-width:0}
h1{margin:0 0 4px;font-size:26px;font-weight:650}
.sub{color:var(--muted);margin:0 0 20px}
.qrbox{display:flex;flex-direction:column;align-items:center;gap:12px}
.qr{width:340px;height:340px;max-width:80vw;max-height:80vw;background:#fff;border-radius:14px;padding:12px;border:1px solid var(--line)}
.qr img{width:100%;height:100%;image-rendering:pixelated;display:block}
.timer{font-variant-numeric:tabular-nums;font-size:15px;color:var(--muted)}
.timer b{color:var(--fg);font-size:18px}
ol.steps{list-style:none;margin:0;padding:0;display:grid;gap:16px}
ol.steps li{display:grid;grid-template-columns:40px 1fr;gap:14px;align-items:start;font-size:18px;line-height:1.45}
.n{display:inline-grid;place-items:center;width:40px;height:40px;border-radius:50%;background:var(--accent);color:#fff;font-weight:700;font-size:19px}
ol.steps li span{padding-top:6px}
.ready{margin:18px 0 0;color:var(--ok);font-weight:600}
details{margin-top:14px;border:1px solid var(--line);border-radius:12px;padding:10px 14px}
details+details{margin-top:10px}
summary{cursor:pointer;font-size:14px;font-weight:600;color:var(--muted)}
details[open] summary{margin-bottom:8px}
details p{margin:0 0 8px;font-size:14px}
.fp{font-size:12px;line-height:1.4;color:var(--muted);margin-top:8px}
.fp bdi{font-family:Consolas,monospace;word-break:break-all}
.fplabel{font-size:13px;font-weight:600;color:var(--muted)}
.fpshort{font:700 22px/1.25 Consolas,monospace;letter-spacing:.04em;margin:2px 0 6px;white-space:nowrap}
@media (max-width:420px){.fpshort{font-size:19px}}
.state{display:none;flex-direction:column;align-items:center;justify-content:center;width:340px;height:340px;max-width:80vw;text-align:center;
 border:1px dashed var(--line);border-radius:12px;gap:12px;padding:16px}
.big{font-size:48px;line-height:1}
.ok{color:var(--ok)} .warn{color:var(--warn)}
.before{grid-column:1/-1;background:var(--butter-bg);color:var(--on-butter);border:2px solid var(--butter-line);border-radius:14px;padding:14px 18px 16px}
.before h2{margin:0 0 12px;font-size:19px;font-weight:700;display:flex;align-items:center;gap:10px}
.badge{display:inline-grid;place-items:center;width:30px;height:30px;border-radius:50%;background:var(--butter);color:#1E1A05;flex:none}
.badge svg{width:18px;height:18px}
.before ul{list-style:none;margin:0;padding:0;display:grid;grid-template-columns:repeat(3,1fr);gap:12px 28px}
@media (max-width:640px){.before ul{grid-template-columns:1fr}}
.before li{display:grid;grid-template-columns:24px 1fr;gap:10px;align-items:start;font-size:15px;line-height:1.45}
.before li>svg{width:22px;height:22px;margin-top:1px}
.ssid{font-weight:800;white-space:nowrap}
.before bdi,#addr{white-space:nowrap}
.ic{fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round;stroke-linejoin:round}
details.trouble{border:2px solid var(--butter-line)}
details.trouble summary{color:var(--fg);font-size:15px}
details.trouble p{line-height:1.45}
body.paired #pair{display:none}
body.expired .qr,body.expired .timer{display:none}
body.expired #expired{display:flex}
body.offline .live{opacity:.45;pointer-events:none}
#offline{display:none;background:var(--butter-bg);color:var(--on-butter);border:2px solid var(--butter-line);border-radius:14px;padding:12px 16px;font-weight:600}
body.offline #offline{display:block}
/* transfers */
.tlist{list-style:none;margin:0;padding:0;display:grid;gap:12px}
.tlist li.t{display:grid;grid-template-columns:1fr auto;gap:4px 12px;align-items:center}
.tname{font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.tdir{font-size:13px;color:var(--muted)}
.tpct{font-variant-numeric:tabular-nums;font-weight:700}
.tlist progress{grid-column:1/-1;width:100%;height:10px;appearance:none;border:0;border-radius:6px;background:var(--line);overflow:hidden}
.tlist progress::-webkit-progress-bar{background:var(--line);border-radius:6px}
.tlist progress::-webkit-progress-value{background:var(--accent);border-radius:6px}
.tlist progress::-moz-progress-bar{background:var(--accent)}
.tlist:empty{display:none}
.tlist:not(:empty)+#tempty{display:none}
#tempty{margin:0}
/* settings */
.row{display:grid;grid-template-columns:minmax(150px,220px) 1fr;gap:8px 20px;padding:14px 0;border-top:1px solid var(--line);align-items:start}
.row:first-of-type{border-top:0;padding-top:4px}
@media (max-width:640px){.row{grid-template-columns:1fr}}
.row>.lbl{font-weight:650;padding-top:6px}
.opts{display:grid;gap:10px;grid-template-columns:repeat(auto-fit,minmax(210px,1fr))}
label.opt{display:grid;grid-template-columns:auto 1fr;align-content:start;gap:2px 10px;border:1px solid var(--line);border-radius:12px;padding:10px 12px;cursor:pointer;align-items:start}
label.opt:has(input:checked){border-color:var(--accent);background:var(--accent-bg)}
label.opt input{margin:4px 0 0;accent-color:var(--accent)}
label.opt b{font-weight:650}
label.opt span.note{grid-column:2}
.path{font:14px Consolas,monospace;background:var(--bg);border:1px solid var(--line);border-radius:8px;padding:7px 10px;overflow-wrap:anywhere;margin-bottom:10px}
.btns{display:flex;gap:10px;flex-wrap:wrap}
select{font:inherit;color:var(--fg);background:var(--card);border:1px solid var(--line);border-radius:10px;padding:8px 12px;min-width:160px}
.switch{display:flex;align-items:center;gap:12px;cursor:pointer}
.switch input{position:absolute;opacity:0;width:1px;height:1px}
.sw{width:46px;height:26px;border-radius:13px;background:var(--line);position:relative;flex:none;transition:background .15s}
.sw::after{content:"";position:absolute;top:3px;inset-inline-start:3px;width:20px;height:20px;border-radius:50%;background:#fff;box-shadow:0 1px 3px rgba(0,0,0,.3);transition:inset-inline-start .15s}
.switch input:checked+.sw{background:var(--ok)}
.switch input:checked+.sw::after{inset-inline-start:23px}
#unNote{display:none;margin:8px 0 0}
#unNote.show{display:block}
#msg{position:fixed;bottom:16px;left:16px;right:16px;margin:0 auto;max-width:560px;background:var(--fg);color:var(--bg);border-radius:12px;padding:12px 16px;font-weight:600;box-shadow:0 8px 30px rgba(0,0,0,.25);z-index:5}
#msg:empty{display:none}
.warnline{margin:12px 0 0;padding:8px 12px;border-radius:10px;background:var(--accent-bg);color:var(--fg)}
#msg.bad{background:var(--danger);color:#fff}
footer{font-size:13px;color:var(--muted);padding:0 8px 12px;display:grid;gap:4px}
footer bdi{font-family:Consolas,monospace;overflow-wrap:anywhere}
</style>
</head>
<body data-wifi="{{.WiFi}}" class="{{.Class}}">
<div class="wrap">
<header class="top">
 <span class="brand"><span class="logo"><svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><rect x="8" y="8" width="12" height="13" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v11a2 2 0 0 0 2 2h2"/></svg></span>CopyApp</span>
 <div class="langs" role="group" aria-label="{{index .S "pair.lang"}}" id="langs">
  <button type="button" data-lang="he" lang="he" aria-pressed="{{if eq .Lang "he"}}true{{else}}false{{end}}">עב</button>
  <button type="button" data-lang="ru" lang="ru" aria-pressed="{{if eq .Lang "ru"}}true{{else}}false{{end}}">РУ</button>
  <button type="button" data-lang="en" lang="en" aria-pressed="{{if eq .Lang "en"}}true{{else}}false{{end}}">EN</button>
 </div>
</header>
<div id="offline" role="alert" data-k="home.offline">{{index .S "home.offline"}}</div>

<section class="card live" id="status" aria-labelledby="st-h">
 <h2 id="st-h" data-k="home.status">{{index .S "home.status"}}</h2>
 <dl class="facts">
  <div><dt data-k="home.phone">{{index .S "home.phone"}}</dt>
   <dd><span class="dot{{if .Snap.Phone.Paired}} on{{end}}" id="phoneDot"></span><span id="phoneState">{{if .Snap.Phone.Paired}}{{index .S "home.paired"}}{{else}}{{index .S "home.notPaired"}}{{end}}</span><span class="sub2" id="phoneSeen"></span></dd></div>
  <div><dt data-k="home.wifi">{{index .S "home.wifi"}}</dt><dd id="wifi">{{if .WiFi}}<bdi>{{.WiFi}}</bdi>{{else}}{{index .S "home.noWifi"}}{{end}}</dd></div>
  <div><dt data-k="home.address">{{index .S "home.address"}}</dt><dd><bdi dir="ltr" id="addr">{{.Snap.Address}}</bdi></dd></div>
  <div><dt data-k="home.hotkey">{{index .S "home.hotkey"}}</dt><dd><kbd dir="ltr" id="hotkey">{{.Snap.Hotkey}}</kbd></dd></div>
 </dl>
 <p class="note warnline" id="pairedFrom" hidden></p>
 <div class="statusbar">
  <button type="button" id="sendNow" data-k="home.sendNow">{{index .S "home.sendNow"}}</button>
  <button type="button" class="ghost" id="repair" data-k="home.repair"{{if not .Snap.Phone.Paired}} hidden{{end}}>{{index .S "home.repair"}}</button>
 </div>
</section>

<main class="card pair live" id="pair">
 <section class="before" aria-labelledby="before-h">
  <h2 id="before-h"><span class="badge"><svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 6v8M12 18h.01" stroke-width="3"/></svg></span><span data-k="pair.beforeTitle">{{index .S "pair.beforeTitle"}}</span></h2>
  <ul>
   <li class="b"><svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><path d="M2 9a15 15 0 0 1 20 0M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0M12 19.5h.01"/></svg><span data-k="{{.B1Key}}">{{index .S .B1Key}}</span></li>
   <li class="b"><svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 16.5h.01M11 16.5h.01M17 13V5"/></svg><span data-k="pair.b2">{{index .S "pair.b2"}}</span></li>
   <li class="b"><svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg><span data-k="pair.b3">{{index .S "pair.b3"}}</span></li>
  </ul>
 </section>
 <section class="qrbox" aria-live="polite">
  <div class="qr"><img id="qrimg" alt="{{index .S "pair.qrAlt"}}" data-k-alt="pair.qrAlt" width="256" height="256"></div>
  <div class="timer" id="timer">{{index .S "pair.expires"}}</div>
  <div class="state" id="expired"><div class="big warn">&#8635;</div><div><b data-k="pair.expired">{{index .S "pair.expired"}}</b></div>
   <button id="again" type="button" data-k="pair.again">{{index .S "pair.again"}}</button></div>
 </section>
 <section>
  <h1 data-k="pair.h1">{{index .S "pair.h1"}}</h1>
  <p class="sub" data-k="pair.sub">{{index .S "pair.sub"}}</p>
  <ol class="steps">
   <li><b class="n">1</b><span data-k="pair.s1">{{index .S "pair.s1"}}</span></li>
   <li><b class="n">2</b><span data-k="pair.s2">{{index .S "pair.s2"}}</span></li>
  </ol>
  <p class="ready" data-k="pair.ready">{{index .S "pair.ready"}}</p>
  <details id="trouble" class="trouble">
   <summary data-k="pair.tSum">{{index .S "pair.tSum"}}</summary>
   <p data-k="pair.t1">{{index .S "pair.t1"}}</p>
   <p data-k="pair.t2">{{index .S "pair.t2"}}</p>
   <p data-k="pair.t3">{{index .S "pair.t3"}}</p>
   <p data-k="pair.t4">{{index .S "pair.t4"}}</p>
  </details>
  <details id="careful">
   <summary data-k="pair.carefulSum">{{index .S "pair.carefulSum"}}</summary>
   <div class="fplabel" data-k="pair.fpLabel">{{index .S "pair.fpLabel"}}</div>
   <div class="fpshort"><bdi dir="ltr" id="shortFP">{{.Short}}</bdi></div>
   <p class="note" data-k="pair.fpNote">{{index .S "pair.fpNote"}}</p>
   <div class="fp"><span data-k="pair.fpFull">{{index .S "pair.fpFull"}}</span><br><bdi dir="ltr" id="fullFP">{{.Fingerprint}}</bdi></div>
  </details>
  <details id="optional">
   <summary data-k="pair.optSum">{{index .S "pair.optSum"}}</summary>
   <p data-k="pair.opt">{{index .S "pair.opt"}}</p>
  </details>
 </section>
</main>

<section class="card live" id="transfers" aria-labelledby="tr-h">
 <h2 id="tr-h" data-k="home.transfers">{{index .S "home.transfers"}}</h2>
 <ul class="tlist" id="tlist" aria-live="polite"></ul>
 <p class="note" id="tempty" data-k="home.noTransfers">{{index .S "home.noTransfers"}}</p>
</section>

<section class="card live" id="settings" aria-labelledby="set-h">
 <h2 id="set-h" data-k="home.settings">{{index .S "home.settings"}}</h2>
 <div class="row"><label class="lbl" for="langSel" data-k="home.language">{{index .S "home.language"}}</label>
  <div><select id="langSel"><option value="he" lang="he"{{if eq .Lang "he"}} selected{{end}}>עברית</option><option value="ru" lang="ru"{{if eq .Lang "ru"}} selected{{end}}>Русский</option><option value="en" lang="en"{{if eq .Lang "en"}} selected{{end}}>English</option></select></div></div>
 <div class="row" role="radiogroup" aria-labelledby="mode-l"><div class="lbl" id="mode-l" data-k="home.mode">{{index .S "home.mode"}}</div>
  <div class="opts">
   <label class="opt"><input type="radio" name="mode" value="clipboard"{{if not .Folder}} checked{{end}}><b data-k="home.modeClip">{{index .S "home.modeClip"}}</b><span class="note" data-k="home.modeClipHint">{{index .S "home.modeClipHint"}}</span></label>
   <label class="opt"><input type="radio" name="mode" value="folder"{{if .Folder}} checked{{end}}><b data-k="home.modeFolder">{{index .S "home.modeFolder"}}</b><span class="note" data-k="home.modeFolderHint">{{index .S "home.modeFolderHint"}}</span></label>
  </div></div>
 <div class="row"><div class="lbl" data-k="home.folder">{{index .S "home.folder"}}</div>
  <div><div class="path"><bdi dir="ltr" id="saveDir">{{.Snap.Settings.SaveDir}}</bdi></div>
   <div class="btns"><button type="button" id="choose" data-k="home.choose">{{index .S "home.choose"}}</button>
    <button type="button" class="ghost" id="openDir" data-k="home.openFolder">{{index .S "home.openFolder"}}</button></div></div></div>
 <div class="row"><label class="lbl" for="maxSel" data-k="home.max">{{index .S "home.max"}}</label>
  <div><select id="maxSel">{{range .Sizes}}<option value="{{.Value}}"{{if .Selected}} selected{{end}}>{{.Label}}</option>{{end}}</select>
   <p class="note" data-k="home.maxHint">{{index .S "home.maxHint"}}</p></div></div>
 <div class="row"><div class="lbl" data-k="home.autostart">{{index .S "home.autostart"}}</div>
  <div><label class="switch"><input type="checkbox" role="switch" id="auto"{{if .Snap.Autostart}} checked{{end}}><span class="sw" aria-hidden="true"></span><span data-k="home.autostartHint">{{index .S "home.autostartHint"}}</span></label></div></div>
 <div class="row"><div class="lbl" data-k="home.app">{{index .S "home.app"}}</div>
  <div><div class="btns"><button type="button" class="danger" id="uninstall" data-k="home.uninstall">{{index .S "home.uninstall"}}</button></div>
   <p class="note" id="unNote" data-k="home.uninstallAsk">{{index .S "home.uninstallAsk"}}</p></div></div>
</section>

<footer>
 <div><span data-k="home.log">{{index .S "home.log"}}</span> <bdi dir="ltr" id="logPath">{{.Snap.LogPath}}</bdi></div>
 {{if .Snap.Version}}<div>CopyApp · <span data-k="home.version">{{index .S "home.version"}}</span> <bdi dir="ltr" id="version">{{.Snap.Version}}</bdi></div>{{end}}
</footer>
</div>
<p id="msg" role="status" aria-live="polite"></p>
<script type="application/json" id="strings" nonce="{{.Nonce}}">{{.Dict}}</script>
<script nonce="{{.Nonce}}">
(function(){
 var token={{.LocalToken}};
 try{if(token) sessionStorage.setItem('copyapp.localSession',token); else token=sessionStorage.getItem('copyapp.localSession')||''}catch(e){}
 if(location.search) history.replaceState(null,'','/pairing');
 function controlFetch(url,opts){opts=opts||{};opts.headers=Object.assign({},opts.headers,{'X-CopyApp-Session':token});return fetch(url,opts)}
 var qrURL='';
 function loadQR(){return controlFetch('/qr',{cache:'no-store'}).then(function(r){if(!r.ok) return;return r.blob()}).then(function(blob){if(!blob)return;if(qrURL)URL.revokeObjectURL(qrURL);qrURL=URL.createObjectURL(blob);document.getElementById('qrimg').src=qrURL})}
 var b=document.body, root=document.documentElement, secs={{.State.SecondsLeft}}, wasOpen={{.State.Open}};
 var dict=JSON.parse(document.getElementById('strings').textContent), lang=root.lang, busy=0, last=null, autoOpened=false;
 function $(id){return document.getElementById(id)}
 function get(k){return (dict[lang]||{})[k]||dict.en[k]||k}
 function fill(k,o){var s=get(k);for(var x in o){s=s.split('{'+x+'}').join('\u2068'+o[x]+'\u2069')}return s}
 function ms(v){return Math.floor(v/60)+':'+String(v%60).padStart(2,'0')}
 function size(n){
  var nf=new Intl.NumberFormat(lang,{maximumFractionDigits:n>=1073741824||n<10485760?1:0});
  if(n>=1073741824) return fill('home.gb',{n:nf.format(n/1073741824)});
  return fill('home.mb',{n:nf.format(n/1048576)});
 }
 function ago(t){
  var s=Math.round((Date.now()-new Date(t).getTime())/1000), rtf=new Intl.RelativeTimeFormat(lang,{numeric:'auto'});
  if(s<45) return rtf.format(0,'second');
  if(s<3600) return rtf.format(-Math.round(s/60),'minute');
  if(s<86400) return rtf.format(-Math.round(s/3600),'hour');
  return rtf.format(-Math.round(s/86400),'day');
 }
 function timer(){
  var t=$('timer'), parts=get('pair.expires').split('{t}'), n=document.createElement('b');
  n.id='left'; n.dir='ltr'; n.textContent=ms(secs);
  t.replaceChildren(parts[0], n, parts[1]||'');
 }
 function apply(l){
  if(!dict[l]) return;
  lang=l; root.lang=l; root.dir=l==='he'?'rtl':'ltr';
  document.title=get('home.htmlTitle');
  document.querySelectorAll('[data-k]').forEach(function(e){
   var h=get(e.dataset.k);
   if(h.indexOf('{wifi}')<0){e.innerHTML=h;return}
   e.innerHTML=h.replace('{wifi}','<b class="ssid"><bdi></bdi></b>');
   e.querySelector('.ssid bdi').textContent=b.dataset.wifi||'';
  });
  document.querySelectorAll('[data-k-alt]').forEach(function(e){e.alt=get(e.dataset.kAlt)});
  $('langs').setAttribute('aria-label',get('pair.lang'));
  document.querySelectorAll('[data-lang]').forEach(function(e){e.setAttribute('aria-pressed',String(e.dataset.lang===l))});
  $('langSel').value=l;
  Array.prototype.forEach.call($('maxSel').options,function(o){o.textContent=size(+o.value)});
  timer();
  if(last) render(last);
 }
 function post(url,body){
  busy++;
  return controlFetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body||{})})
   .then(function(r){if(!r.ok) return r.json().catch(function(){return {}}).then(function(j){throw {status:r.status,code:j.code}}); return r.status===200?r.json():null})
   .finally(function(){busy--});
 }
 var msgT=0;
 function say(k,bad){var m=$('msg'); m.textContent=get(k); m.className=bad?'bad':''; clearTimeout(msgT); msgT=setTimeout(function(){m.textContent=''},bad?8000:4000)}
 function failed(e){if(e&&e.code==='session_save'){alert(get('home.revocationFail'));poll1();return}say(e&&e.status===400?'home.badFolder':'home.saveFail',true); poll1()}
 function setLang(l){apply(l); post('/local/settings',{language:l}).then(render).catch(failed)}
 function render(s){

  if(!s.pairing) return;
  last=s;
  $('shortFP').textContent=s.shortFingerprint||''; $('fullFP').textContent=s.fingerprint||'';
  var p=s.pairing;
  b.classList.toggle('paired',p.paired); b.classList.toggle('expired',!p.paired&&!p.open);
  if(p.open&&!wasOpen) loadQR();
  // No phone yet and the code ran out (e.g. CopyApp started with Windows):
  // show a fresh code instead of asking for a click.
  if(!autoOpened&&!p.open&&!p.paired&&!s.phone.paired){autoOpened=true; controlFetch('/pairing/open',{method:'POST'}).then(function(){loadQR(); wasOpen=true; poll1()}).catch(function(){})}
  wasOpen=p.open; secs=Math.max(0,p.secondsLeft|0);
  var left=$('left'); if(left) left.textContent=ms(secs);
  $('phoneDot').classList.toggle('on',s.phone.paired&&s.phone.connected);
  $('phoneState').textContent=get(s.phone.paired?'home.paired':'home.notPaired');
  $('phoneSeen').textContent=!s.phone.paired?'':s.phone.connected?get('home.connected'):s.phone.lastSeen?fill('home.lastSeen',{t:ago(s.phone.lastSeen)}):get('home.notSeen');
  $('repair').hidden=!s.phone.paired;
  var pf=$('pairedFrom');
  if(s.phone.paired&&s.phone.pairedFrom){pf.textContent=fill('home.pairedFrom',{ip:s.phone.pairedFrom,t:new Date(s.phone.pairedAt).toLocaleString(lang,{dateStyle:'short',timeStyle:'short'})}); pf.hidden=false}else{pf.hidden=true}
  if(s.wifi){var bd=document.createElement('bdi');bd.textContent=s.wifi;$('wifi').replaceChildren(bd)}else{$('wifi').textContent=get('home.noWifi')}
  b.dataset.wifi=s.wifi||'';
  $('addr').textContent=s.address; $('hotkey').textContent=s.hotkey; $('logPath').textContent=s.logPath;
  if(!busy){
   var st=s.settings;
   document.querySelectorAll('input[name=mode]').forEach(function(r){r.checked=r.value===st.receiveMode});
   $('saveDir').textContent=st.saveDir; $('maxSel').value=String(st.maxBytes); $('auto').checked=!!s.autostart;
   if(st.language!==lang) apply(st.language);
  }
  var list=$('tlist'), items=(s.transfers||[]).map(function(t){
   var li=document.createElement('li'); li.className='t';
   var name=document.createElement('div'); name.className='tname';
   var bd=document.createElement('bdi'); bd.textContent=t.name||'…'; name.append(bd);
   var dir=document.createElement('span'); dir.className='tdir sub2'; dir.textContent=get(t.dir==='in'?'home.in':'home.out');
   name.append(dir);
   var pct=document.createElement('span'); pct.className='tpct'; pct.dir='ltr';
   var bar=document.createElement('progress'); bar.max=100;
   if(t.total>0){var v=Math.min(100,Math.floor(t.done*100/t.total)); bar.value=v; pct.textContent=v+'%'; bar.setAttribute('aria-label',(t.name||'')+' '+v+'%')}
   else{pct.textContent=size(t.done); bar.removeAttribute('value')}
   li.append(name,pct,bar); return li;
  });
  list.replaceChildren.apply(list,items);
 }
 function poll(){
  controlFetch('/local/state',{cache:'no-store'}).then(function(r){if(!r.ok) throw r; return r.json()}).then(function(s){
   b.classList.remove('offline'); render(s); setTimeout(poll,1000);
  }).catch(function(e){
   // 403: CopyApp restarted and forgot this window's session; anything else: not running.
   var k=e&&e.status===403?'home.reopen':'home.offline', o=$('offline');
   o.dataset.k=k; o.textContent=get(k); b.classList.add('offline'); setTimeout(poll,3000)});
 }
 $('langs').addEventListener('click',function(e){var btn=e.target.closest('[data-lang]'); if(btn) setLang(btn.dataset.lang)});
 $('langSel').addEventListener('change',function(){setLang(this.value)});
 document.querySelectorAll('input[name=mode]').forEach(function(r){r.addEventListener('change',function(){post('/local/settings',{receiveMode:this.value}).then(render).catch(failed)})});
 $('maxSel').addEventListener('change',function(){post('/local/settings',{maxBytes:+this.value}).then(render).catch(failed)});
 $('auto').addEventListener('change',function(){var c=this; post('/local/settings',{autostart:c.checked}).then(render).catch(function(){c.checked=!c.checked; say('home.autostartFail',true)})});
 $('choose').addEventListener('click',function(){var btn=this; btn.disabled=true;
  post('/local/choose-folder').then(function(r){if(r) $('saveDir').textContent=r.saveDir}).catch(failed).finally(function(){btn.disabled=false})});
 $('openDir').addEventListener('click',function(){post('/local/open-folder').catch(function(){})});
 $('sendNow').addEventListener('click',function(){var btn=this; btn.disabled=true;
  post('/local/send-clipboard').then(function(r){var k='home.res.'+((r&&r.result)||'failed'); say(k,k!=='home.res.sent')}).catch(function(){say('home.res.failed',true)}).finally(function(){btn.disabled=false})});
 $('repair').addEventListener('click',function(){if(!confirm(get('home.repairConfirm'))) return; post('/local/repair').then(function(){loadQR(); wasOpen=true}).catch(failed)});
 $('uninstall').addEventListener('click',function(){post('/local/uninstall').then(function(){$('unNote').classList.add('show')}).catch(function(){})});
 $('again').addEventListener('click',function(){
  controlFetch('/pairing/open',{method:'POST'}).then(function(){loadQR(); wasOpen=true; poll1()});
 });
 function poll1(){controlFetch('/local/state',{cache:'no-store'}).then(function(r){return r.json()}).then(render).catch(function(){})}
 apply(lang);
 loadQR();
 poll();
})();
</script>
</body>
</html>
`))
