package server

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/igalgerman/copyapp/internal/i18n"
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
	Paired      bool `json:"paired"` // a phone paired since this window opened
	SecondsLeft int  `json:"secondsLeft"`
}

func (s *Server) state() pairState {
	s.mu.Lock()
	opened, paired, tok := s.openedAt, s.pairedAt, s.pairToken
	s.mu.Unlock()
	st := pairState{Paired: !paired.IsZero() && !paired.Before(opened) && s.cfg.Auth.Paired()}
	if d := s.cfg.Auth.PairingDeadline(); tok != "" && !d.IsZero() {
		st.Open = true
		st.SecondsLeft = int(time.Until(d).Round(time.Second) / time.Second)
	}
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

func (s *Server) pairingPage(w http.ResponseWriter, r *http.Request) {
	nonce := randomNonce()
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; connect-src 'self'; "+
		"style-src 'nonce-"+nonce+"'; script-src 'nonce-"+nonce+"'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	st := s.state()
	// Rendered in the tray language; the page's script switches live and
	// remembers the visitor's choice in localStorage.
	lang := i18n.Lang()
	dict := i18n.Prefixed("pair.")
	str := map[string]template.HTML{}
	for k, v := range dict[lang] {
		str[k] = template.HTML(v) // our own strings; they may hold <b>/<code>
	}
	str["pair.expires"] = template.HTML(strings.Replace(dict[lang]["pair.expires"], "{s}",
		`<b id="left">`+strconv.Itoa(st.SecondsLeft)+`</b>`, 1))
	pairingTmpl.Execute(w, map[string]any{
		"Nonce":       nonce,
		"State":       st,
		"Fingerprint": fingerprint(s.cfg.CAPEM),
		"Short":       tlsutil.ShortFingerprint(s.cfg.CAPEM),
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
<title>{{index .S "pair.htmlTitle"}}</title>
<style nonce="{{.Nonce}}">
:root{--bg:#f4f5f7;--card:#fff;--fg:#1b1d22;--muted:#5d6370;--line:#e3e5ea;--accent:#2f6bff;--ok:#16a34a;--warn:#b45309}
@media (prefers-color-scheme:dark){:root{--bg:#121318;--card:#1c1e25;--fg:#eceef3;--muted:#a0a6b4;--line:#2c2f39;--accent:#7aa2ff;--ok:#4ade80;--warn:#fbbf24}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:var(--bg);color:var(--fg);
 font:16px/1.5 "Segoe UI",system-ui,sans-serif;padding:24px}
.card{background:var(--card);border:1px solid var(--line);border-radius:16px;max-width:760px;width:100%;padding:20px 32px 32px;
 display:grid;grid-template-columns:auto 1fr;gap:20px 32px;box-shadow:0 8px 30px rgba(0,0,0,.06)}
@media (max-width:640px){.card{grid-template-columns:1fr}}
.card>section{min-width:0}
.top{grid-column:1/-1;display:flex;align-items:center;justify-content:space-between;gap:12px}
.brand{font-weight:700;font-size:18px}
.langs{display:inline-flex;border:1px solid var(--line);border-radius:10px;padding:2px;gap:2px}
.langs button{background:transparent;color:var(--muted);padding:4px 12px;border-radius:8px;font-size:14px;font-weight:600}
.langs button[aria-pressed="true"]{background:var(--accent);color:#fff}
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
.note{color:var(--muted);font-size:14px}
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
button{font:inherit;background:var(--accent);color:#fff;border:0;border-radius:10px;padding:10px 18px;cursor:pointer}
button:focus-visible{outline:3px solid var(--accent);outline-offset:2px}
body.paired .qr,body.expired .qr,body.paired .timer,body.expired .timer{display:none}
body.paired #paired,body.expired #expired{display:flex}
</style>
</head>
<body class="{{if .State.Paired}}paired{{else if not .State.Open}}expired{{end}}">
<main class="card">
 <header class="top">
  <span class="brand">CopyApp</span>
  <div class="langs" role="group" aria-label="{{index .S "pair.lang"}}" id="langs">
   <button type="button" data-lang="he" lang="he" aria-pressed="false">עב</button>
   <button type="button" data-lang="ru" lang="ru" aria-pressed="false">РУ</button>
   <button type="button" data-lang="en" lang="en" aria-pressed="false">EN</button>
  </div>
 </header>
 <section class="qrbox" aria-live="polite">
  <div class="qr"><img src="/qr?{{.Stamp}}" alt="{{index .S "pair.qrAlt"}}" data-k-alt="pair.qrAlt" width="256" height="256"></div>
  <div class="timer" id="timer">{{index .S "pair.expires"}}</div>
  <div class="state" id="paired"><div class="big ok">&#10003;</div><div><b data-k="pair.paired">{{index .S "pair.paired"}}</b></div>
   <div class="note" data-k="pair.close">{{index .S "pair.close"}}</div></div>
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
  <details id="careful">
   <summary data-k="pair.carefulSum">{{index .S "pair.carefulSum"}}</summary>
   <div class="fplabel" data-k="pair.fpLabel">{{index .S "pair.fpLabel"}}</div>
   <div class="fpshort"><bdi dir="ltr">{{.Short}}</bdi></div>
   <p class="note" data-k="pair.fpNote">{{index .S "pair.fpNote"}}</p>
   <div class="fp"><span data-k="pair.fpFull">{{index .S "pair.fpFull"}}</span><br><bdi dir="ltr">{{.Fingerprint}}</bdi></div>
  </details>
  <details id="optional">
   <summary data-k="pair.optSum">{{index .S "pair.optSum"}}</summary>
   <p data-k="pair.opt">{{index .S "pair.opt"}}</p>
  </details>
 </section>
</main>
<script type="application/json" id="strings" nonce="{{.Nonce}}">{{.Dict}}</script>
<script nonce="{{.Nonce}}">
(function(){
 var b=document.body, root=document.documentElement, secs={{.State.SecondsLeft}};
 var dict=JSON.parse(document.getElementById('strings').textContent), lang=root.lang;
 function get(k){return (dict[lang]||{})[k]||dict.en[k]||k}
 function timer(){
  var t=document.getElementById('timer'), parts=get('pair.expires').split('{s}'), n=document.createElement('b');
  n.id='left'; n.textContent=secs;
  t.replaceChildren(parts[0], n, parts[1]||'');
 }
 function apply(l){
  if(!dict[l]) return;
  lang=l; root.lang=l; root.dir=l==='he'?'rtl':'ltr';
  document.title=get('pair.htmlTitle');
  document.querySelectorAll('[data-k]').forEach(function(e){e.innerHTML=get(e.dataset.k)});
  document.querySelectorAll('[data-k-alt]').forEach(function(e){e.alt=get(e.dataset.kAlt)});
  document.getElementById('langs').setAttribute('aria-label',get('pair.lang'));
  document.querySelectorAll('[data-lang]').forEach(function(e){e.setAttribute('aria-pressed',String(e.dataset.lang===l))});
  timer();
 }
 var saved=null; try{saved=localStorage.getItem('copyapp.lang')}catch(e){}
 apply(saved&&dict[saved]?saved:lang);
 document.getElementById('langs').addEventListener('click',function(e){
  var btn=e.target.closest('[data-lang]'); if(!btn) return;
  apply(btn.dataset.lang); try{localStorage.setItem('copyapp.lang',lang)}catch(e){}
 });
 function set(st){
  b.className = st.paired ? 'paired' : (st.open ? '' : 'expired');
  secs = Math.max(0, st.secondsLeft|0);
  var left=document.getElementById('left'); if(left) left.textContent=secs;
 }
 function poll(){
  fetch('/pairing/status',{cache:'no-store'}).then(function(r){return r.json()}).then(function(st){
   set(st); if(!st.paired) setTimeout(poll,1000);
  }).catch(function(){setTimeout(poll,3000)});
 }
 document.getElementById('again').addEventListener('click',function(){
  fetch('/pairing/open',{method:'POST'}).then(function(){location.reload()});
 });
 poll();
})();
</script>
</body>
</html>
`))
