// HTML mock-ups (illustrations) used in the how-to videos. Nothing here captures the real desktop.
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

const BASE = `
*{box-sizing:border-box}
body{margin:0;width:1280px;height:720px;overflow:hidden;font:16px "Segoe UI",system-ui,sans-serif;color:#1b1d22;
 background:linear-gradient(135deg,#cfd8f6,#e9edf6 55%,#fff1c2)}
body::after{content:'Illustration';position:fixed;top:6px;right:12px;padding:4px 10px;background:#fff;color:#4b5371;border-radius:5px;font:14px "Segoe UI";z-index:9999}
.win{position:absolute;background:#fff;border:1px solid #b9c0d4;border-radius:10px;box-shadow:0 18px 50px rgba(21,27,51,.28);overflow:hidden}
.tb{height:38px;background:#f1f3f9;border-bottom:1px solid #d9deea;display:flex;align-items:center;padding:0 14px;font-size:14px;color:#4b5371;gap:10px}
.tb i{width:12px;height:12px;border-radius:50%;background:#c9cfdf;display:inline-block}
.pulse{outline:4px solid #FFD447;outline-offset:3px;animation:p 1s infinite}
@keyframes p{50%{outline-color:#2E44E8}}
.btn{display:inline-block;padding:10px 22px;border-radius:6px;background:#2E44E8;color:#fff;font-weight:600}
.btn.alt{background:#e4e8f3;color:#1b1d22}
.hide{display:none!important}
.big{font-size:30px;font-weight:700}
`;
const page = (css, body, script = '') =>
  `<!doctype html><html><head><meta charset="utf-8"><style>${BASE}${css}</style></head><body>${body}<script>${script}</script></body></html>`;

// 1) a stand-in for the Releases page (NOT the real site)
exports.releases = (icon) => page(`
.win{left:90px;top:40px;width:1100px;height:640px}
.addr{margin-left:14px;background:#fff;border:1px solid #d3d9e8;border-radius:16px;padding:3px 18px;flex:1;color:#6e7693}
.body{padding:34px 60px}
h1{margin:0;font-size:34px} .tag{display:inline-block;background:#dcf5e6;color:#0E7A57;border-radius:20px;padding:2px 12px;font-size:14px;margin-left:12px;vertical-align:middle}
.assets{margin-top:30px;border:1px solid #d3d9e8;border-radius:10px}
.assets h3{margin:0;padding:12px 20px;background:#f4f6fb;border-bottom:1px solid #d3d9e8;font-size:16px}
.row{display:flex;align-items:center;gap:14px;padding:16px 20px;border-bottom:1px solid #eef0f6;font-size:19px}
.row:last-child{border:0} .row small{margin-left:auto;color:#6e7693;font-size:15px}
.row img{width:34px;height:34px;border-radius:8px}
.row.hit{background:#fffbe8}
.dl{position:absolute;right:40px;bottom:30px;width:380px;background:#fff;border:1px solid #b9c0d4;border-radius:10px;padding:16px 18px;box-shadow:0 10px 30px rgba(0,0,0,.25);transition:.4s;transform:translateY(160px);opacity:0}
.go .dl{transform:none;opacity:1}
.bar{height:8px;border-radius:4px;background:#e4e8f3;margin-top:10px;overflow:hidden}.bar b{display:block;height:100%;width:0;background:#2E44E8;transition:width 2.2s}.go .bar b{width:100%}
.note{position:absolute;left:0;right:0;bottom:6px;text-align:center;color:#6e7693;font-size:13px}
`, `<div class="win"><div class="tb"><i></i><i></i><i></i><span class="addr">copyapp.igalg.chatgpt.site (illustration)</span></div>
<div class="body"><h1>CopyApp <span class="tag">Latest</span></h1><p style="color:#4b5371;font-size:18px">Copy and paste between your PC and your Android phone.</p>
<div class="assets"><h3>Download for Windows</h3>
<div class="row" id="exe"><img src="${icon}"><b>copyapp.exe</b><small>about 12 MB</small></div>
<div class="row"><span style="width:34px"></span>Install and use videos<small></small></div></div></div>
<div class="dl"><b>copyapp.exe</b><div class="bar"><b></b></div><div style="margin-top:8px;color:#0E7A57" id="done"></div></div>
<div class="note">Illustration - get the real file from the CopyApp website</div></div>`,
`setTimeout(()=>document.getElementById('exe').classList.add('pulse','hit'),600);
window.click=()=>{document.body.classList.add('go');setTimeout(()=>document.getElementById('done').textContent='Downloaded - open it',2400)}`);

