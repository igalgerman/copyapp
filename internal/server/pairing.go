package server

import (
	"html/template"
	"net/http"
	"time"

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
	pairingTmpl.Execute(w, map[string]any{
		"Nonce":       nonce,
		"State":       st,
		"Fingerprint": fingerprint(s.cfg.CAPEM),
		"Stamp":       time.Now().UnixNano(),
	})
}

var pairingTmpl = template.Must(template.New("pairing").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pair your phone – CopyApp</title>
<style nonce="{{.Nonce}}">
:root{--bg:#f4f5f7;--card:#fff;--fg:#1b1d22;--muted:#5d6370;--line:#e3e5ea;--accent:#2f6bff;--ok:#16a34a;--warn:#b45309}
@media (prefers-color-scheme:dark){:root{--bg:#121318;--card:#1c1e25;--fg:#eceef3;--muted:#a0a6b4;--line:#2c2f39;--accent:#7aa2ff;--ok:#4ade80;--warn:#fbbf24}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:var(--bg);color:var(--fg);
 font:16px/1.5 "Segoe UI",system-ui,sans-serif;padding:24px}
.card{background:var(--card);border:1px solid var(--line);border-radius:16px;max-width:760px;width:100%;padding:32px;
 display:grid;grid-template-columns:auto 1fr;gap:32px;box-shadow:0 8px 30px rgba(0,0,0,.06)}
@media (max-width:640px){.card{grid-template-columns:1fr}}
h1{margin:0 0 4px;font-size:24px;font-weight:600}
.sub{color:var(--muted);margin:0 0 20px}
.qrbox{display:flex;flex-direction:column;align-items:center;gap:12px}
.qr{width:280px;height:280px;background:#fff;border-radius:12px;padding:12px;border:1px solid var(--line)}
.qr img{width:100%;height:100%;image-rendering:pixelated;display:block}
.timer{font-variant-numeric:tabular-nums;font-size:15px;color:var(--muted)}
.timer b{color:var(--fg);font-size:18px}
ol{margin:0;padding-left:20px}
ol li{margin:0 0 10px}
.note{color:var(--muted);font-size:14px}
.fp{font:12px/1.4 Consolas,monospace;color:var(--muted);word-break:break-all;margin-top:16px;padding-top:16px;border-top:1px solid var(--line)}
.state{display:none;flex-direction:column;align-items:center;justify-content:center;width:280px;height:280px;text-align:center;
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
 <section class="qrbox" aria-live="polite">
  <div class="qr"><img src="/qr?{{.Stamp}}" alt="Pairing QR code" width="256" height="256"></div>
  <div class="timer">Code expires in <b id="left">{{.State.SecondsLeft}}</b> s</div>
  <div class="state" id="paired"><div class="big ok">&#10003;</div><div><b>Phone paired</b></div>
   <div class="note">You can close this window.</div></div>
  <div class="state" id="expired"><div class="big warn">&#8635;</div><div><b>Code expired</b></div>
   <button id="again" type="button">Show a new code</button></div>
 </section>
 <section>
  <h1>Pair your phone</h1>
  <p class="sub">Phone and PC must be on the same Wi-Fi.</p>
  <ol>
   <li><b>Scan the code</b> with your Android phone's camera and open the link in Chrome.</li>
   <li>The first time, Chrome warns that the connection is not private. Tap <b>Advanced</b> &rarr; <b>Proceed</b>.
     <span class="note">This is expected until step 3.</span></li>
   <li><b>Install the CopyApp certificate</b> (once): in the app tap <b>Install certificate</b>, then open
     Settings &rarr; Security &amp; privacy &rarr; More security settings &rarr; Encryption &amp; credentials &rarr;
     Install a certificate &rarr; <b>CA certificate</b>, and pick <code>copyapp-ca.crt</code>.
     <span class="note">Menu names vary slightly between phone brands.</span></li>
   <li>Reopen the app in Chrome and choose <b>Add to Home screen</b> / <b>Install app</b>. CopyApp then appears in the share sheet.</li>
  </ol>
  <div class="fp">Certificate fingerprint (SHA-256), shown by Android when installing:<br>{{.Fingerprint}}</div>
 </section>
</main>
<script nonce="{{.Nonce}}">
(function(){
 var left=document.getElementById('left'), b=document.body;
 function set(st){
  b.className = st.paired ? 'paired' : (st.open ? '' : 'expired');
  left.textContent = Math.max(0, st.secondsLeft|0);
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
