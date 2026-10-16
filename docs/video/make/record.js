// Records the raw PC + phone clips for one scenario ("install" or "use") against the REAL running copyapp.exe.
// Called by make.ps1, which starts the exe with a temp APPDATA and sets the env vars below.
//   env: COPYAPP_LPORT (loopback port), COPYAPP_EXE, RAW_DIR, WEB_DIR (internal/server/web), NODE_PATH (playwright, jsqr, pngjs)
const { chromium } = require('playwright');
const fs = require('fs'), path = require('path'), cp = require('child_process');
const { PNG } = require('pngjs'); const jsQR = require('jsqr');
const mocks = require('./mocks');

const scenario = process.argv[2];
const ILLUSTRATE_NATIVE = process.env.COPYAPP_ILLUSTRATE_NATIVE === '1';
if (!['install', 'use'].includes(scenario)) throw new Error('Expected install or use');
const LPORT = process.env.COPYAPP_LPORT, EXE = process.env.COPYAPP_EXE, RAW = path.join(process.env.RAW_DIR, scenario), WEB = process.env.WEB_DIR;
const L = `http://127.0.0.1:${LPORT}`;
// per-launch local key: the PC-only routes need it (header on fetches, ?k= to open a page in the browser)
const KEY = fs.readFileSync(path.join(process.env.COPYAPP_APPDATA, 'CopyApp', 'local.key'), 'utf8').trim();
const LH = { 'X-CopyApp': KEY };
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
if (process.env.COPYAPP_WIFI_NAME !== 'Home Wi-Fi') throw new Error('Public recordings require COPYAPP_WIFI_NAME=Home Wi-Fi');
const dataUri = (file, mime) => `data:${mime};base64,` + fs.readFileSync(file).toString('base64');
const ICON = dataUri(path.join(WEB, 'icon-192.png'), 'image/png');
// Public pixels contain an illustrative address and an unusable token. Actual pairing uses /qr off-screen.
const PUBLIC_QR = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAWgAAAFoAQMAAABAK/OWAAAABlBMVEX///8AAABVwtN+AAACU0lEQVR42uzau5GlMBCF4Z+6BqZCIBRCE6ERCiFgylBxtvSAYdiZMWZvSWt0WxR8XpfEaRBWVlZWVlaty6nUArjwOgbBLO2jItRnwfT79A6AllmbCwwH0jpvLrx0oAVgNN1Pj5LQwpSJpGVep9rLxae2mu6vmXaSxq/gApj+f/QsSTGvNKVextdhur8GKCuNMQ4SXtrgFflh1zT9W32mgnlLLUndWearO99mCNNN9EdN9RmkgCDFH9Oy6RbapQaWVKB9jIC0gqvYr7hguptOLxsJSio4spilQMlvmH6rzkNleber7Gx1viw728J0f8+bbqtdWTB4bXlrO+raCa846MuVZrqdBhhKKsDpmQoWr+2e2Uy31TWzkW4z5kuYt9pLv8ymO+pCzukfqHui4kt5pU2S6TdqFxiuTyz5O/JtvkxL6pEKTDfX3OZLgGmHvLOVS9OdtFMABEUzqCRqRQC/POZL0631GZ23KyBciTr3cgym36bhzMjSTv3ndWVkn67BdCftysifu1YnmJ8ysumm+kwFXCst7Ww5FdSM7GS6l3Z7JpK2qdwuvayp4JGoTbfVOOkAvPL0X1PBxqirlwHTb9NuH3XkUUWJ6AC/kne23LP1c0Y23VRflTTXfFlOHpUWm+6mr//FTBujDh6pYJ0eG5vplnoHOCeb+JGoz79l35zMMP1bfZ4l2vjUndq1Z6I23UV/nDC+ZWS8/koFpjvo3MtzpU0K5VyYX6d9DKa7aThP7d2/8o/6upemm+rr1J5uqQCn+OU0avoftZWVlZWV1a/rzwBmCyl2T0aBBgAAAABJRU5ErkJggg==';
const ps = (cmd) => cp.execFileSync('powershell.exe', ['-NoProfile', '-STA', '-Command', '[Console]::OutputEncoding=[Text.Encoding]::UTF8;' + cmd], { encoding: 'utf8' }).replace(/\r?\n$/, '');
const getClip = () => ps('Get-Clipboard -Raw');
const setClip = (t) => ps(t ? `Set-Clipboard -Value '${t.replace(/'/g, "''")}'` : 'Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Clipboard]::Clear()');