// 2) Downloads folder with copyapp.exe
exports.downloads = (icon) => page(`
.win{left:190px;top:80px;width:900px;height:520px}
.lst{padding:24px 30px}.lst h2{margin:0 0 18px;font-size:20px;color:#4b5371}
.it{display:inline-block;width:150px;text-align:center;font-size:16px}
.it img{width:76px;height:76px;border-radius:16px;display:block;margin:0 auto 8px}
.it.sel{background:#dcE1ff;border-radius:8px;padding:6px 0}
`, `<div class="win"><div class="tb"><i></i><i></i><i></i> This PC &rsaquo; Downloads</div><div class="lst"><h2>Downloads</h2>
<div class="it" id="it"><img src="${icon}">copyapp.exe</div></div></div>
<div id="hint" style="position:absolute;left:0;right:0;top:620px;text-align:center" class="big">Double-click copyapp.exe</div>`,
`setTimeout(()=>document.getElementById('it').classList.add('pulse','sel'),500)`);

// 3) Windows prompts (stylised illustrations of the two prompts people see)
exports.prompts = () => page(`
.win{left:290px;top:90px;width:700px}
.hd{background:#1b4fa8;color:#fff;padding:26px 30px;font-size:26px;font-weight:600}
.pd{padding:24px 30px;font-size:18px;line-height:1.5;color:#2b3045;min-height:150px}
.ft{padding:16px 30px 24px;text-align:right;background:#f4f6fb;display:flex;gap:14px;justify-content:flex-end}
.link{color:#1b4fa8;text-decoration:underline}
.chk{margin:10px 0;font-size:18px}
`, `
<div class="win" id="s1"><div class="hd">Windows protected your PC</div>
<div class="pd">A new app tried to start. This is normal for a brand-new app that Windows has not seen before.
<p id="mi" class="link">More info</p><p id="pub" class="hide"><b>App:</b> copyapp.exe</p></div>
<div class="ft"><span class="btn alt hide" id="run">Run anyway</span><span class="btn alt">Don't run</span></div></div>
<div class="win hide" id="s2"><div class="hd" style="background:#2b3045">Windows firewall</div>
<div class="pd">Allow <b>CopyApp</b> to talk on these networks:
<div class="chk">&#9745; <b>Private networks</b>, such as my home or work network</div>
<div class="chk">&#9745; Public networks (if this is your Windows network profile)</div></div>
<div class="ft"><span class="btn" id="allow">Allow access</span><span class="btn alt">Cancel</span></div></div>`,
`const $=i=>document.getElementById(i);
window.step=(n)=>{
 if(n==='a'){$('mi').classList.add('pulse')}
 if(n==='b'){$('mi').classList.add('hide');$('pub').classList.remove('hide');$('run').classList.remove('hide');$('run').classList.add('pulse')}
 if(n==='c'){$('s1').classList.add('hide');$('s2').classList.remove('hide');$('allow').classList.add('pulse')}
}`);

// 4) Notepad-like window showing real text
exports.startupChoice = () => page(`.win{left:290px;top:170px;width:700px;padding:32px}.actions{display:flex;gap:18px;margin-top:30px}`, `<div class="win"><h1>CopyApp is installed</h1><p>Desktop and Start Menu shortcuts are ready.</p><h2>Start CopyApp with Windows?</h2><p>You can change this later in Settings.</p><div class="actions"><span class="btn">Yes</span><span class="btn alt">No</span></div></div>`);

exports.notepad = (title, text, hint, big) => page(`
.win{left:190px;top:70px;width:900px;height:500px}
pre{margin:0;padding:26px 30px;font:26px/1.5 Consolas,monospace;white-space:pre-wrap;color:#151B33}
.caret{display:inline-block;width:2px;height:28px;background:#151B33;vertical-align:middle;animation:b 1s steps(1) infinite}@keyframes b{50%{opacity:0}}
.hintbar{position:absolute;left:0;right:0;top:600px;text-align:center}
.k{display:inline-block;border:2px solid #151B33;border-bottom-width:5px;border-radius:8px;padding:2px 16px;margin:0 4px;background:#fff;font-weight:700}
.k.on{background:#FFD447}
`, `<div class="win"><div class="tb"><i></i><i></i><i></i> ${esc(title)}</div><pre>${esc(text)}<span class="caret"></span></pre></div>
<div class="hintbar big">${hint || ''}</div>`);

// 5) Paint-like window showing a real clipboard image
exports.paint = (imgData, note) => page(`
.win{left:190px;top:50px;width:900px;height:560px}
.cv{background:#e9edf6;height:520px;display:flex;align-items:center;justify-content:center}
.cv img{max-width:600px;max-height:420px;border:1px solid #888;box-shadow:0 8px 24px rgba(0,0,0,.25)}
.hintbar{position:absolute;left:0;right:0;top:630px;text-align:center}
`, `<div class="win"><div class="tb"><i></i><i></i><i></i> Untitled - Paint (after Ctrl+V)</div><div class="cv"><img src="${imgData}"></div></div>
<div class="hintbar big">${esc(note || '')}</div>`);

// 6) Explorer listing with real file names
exports.folder = (path, names, note) => page(`
.win{left:190px;top:80px;width:900px;height:460px}
.lst{padding:20px 30px}.lst h2{margin:0 0 14px;font-size:20px;color:#4b5371}
.f{display:flex;align-items:center;gap:14px;padding:14px 16px;border-radius:8px;font-size:22px}
.f.new{background:#fffbe8;outline:3px solid #FFD447}
.ic{width:34px;height:40px;border-radius:5px;background:#dcE1ff;border:2px solid #2E44E8}
.hintbar{position:absolute;left:0;right:0;top:590px;text-align:center}
`, `<div class="win"><div class="tb"><i></i><i></i><i></i> ${esc(path)}</div><div class="lst"><h2>${esc(path)}</h2>
${names.map((n, i) => `<div class="f ${i === 0 ? 'new' : ''}"><span class="ic"></span>${esc(n)}</div>`).join('')}</div></div>
<div class="hintbar big">${esc(note || '')}</div>`);

// 7) Explorer with a right-click "Send to" menu
exports.sendto = (fileName) => page(`
.win{left:140px;top:60px;width:560px;height:520px}
.lst{padding:30px}.it{display:inline-block;width:160px;text-align:center;font-size:18px}
.doc{width:70px;height:88px;border:3px solid #2E44E8;border-radius:8px;background:#f2f4ff;margin:0 auto 8px}
.it.sel{background:#dcE1ff;border-radius:8px;padding:8px 0}
.menu{position:absolute;left:320px;top:230px;width:250px;background:#fff;border:1px solid #b9c0d4;border-radius:8px;box-shadow:0 12px 30px rgba(0,0,0,.3);padding:6px 0;font-size:17px}
.menu div{padding:8px 18px}.menu .m{background:#e4e8f3}
.sub{position:absolute;left:566px;top:330px;width:290px;background:#fff;border:1px solid #b9c0d4;border-radius:8px;box-shadow:0 12px 30px rgba(0,0,0,.3);padding:6px 0;font-size:17px}
.sub div{padding:8px 18px}
.menu,.sub{display:none}.s1 .menu{display:block}.s2 .sub{display:block}
.hintbar{position:absolute;left:0;right:0;top:620px;text-align:center}
`, `<div class="win"><div class="tb"><i></i><i></i><i></i> Documents</div><div class="lst"><div class="it sel"><div class="doc"></div>${esc(fileName)}</div></div></div>
<div class="menu"><div>Open</div><div>Copy</div><div class="m" id="st">Send to &#9656;</div><div>Delete</div></div>
<div class="sub"><div>Desktop</div><div>Mail recipient</div><div class="pulse" id="ph" style="background:#fffbe8"><b>Phone (CopyApp)</b></div></div>
<div class="hintbar big">Right-click the file</div>`,
`window.step=(n)=>{document.body.className=n==='menu'?'s1':'s1 s2';document.querySelector('.hintbar').textContent=n==='menu'?'Right-click the file, choose Send to':'Choose Phone (CopyApp)'}`);