const marks = [];           // caption timeline, seconds from PC clip start
let pcT0 = 0;
const cap = (text) => { marks.push({ t: (Date.now() - pcT0) / 1000, text }); console.log(`[${((Date.now() - pcT0) / 1000).toFixed(1)}s] ${text}`); };

// fake finger: a ripple shown in the top layer, so it also appears above open sheets
async function ripple(page, x, y) {
  await page.evaluate(([x, y]) => {
    const d = document.createElement('div');
    d.setAttribute('popover', 'manual');
    d.style.cssText = `position:fixed;inset:auto;margin:0;left:${x - 28}px;top:${y - 28}px;width:56px;height:56px;border-radius:50%;background:rgba(255,212,71,.55);border:3px solid #FFD447;pointer-events:none;transition:transform .5s,opacity .5s;overflow:hidden;padding:0`;
    document.body.append(d); try { d.showPopover(); } catch {}
    requestAnimationFrame(() => { d.style.transform = 'scale(1.5)'; d.style.opacity = '0.1'; });
    setTimeout(() => d.remove(), 650);
  }, [x, y]);
}
async function tap(page, locator) {
  const el = typeof locator === 'string' ? page.locator(locator).first() : locator;
  await el.scrollIntoViewIfNeeded();
  const b = await el.boundingBox();
  await ripple(page, b.x + b.width / 2, b.y + b.height / 2);
  await sleep(350);
  await el.click();
}

async function ctxPair(browser) { // pairs a phone like the camera would, returns a storageState
  const qr = await fetch(`${L}/qr`, { headers: LH });
  if (!qr.ok) throw new Error('QR request failed: ' + qr.status);
  const png = PNG.sync.read(Buffer.from(await qr.arrayBuffer()));
  const r = jsQR(new Uint8ClampedArray(png.data), png.width, png.height);
  if (!r) throw new Error('no QR'); BASE = new URL(r.data).origin;
  const c = await browser.newContext({ ignoreHTTPSErrors: true });
  const resp = await c.request.get(r.data); if (!resp.ok()) throw new Error('pair failed ' + resp.status());
  const st = await c.storageState(); await c.close(); return st;
}
async function openPairing() { // same as tray "Pair phone": opens a fresh 10-minute window
  const r = await fetch(`${L}/pairing/open`, { method: 'POST', headers: { Origin: L, ...LH } });
  if (r.status !== 204) throw new Error('Opening pairing failed: ' + r.status);
}
let BASE = "";
async function lanBase() { return BASE; }
function mkImage() { return path.join(RAW, 'sunset.png'); }

(async () => {
  const rawRoot = path.resolve(process.env.RAW_DIR);
  if (path.dirname(path.resolve(RAW)) !== rawRoot || !/^copyapp-video-[a-f0-9]{32}$/i.test(path.basename(path.dirname(rawRoot)))) throw new Error('Unsafe raw recording path');
  fs.rmSync(RAW, { recursive: true, force: true }); fs.mkdirSync(RAW, { recursive: true });
  const browser = await chromium.launch();
  const phoneOpts = { viewport: { width: 390, height: 844 }, deviceScaleFactor: 1, isMobile: true, hasTouch: true, ignoreHTTPSErrors: true, colorScheme: 'light', locale: 'en-US' };

  // a pretty photo to send
  {
    const c = await browser.newContext({ viewport: { width: 640, height: 420 } }); const p = await c.newPage();
    await p.setContent('<body style="margin:0;width:640px;height:420px;background:linear-gradient(180deg,#2E44E8 0%,#8b5cf6 45%,#ff9a62 75%,#FFD447 100%);overflow:hidden">' +
      '<div style="position:absolute;left:250px;top:210px;width:140px;height:140px;border-radius:50%;background:#FFD447;box-shadow:0 0 80px #fff6"></div>' +
      '<div style="position:absolute;left:0;right:0;bottom:0;height:110px;background:#10152A"></div>' +
      '<div style="position:absolute;left:0;right:0;bottom:30px;text-align:center;color:#fff;font:700 34px Segoe UI">Holiday sunset</div></body>');
    await p.screenshot({ path: mkImage() }); await c.close();
  }

  // Masks anything personal on screen (user name, temp paths, LAN addresses) and enlarges the real PC window page.
  const maskScript = (publicQr) => {
    const bytes = Uint8Array.from(atob(publicQr.split(',')[1]), c => c.charCodeAt(0));
    const displayQr = URL.createObjectURL(new Blob([bytes], { type: 'image/png' }));
    window.__copyappPublicQrURL = displayQr;
    const fix = (t) => t
      .replace(/[A-Za-z]:\\[^\r\n<>"']*?copyapp-video-[a-f0-9]{32}\\received-\w+/gi, 'C:\\Users\\You\\Downloads\\CopyApp')
      .replace(/[A-Za-z]:\\[^\r\n<>"']*?copyapp-video-[a-f0-9]{32}\\appdata-\w+\\CopyApp/gi, 'C:\\Users\\You\\AppData\\Roaming\\CopyApp')
      .replace(/[A-Za-z]:\\Users\\[^\\\s]+/gi, 'C:\\Users\\You')
      .replace(/\blocalhost\b/gi, '192.168.1.20')
      .replace(/\b(?:\d{1,3}\.){3}\d{1,3}\b/g, '192.168.1.20');
    const run = () => {
      const qr = document.querySelector('#qrimg');
      if (qr && qr.src !== displayQr) qr.src = displayQr;
      const w = document.createTreeWalker(document.documentElement, NodeFilter.SHOW_TEXT);
      for (let n = w.nextNode(); n; n = w.nextNode()) { if (['SCRIPT', 'STYLE'].includes(n.parentElement?.tagName)) continue; const v = fix(n.nodeValue); if (v !== n.nodeValue) n.nodeValue = v; }
      for (const i of document.querySelectorAll('input,textarea')) if (i.type !== 'file' && typeof i.value === 'string') { const v = fix(i.value); if (v !== i.value) { if (i.readOnly) i.value = v; else i.style.filter = 'blur(7px)'; } }
    };
    const frame = () => { run(); requestAnimationFrame(frame); }; requestAnimationFrame(frame);
    new MutationObserver(run).observe(document, { childList: true, subtree: true, characterData: true });
    if (location.hostname === '127.0.0.1') document.addEventListener('DOMContentLoaded', () => { document.documentElement.style.zoom = '1.3'; });
  };

  const pcCtx = await browser.newContext({ viewport: { width: 1280, height: 720 }, colorScheme: 'light', recordVideo: { dir: RAW, size: { width: 1280, height: 720 } } });
  await pcCtx.addInitScript(maskScript, PUBLIC_QR);
  let st = null;
  if (scenario === 'use') { await openPairing(); st = await ctxPair(browser); }
  const phoneCtx = await browser.newContext({ ...phoneOpts, ...(st ? { storageState: st } : {}), recordVideo: { dir: RAW, size: { width: 390, height: 844 } } });
  await phoneCtx.addInitScript(maskScript, PUBLIC_QR);
  await phoneCtx.addInitScript(() => { try { localStorage.setItem('copyapp.lang', 'en'); localStorage.setItem('tipDismissed', '1'); localStorage.setItem('setupDismissed', '1'); } catch {} });

  const pc = await pcCtx.newPage(); pcT0 = Date.now();
  const ph = await phoneCtx.newPage(); const phT0 = Date.now();
  const phoneOffset = (phT0 - pcT0) / 1000;
  const goPC = async (html) => { await pc.goto('about:blank'); await pc.setContent(html, { waitUntil: 'load' }); await pc.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)))); };
  const goWin = async () => {
    // Bootstrap is a 200 response. The page removes ?k=, keeps its browser token
    // in sessionStorage, and explicitly authenticates state/QR requests with it.
    const qrRequest = pc.waitForResponse(r => r.url() === L + '/qr').catch(() => null);
    const response = await pc.goto(L + '/pairing?k=' + KEY);
    if (!response || response.status() !== 200) throw new Error('Control bootstrap failed');
    await pc.waitForFunction(() => location.pathname === '/pairing' && !location.search && !!sessionStorage.getItem('copyapp.localSession'));
    const state = await pc.evaluate(async () => {
      const r = await fetch('/local/state', { headers: { 'X-CopyApp-Session': sessionStorage.getItem('copyapp.localSession') } });
      if (!r.ok) throw new Error('Control browser session rejected: ' + r.status);
      return r.json();
    });
    if (!process.env.COPYAPP_RECORD_VERSION || state.version !== process.env.COPYAPP_RECORD_VERSION) {
      throw new Error('Recording version must match the selected executable');
    }
    await pc.waitForFunction(() => !document.body.classList.contains('offline'));
    if (state.pairing.open) {
      const qrResponse = await qrRequest;
      if (!qrResponse?.ok()) throw new Error('Control session could not load its QR');
      await pc.waitForFunction(() => { const i = document.querySelector('#qrimg'); return i?.src === window.__copyappPublicQrURL && i.complete && i.naturalWidth > 0; });
    }
    await sleep(500);
  };
  const scrollPC = (sel, block = 'start') => pc.evaluate(([sel, block]) => { const e = document.querySelector(sel); if (e) e.scrollIntoView({ behavior: 'smooth', block }); }, [sel, block]);

  const clipBefore = (() => { try { return getClip(); } catch { return ''; } })();
  const recvDir = process.env.COPYAPP_RECEIVE_DIR;
  if (!recvDir) throw new Error('Recording requires an isolated COPYAPP_RECEIVE_DIR');
  const hadRecv = fs.existsSync(recvDir); const recvBefore = hadRecv ? fs.readdirSync(recvDir) : [];

  try {
    if (scenario === 'install') await installScenario(pc, ph, goPC, goWin);
    else await useScenario(pc, ph, goPC, goWin, scrollPC);
    cap('END');
  } finally {
    try { fs.copyFileSync(path.join(process.env.COPYAPP_APPDATA, 'CopyApp', 'copyapp.log'), path.join(RAW, 'copyapp.log')); } catch {}
    const pcVideo = pc.video(), phVideo = ph.video();
    await pcCtx.close(); await phoneCtx.close(); await browser.close();
    fs.renameSync(await pcVideo.path(), path.join(RAW, 'pc.webm'));
    fs.renameSync(await phVideo.path(), path.join(RAW, 'phone.webm'));
    fs.writeFileSync(path.join(RAW, 'meta.json'), JSON.stringify({ phoneOffset, marks }, null, 1));
    if (fs.existsSync(recvDir)) {
      for (const f of fs.readdirSync(recvDir)) if (!recvBefore.includes(f)) fs.rmSync(path.join(recvDir, f), { force: true });
      if (!hadRecv && !fs.readdirSync(recvDir).length) fs.rmdirSync(recvDir);
    }
    try { setClip(clipBefore); } catch {}
  }

  // ---------------------------------------------------------------- install
  async function installScenario(pc, ph, goPC, goWin) {
    const startupPng = ILLUSTRATE_NATIVE ? null : dataUri(path.join(process.env.CAP_DIR, 'startup.png'), 'image/png');
    await ph.setContent('<meta name="viewport" content="width=device-width,initial-scale=1"><body style="margin:0;height:100vh;background:linear-gradient(160deg,#2E44E8,#10152A);color:#fff;font:700 26px Segoe UI;display:flex;align-items:center;justify-content:center;text-align:center;padding:30px;box-sizing:border-box">Connect your phone<br>to Home Wi-Fi<br>(the same Wi-Fi as the PC)</body>');
    await goPC(mocks.releases(ICON)); cap('1. Download copyapp.exe from the CopyApp Releases page');
    await sleep(1800); await pc.evaluate(() => click()); await sleep(2200);

    await goPC(mocks.downloads(ICON)); cap('2. Open Downloads and double-click copyapp.exe');
    await sleep(3600);

    await goPC(mocks.prompts()); cap('3. Windows may warn about a new app: click More info');
    await sleep(1600); await pc.evaluate(() => step('a')); await sleep(1800);
    await pc.evaluate(() => step('b')); cap('Then click Run anyway (normal for a new app)');
    await sleep(3600);
    await pc.evaluate(() => step('c')); cap('If the firewall asks, click Allow access');
    await sleep(3600);

    await goPC(mocks.startupQuestion(startupPng, ICON)); await pc.screenshot({ path: path.join(RAW, 'dbg-startup.png') });
    cap('CopyApp installs itself and asks: Start with Windows?');
    await sleep(4200);
    await goPC(mocks.desktop(ICON)); cap('A CopyApp shortcut now sits on your Desktop');
    await sleep(3600);

    await openPairing();
    await goWin(); cap('The CopyApp window opens. Note your Wi-Fi name here');
    await sleep(4200);
    await pc.evaluate(() => document.querySelector('#pair')?.scrollIntoView({ block: 'center' })); cap('Scan this QR code with your phone camera');
    await sleep(3600);

    const qrPng = await (await fetch(L + '/qr', { headers: LH })).arrayBuffer();
    const png = PNG.sync.read(Buffer.from(qrPng));
    const decoded = jsQR(new Uint8ClampedArray(png.data), png.width, png.height);
    if (!decoded) throw new Error('Displayed pairing QR does not decode');
    const pairUrl = decoded.data;
    await ph.setContent(mocks.phoneCamera(PUBLIC_QR));
    cap('4. On the phone, open the camera and point it at the QR');
    await sleep(2200); await ph.evaluate(() => found()); await sleep(1800);
    await ripple(ph, 195, 725); await sleep(800);

    await ph.setContent(mocks.chromeWarning());
    cap('Chrome says "not private": tap Advanced');
    await sleep(1200); await ph.evaluate(() => step('a')); await sleep(2200);
    await ph.evaluate(() => step('b')); cap('Then Proceed. It is your own PC on your own Wi-Fi');
    await sleep(3600);

    await ph.goto(pairUrl); await ph.waitForSelector('body[data-view="main"]', { timeout: 15000 });
    cap('5. The phone says Connected'); await sleep(3600);
    await goWin(); cap('The PC window says "Paired from ..." with the phone address');
    await sleep(4200);

    cap('Optional: Install as an app adds Android sharing');
    await tap(ph, '[data-open="sheet-settings"]'); await sleep(1000);
    await tap(ph, '#app-row'); await sleep(1200);
    const h = await ph.evaluate(() => { const d = document.getElementById('sheet-cert'); return d.scrollHeight - d.clientHeight; });
    for (let i = 1; i <= 4; i++) { await ph.evaluate(([i, h]) => document.getElementById('sheet-cert').scrollTo({ top: h * i / 4, behavior: 'smooth' }), [i, h]); await sleep(900); }
    await ph.evaluate(() => document.getElementById('sheet-cert').close()); await sleep(300);
    await ph.evaluate(() => { const d = document.getElementById('sheet-settings'); if (d && d.open) d.close(); }); await sleep(1500);
  }

  // -------------------------------------------------------------------- use
  async function useScenario(pc, ph, goPC, goWin, scrollPC) {
    await ph.goto(await lanBase() + '/');
    await ph.waitForSelector('body[data-view="main"]', { timeout: 15000 });
    await goPC(mocks.notepad('Untitled - Notepad', '', 'Your PC is waiting...'));
    await sleep(1500);
    const dest = (sel, v) => tap(ph, 'label:has(input[name="' + sel + '"][value="' + v + '"])');

    // --- phone -> PC: text, destination Clipboard
    const msg = 'Meeting moved to 4pm, room 12';
    cap('Phone to PC: tap Type text and write your message');
    await tap(ph, '[data-open="sheet-paste"]'); await sleep(800);
    await ph.locator('#paste-text').focus(); await ph.keyboard.type(msg, { delay: 70 }); await sleep(700);
    cap('Pick the destination Clipboard, then Send to PC');
    await dest('dest-paste', 'clipboard'); await sleep(1200);
    await tap(ph, '#paste-form button[type=submit]'); await sleep(2200);
    cap('The phone says where it went. On the PC just press Ctrl+V');
    await goPC(mocks.notepad('Untitled - Notepad (after Ctrl+V)', getClip(), 'Ctrl+V pastes the text'));
    await sleep(4000);

    // --- phone -> PC: photo saved as a file
    await goPC(mocks.notepad('Untitled - Notepad', '', 'Next: a photo'));
    await dest('dest-dock', 'file'); await sleep(800);
    cap('A photo: choose File as the destination, then pick the photo');
    await tap(ph, 'label:has(#pick-photo)');
    await ph.setInputFiles('#pick-photo', mkImage()); await sleep(2400);
    const names = fs.readdirSync(recvDir).sort((a, b) => fs.statSync(path.join(recvDir, b)).mtimeMs - fs.statSync(path.join(recvDir, a)).mtimeMs);
    await goPC(mocks.folder('This PC > Downloads > CopyApp', names, 'Saved on your PC: Downloads\\CopyApp'));
    cap('Saved on your PC, in Downloads\\CopyApp'); await sleep(3800);

    // --- phone -> PC: big file with progress
    const big = path.join(RAW, 'Holiday-video.mp4');
    fs.writeFileSync(big, Buffer.alloc(110 * 1024 * 1024, 7));
    const cdp = await ph.context().newCDPSession(ph);
    await cdp.send('Network.enable');
    await cdp.send('Network.emulateNetworkConditions', { offline: false, latency: 30, downloadThroughput: -1, uploadThroughput: 16 * 1024 * 1024 });
    await goWin(); await scrollPC('#transfers');
    cap('Big files stream with progress, speed and time left');
    await tap(ph, 'label:has(#pick-file)');
    await ph.setInputFiles('#pick-file', big);
    await sleep(9000);
    await cdp.send('Network.emulateNetworkConditions', { offline: false, latency: 0, downloadThroughput: -1, uploadThroughput: -1 });
    await sleep(1500);

    // --- PC -> phone: Ctrl+Alt+C
    const pcText = 'Shopping list: coffee, apples, bread';
    setClip(pcText);
    const pcState = await (await fetch(L + '/local/state', { headers: LH })).json();
    if (!/^Ctrl\+Alt\+(Shift\+)?C$/.test(pcState.hotkey)) throw new Error('No available global hotkey');
    if (ILLUSTRATE_NATIVE) {
      await goWin();
      cap('PC to phone: copy something, then click Send PC clipboard to phone now'); await sleep(2800);
      const sent = pc.waitForResponse(r => new URL(r.url()).pathname === '/local/send-clipboard');
      await tap(pc, '#sendNow');
      const result = await (await sent).json();
      if (result.result !== 'sent') throw new Error('Actual PC-window clipboard send failed: ' + result.result);
      await goPC(mocks.osdScene(null, pcText));
    } else {
      await goPC(mocks.notepad('Notes - Notepad', pcText, 'Copy it (Ctrl+C), then press ' + pcState.hotkey.split('+').map(k => '<span class="k">' + k + '</span>').join('+')));
      cap('PC to phone: copy something, then press ' + pcState.hotkey); await sleep(3200);
      await pc.evaluate(() => document.querySelectorAll('.k').forEach((k) => k.classList.add('on')));
      const osdPng = path.join(RAW, 'osd.png');
      const cpo = cp.spawn('pwsh', ['-NoProfile', '-File', path.join(__dirname, 'caposd.ps1'), '-Out', osdPng, '-ProcessId', process.env.COPYAPP_PID, '-Hotkey', pcState.hotkey], { stdio: 'inherit', windowsHide: true });
      const cpoDone = new Promise((resolve, reject) => { cpo.on('error', reject); cpo.on('exit', code => code === 0 ? resolve() : reject(new Error('Native green box capture failed: ' + code))); });
      await cpoDone;
      await goPC(mocks.osdScene(dataUri(osdPng, 'image/png'), pcText));
      }
    cap('A green "Sent to phone" box confirms it'); await sleep(3400);
    await ph.waitForSelector('#feed .msg-in', { timeout: 10000 });
    cap('On the phone it appears: tap Copy'); await sleep(2400);
    await tap(ph, '#feed .msg-in:last-child .btn-primary'); await sleep(1800);

    // --- PC -> phone: Send a file...
    fs.writeFileSync(path.join(RAW, 'Holiday-photos-list.txt'), 'Photos to print:\n1. Beach\n2. Hike\n');
    await goPC(mocks.tray(ICON));
    cap('Send a file: tray icon menu, "Send a file to phone..."'); await sleep(3600);
    await goPC(mocks.sendto('Holiday-photos-list.txt'));
    cap('Or right-click any file, then Send to > Phone (CopyApp)'); await sleep(600);
    await pc.evaluate(() => step('menu')); await sleep(1400);
    await pc.evaluate(() => step('sub')); await sleep(1600);
    const before = await ph.locator('#feed .msg-in').count();
    cp.execFileSync(EXE, [path.join(RAW, 'Holiday-photos-list.txt')], { timeout: 20000, env: { ...process.env, APPDATA: process.env.COPYAPP_APPDATA } });
    await ph.waitForFunction((n) => document.querySelectorAll('#feed .msg-in').length > n, before, { timeout: 10000 });
    cap('The file shows up on the phone: tap Download'); await sleep(2200);
    await tap(ph, '#feed .msg-in:last-child a.btn-primary'); await sleep(1200);

    // --- PC settings
    await goWin(); await scrollPC('#settings');
    cap('In the PC window, choose what happens to items from the phone'); await sleep(2500);
    await pc.locator('input[type=radio][value=folder]').first().click({ force: true });
    await sleep(2200);
    await tap(ph, '[data-open="sheet-settings"]'); await sleep(600);
    cap('The phone shows the PC setting too (Folder)'); await sleep(3400);
    await ph.evaluate(() => document.getElementById('sheet-settings').close()); await sleep(400);

    // --- Android Share menu + confirm
    await ph.evaluate((html) => { const f = document.createElement('iframe'); f.id = 'shareov'; f.srcdoc = html; f.style.cssText = 'position:fixed;inset:0;width:100%;height:100%;border:0;z-index:2147483000'; document.body.append(f); }, mocks.androidShare(ICON));
    cap('On Android: Share > CopyApp from any app'); await sleep(3400);
    // Feed the same one-use cache payload as the Android share-target service worker.
    // The actual app reads it, creates pendingShare and sends only after confirmation.
    await ph.evaluate(async () => {
      document.getElementById('shareov').remove();
      const cache = await caches.open('copyapp-share-v1');
      await cache.put('/__share/meta', new Response(JSON.stringify({ at: Date.now(), text: '', url: 'https://example.com/trip-plan', title: '', files: [] }), { headers: { 'Content-Type': 'application/json' } }));
    });
    await ph.goto(BASE + '/?share=1');
    await ph.waitForSelector('#sheet-share[open]');
    cap('Confirm on the phone, then it goes to your PC'); await sleep(2400);
    const shared = ph.waitForResponse(r => new URL(r.url()).pathname === '/send' && r.request().method() === 'POST');
    await tap(ph, '#share-send');
    const sent = await shared;
    if (!sent.ok()) throw new Error('Confirmed Android share failed: ' + sent.status());
    await sleep(2400);
  }
})().catch((e) => { console.error(e); process.exit(1); });