// ---- phone side overlays (injected into the phone page) ----
exports.phoneCamera = (qr) => `<!doctype html><meta name=viewport content="width=device-width,initial-scale=1"><style>
*{box-sizing:border-box}body{margin:0;height:100vh;background:#0c0f1d;font:16px "Segoe UI",system-ui,sans-serif;color:#fff;overflow:hidden}
.v{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;background:radial-gradient(circle at 50% 40%,#2a3157,#0c0f1d)}
.qr{width:240px;height:240px;background:#fff;padding:10px;border-radius:10px;transform:rotate(-3deg)}.qr img{width:100%;height:100%;image-rendering:pixelated}
.fr{position:absolute;left:55px;top:290px;width:280px;height:280px;border:4px solid #FFD447;border-radius:24px;animation:z 1.4s infinite}
@keyframes z{50%{transform:scale(1.04)}}
.top{position:absolute;top:20px;left:0;right:0;text-align:center;font-weight:700;font-size:20px}
.chip{position:absolute;left:24px;right:24px;bottom:90px;background:#FFD447;color:#1E1A05;border-radius:18px;padding:16px 20px;font-weight:700;font-size:20px;text-align:center;transform:translateY(200px);transition:.5s}
.chip small{display:block;font-weight:400;font-size:14px;margin-top:4px}
.on .chip{transform:none}
</style><body><div class="v"><div class="qr"><img src="${qr}"></div></div><div class="fr"></div><div class="top">Camera</div>
<div class="chip" id="chip">Open link in Chrome<small>CopyApp - tap to open</small></div>
<script>window.found=()=>document.body.classList.add('on')</script></body>`;

exports.chromeMenu = (icon) => `
<style>
.ov{position:fixed;inset:0;z-index:2147483000;font:17px "Segoe UI",system-ui,sans-serif}
.ov .scrim{position:absolute;inset:0;background:rgba(0,0,0,.45)}
.ov .menu{position:absolute;right:10px;top:52px;width:250px;background:#fff;color:#151B33;border-radius:14px;box-shadow:0 12px 30px rgba(0,0,0,.4);padding:6px 0}
.ov .menu div{padding:12px 18px}.ov .menu .hit{background:#fffbe8;font-weight:700;outline:3px solid #FFD447;outline-offset:-3px}
.ov .dlg{position:absolute;left:24px;right:24px;top:230px;background:#fff;color:#151B33;border-radius:22px;padding:22px;box-shadow:0 12px 30px rgba(0,0,0,.4)}
.ov .dlg img{width:56px;height:56px;border-radius:12px;vertical-align:middle;margin-right:12px}
.ov .dlg .b{display:inline-block;margin:18px 0 0 10px;padding:10px 22px;border-radius:20px;background:#2E44E8;color:#fff;font-weight:700}
.ov .home{position:absolute;inset:0;background:linear-gradient(160deg,#2E44E8,#1d2eb8 60%,#10152A);color:#fff;text-align:center}
.ov .home .app{position:absolute;left:30px;top:90px;width:84px;font-size:14px}
.ov .home .app img{width:64px;height:64px;border-radius:16px;display:block;margin:0 auto 6px}
.ov .home .app.pulse{outline:4px solid #FFD447;outline-offset:6px;border-radius:18px}
.ov .home h2{position:absolute;left:0;right:0;bottom:60px;font-size:22px}
@keyframes q{50%{outline-color:#fff}}
</style>
<div class="ov" id="ov"><div class="scrim" id="scrim"></div>
<div class="menu" id="menu"><div>New tab</div><div>Share...</div><div class="hit" id="a2hs">Add to Home screen</div><div>Settings</div></div>
<div class="dlg" id="dlg" style="display:none"><div style="font-size:20px;font-weight:700"><img src="${icon}">Add to Home screen?</div>
<div style="text-align:right"><span class="b" style="background:#e4e8f3;color:#151B33">Cancel</span><span class="b" id="add">Add</span></div></div>
<div class="home" id="home" style="display:none"><div class="app pulse" id="app"><img src="${icon}">CopyApp</div><h2>CopyApp is on your home screen</h2></div></div>`;

exports.esc = esc;
