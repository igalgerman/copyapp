// Records the raw PC + phone clips for one scenario ("install" or "use") against the REAL running copyapp.exe.
// Called by make.ps1, which starts the exe with a temp APPDATA and sets the env vars below.
//   env: COPYAPP_LPORT (loopback port), COPYAPP_EXE, RAW_DIR, WEB_DIR (internal/server/web), NODE_PATH (playwright, jsqr, pngjs)
const { chromium } = require('playwright');
const fs = require('fs'), path = require('path'), cp = require('child_process');
const { PNG } = require('pngjs'); const jsQR = require('jsqr');
const mocks = require('./mocks');

const scenario = process.argv[2];
const LPORT = process.env.COPYAPP_LPORT, EXE = process.env.COPYAPP_EXE, RAW = path.join(process.env.RAW_DIR, scenario), WEB = process.env.WEB_DIR;
const L = `http://127.0.0.1:${LPORT}`;
// per-launch local key: the PC-only routes need it (header on fetches, ?k= to open a page in the browser)
const KEY = fs.readFileSync(path.join(process.env.COPYAPP_APPDATA, 'CopyApp', 'local.key'), 'utf8').trim();
const LH = { 'X-CopyApp': KEY };
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const dataUri = (file, mime) => `data:${mime};base64,` + fs.readFileSync(file).toString('base64');
const ICON = dataUri(path.join(WEB, 'icon-192.png'), 'image/png');
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
  const png = PNG.sync.read(Buffer.from(await (await fetch(`${L}/qr`, { headers: LH })).arrayBuffer()));
  const r = jsQR(new Uint8ClampedArray(png.data), png.width, png.height);
  if (!r) throw new Error('no QR'); BASE = new URL(r.data).origin;
  const c = await browser.newContext({ ignoreHTTPSErrors: true });
  const resp = await c.request.get(r.data); if (!resp.ok()) throw new Error('pair failed ' + resp.status());
  const st = await c.storageState(); await c.close(); return st;
}
async function openPairing() { // same as tray "Pair phone": opens a fresh 10-minute window
  await fetch(`${L}/pairing/open`, { method: 'POST', headers: { Origin: L, ...LH } });
}
let BASE = "";
async function lanBase() { return BASE; }
function mkImage() { return path.join(RAW, 'sunset.png'); }

(async () => {
  fs.rmSync(RAW, { recursive: true, force: true }); fs.mkdirSync(RAW, { recursive: true });
  const browser = await chromium.launch();
  const phoneOpts = { viewport: { width: 390, height: 844 }, deviceScaleFactor: 1, isMobile: true, hasTouch: true, ignoreHTTPSErrors: true, colorScheme: 'light', locale: 'en-US' };

  // a pretty photo to send
  {
    const c = await browser.newContext({ viewport: { width: 640, height: 420 } }); const p = await c.newPage();
    await p.setContent(`<body style="margin:0;width:640px;height:420px;background:linear-gradient(180deg,#2E44E8 0%,#8b5cf6 45%,#ff9a62 75%,#FFD447 100%);overflow:hidden">
      <div style="position:absolute;left:250px;top:210px;width:140px;height:140px;border-radius:50%;background:#FFD447;box-shadow:0 0 80px #fff6"></div>
      <div style="position:absolute;left:0;right:0;bottom:0;height:110px;background:#10152A"></div>
      <div style="position:absolute;left:0;right:0;bottom:30px;text-align:center;color:#fff;font:700 34px 'Segoe UI'">Holiday sunset</div></body>`);
    await p.screenshot({ path: mkImage() }); await c.close();
  }

  const pcCtx = await browser.newContext({ viewport: { width: 1280, height: 720 }, colorScheme: 'light', recordVideo: { dir: RAW, size: { width: 1280, height: 720 } } });
  let st = null;
  if (scenario === 'use') st = await ctxPair(browser);
  const phoneCtx = await browser.newContext({ ...phoneOpts, ...(st ? { storageState: st } : {}), recordVideo: { dir: RAW, size: { width: 390, height: 844 } } });
  if (scenario === 'use') await phoneCtx.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: BASE });
  if (scenario === 'use') await phoneCtx.addInitScript(() => { try { localStorage.setItem('setupDismissed', '1'); } catch {} });

  pcT0 = Date.now(); const pc = await pcCtx.newPage();
  const phT0 = Date.now(); const ph = await phoneCtx.newPage();
  const phoneOffset = (phT0 - pcT0) / 1000;
  const goPC = async (html) => { await pc.setContent(html, { waitUntil: 'load' }); };

  const clipBefore = (() => { try { return getClip(); } catch { return ''; } })();
  const recvDir = path.join(ps("(New-Object -ComObject Shell.Application).NameSpace('shell:Downloads').Self.Path"), 'CopyApp');
  const hadRecv = fs.existsSync(recvDir); const recvBefore = hadRecv ? fs.readdirSync(recvDir) : [];

  try {
    if (scenario === 'install') await installScenario(pc, ph, goPC, phoneCtx);
    else await useScenario(pc, ph, goPC);
    cap('END');
  } finally {
    const pcVideo = pc.video(), phVideo = ph.video();
    await pcCtx.close(); await phoneCtx.close(); await browser.close();
    fs.renameSync(await pcVideo.path(), path.join(RAW, 'pc.webm'));
    fs.renameSync(await phVideo.path(), path.join(RAW, 'phone.webm'));
    fs.writeFileSync(path.join(RAW, 'meta.json'), JSON.stringify({ phoneOffset, marks }, null, 1));
    // undo what the test sent (like docs/e2e/run.ps1)
    if (fs.existsSync(recvDir)) {
      for (const f of fs.readdirSync(recvDir)) if (!recvBefore.includes(f)) fs.rmSync(path.join(recvDir, f), { force: true });
      if (!hadRecv && !fs.readdirSync(recvDir).length) fs.rmdirSync(recvDir);
    }
    try { setClip(clipBefore); } catch {}
  }

  // ---------------------------------------------------------------- install
  async function installScenario(pc, ph, goPC, phoneCtx) {
    await ph.setContent(`<body style="margin:0;height:100vh;background:linear-gradient(160deg,#2E44E8,#10152A);color:#fff;font:700 24px 'Segoe UI';display:flex;align-items:center;justify-content:center;text-align:center;padding:30px">Your phone<br>waits for step 5</body>`);
    await goPC(mocks.releases(ICON)); cap('1. Download copyapp.exe from the Releases page');
    await sleep(1800); await pc.evaluate(() => click()); await sleep(3800);

    await goPC(mocks.downloads(ICON)); cap('2. Open the Downloads folder and double-click copyapp.exe');
    await sleep(4200);

    await goPC(mocks.prompts()); cap('3. Windows may say "protected your PC": click More info');
    await sleep(1800); await pc.evaluate(() => step('a')); await sleep(2400);
    await pc.evaluate(() => step('b')); cap('3. Then click Run anyway (normal for a new app)');
    await sleep(3500);
    await pc.evaluate(() => step('c')); cap('If asked by the firewall: Allow on private networks');
    await sleep(4200);

    await openPairing();
    await pc.goto(`${L}/pairing?k=${KEY}`); cap('4. CopyApp shows a pairing page with a QR code');
    await sleep(3600);

    const qrPng = await (await fetch(`${L}/qr`, { headers: LH })).arrayBuffer();
    const png = PNG.sync.read(Buffer.from(qrPng));
    const pairUrl = jsQR(new Uint8ClampedArray(png.data), png.width, png.height).data;
    await ph.setContent(mocks.phoneCamera('data:image/png;base64,' + Buffer.from(qrPng).toString('base64')));
    cap('5. On your phone, open the camera and point it at the QR code');
    await sleep(2800); await ph.evaluate(() => found()); await sleep(2600);
    await ripple(ph, 195, 725); await sleep(500);
    await ph.goto(pairUrl); await ph.waitForSelector('body[data-view="main"]', { timeout: 15000 }).catch(() => {});
    await pc.reload(); cap('Done: the PC says "Phone paired"');
    await sleep(3000);

    await ph.waitForSelector('#setup-card:not([hidden])', { timeout: 8000 }).catch(() => {});
    cap('6. One time only: tap "Show me how" to install the certificate');
    await sleep(2800); await tap(ph, '#setup-card .btn-primary'); await sleep(1500);
    await ph.waitForFunction(() => { const n = document.querySelector('#sheet-cert [data-fp-short]'); return n && n.textContent.trim().length > 3 && n.textContent.trim() !== '…'; }, null, { timeout: 8000 }).catch(() => console.log('WARN: short code not shown'));
    await pc.goto(`${L}/pairing?k=${KEY}`); // PC window shows the same code to compare
    cap('Check the code matches your PC'); await sleep(5500);
    cap('Follow the 4 steps: Settings > Security > Install a certificate');
    // gently scroll through the real certificate steps
    const h = await ph.evaluate(() => { const d = document.getElementById('sheet-cert'); return d.scrollHeight - d.clientHeight; });
    for (let i = 1; i <= 8; i++) { await ph.evaluate(([i, h]) => document.getElementById('sheet-cert').scrollTo({ top: h * i / 8, behavior: 'smooth' }), [i, h]); await sleep(950); }
    await sleep(800);
    await ph.evaluate(() => document.getElementById('sheet-cert').close()); await sleep(600);

    cap('7. Chrome menu (3 dots) > Add to Home screen');
    await ph.evaluate((html) => document.body.insertAdjacentHTML('beforeend', html), mocks.chromeMenu(ICON));
    await sleep(2800);
    await ph.evaluate(() => { document.getElementById('menu').style.display = 'none'; document.getElementById('dlg').style.display = 'block'; });
    await sleep(2200);
    await ph.evaluate(() => { document.getElementById('dlg').style.display = 'none'; document.getElementById('scrim').style.display = 'none'; document.getElementById('home').style.display = 'block'; });
    await sleep(3000);
    await ph.evaluate(() => document.getElementById('ov').remove());

    cap('Open CopyApp from the home screen: it says Connected');
    await sleep(3000);
  }

  // -------------------------------------------------------------------- use
  async function useScenario(pc, ph, goPC) {
    await ph.goto(await lanBase() + '/');
    await ph.waitForSelector('body[data-view="main"]', { timeout: 15000 });
    await goPC(mocks.notepad('Untitled - Notepad', '', 'Your PC is waiting...'));
    await sleep(2500);

    // --- phone -> PC: text
    const msg = 'Meeting moved to 4pm, room 12';
    cap('Phone to PC: tap Type, write your text');
    await tap(ph, '[data-open="sheet-paste"]'); await sleep(900);
    await ph.locator('#paste-text').focus(); await ph.keyboard.type(msg, { delay: 70 }); await sleep(900);
    cap('Tap "Send to PC"');
    await tap(ph, '#paste-form button[type=submit]'); await sleep(1500);
    await goPC(mocks.notepad('Untitled - Notepad (after Ctrl+V)', getClip(), 'On the PC just press Ctrl+V to paste'));
    cap('On the PC, press Ctrl+V: the text is there'); await sleep(4200);

    // --- phone -> PC: photo (goes to the PC clipboard as an image)
    cap('Send a photo: tap Photo and pick a picture');
    await tap(ph, 'label:has(#pick-photo)').catch(() => {});
    await ph.setInputFiles('#pick-photo', mkImage()); await sleep(2200);
    const imgOut = path.join(RAW, 'clip.png');
    ps(`Add-Type -AssemblyName System.Windows.Forms,System.Drawing; $i=[System.Windows.Forms.Clipboard]::GetImage(); if($i){$i.Save('${imgOut}')}`);
    await goPC(mocks.paint(dataUri(imgOut, 'image/png'), 'Press Ctrl+V in any app: the photo pastes'));
    cap('Photo arrives on the PC clipboard: Ctrl+V pastes it'); await sleep(4000);

    // --- phone -> PC: file
    fs.writeFileSync(path.join(RAW, 'Trip-plan.txt'), 'Day 1: arrive\nDay 2: beach\nDay 3: hike\n');
    await goPC(mocks.notepad('Untitled - Notepad', '', 'Next: send a file'));
    cap('Send a file: tap File and pick one');
    await tap(ph, 'label:has(#pick-file)').catch(() => {});
    await ph.setInputFiles('#pick-file', path.join(RAW, 'Trip-plan.txt')); await sleep(2000);
    const names = fs.readdirSync(recvDir).sort((a, b) => fs.statSync(path.join(recvDir, b)).mtimeMs - fs.statSync(path.join(recvDir, a)).mtimeMs);
    await goPC(mocks.folder('This PC > Downloads > CopyApp', names, 'Files land in Downloads\\CopyApp'));
    cap('Files are saved on the PC in Downloads\\CopyApp'); await sleep(4000);

    // --- PC -> phone: Ctrl+Alt+C
    const pcText = 'Wi-Fi password: blue-kite-42';
    setClip(pcText);
    await goPC(mocks.notepad('Notes - Notepad', pcText, 'Copy it (Ctrl+C), then press <span class="k">Ctrl</span>+<span class="k">Alt</span>+<span class="k">C</span>'));
    cap('PC to phone: copy something, then press Ctrl+Alt+C'); await sleep(3000);
    await pc.evaluate(() => document.querySelectorAll('.k').forEach((k) => k.classList.add('on')));
    ps(`Add-Type -Name K -Namespace W -MemberDefinition '[DllImport("user32.dll")] public static extern void keybd_event(byte b, byte s, uint f, UIntPtr e);'; ` +
       `[W.K]::keybd_event(0x11,0,0,[UIntPtr]::Zero);[W.K]::keybd_event(0x12,0,0,[UIntPtr]::Zero);[W.K]::keybd_event(0x43,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 120;` +
       `[W.K]::keybd_event(0x43,0,2,[UIntPtr]::Zero);[W.K]::keybd_event(0x12,0,2,[UIntPtr]::Zero);[W.K]::keybd_event(0x11,0,2,[UIntPtr]::Zero)`);
    await ph.waitForSelector('#feed .msg-in', { timeout: 8000 }).catch(() => console.log('WARN: hotkey item did not arrive'));
    cap('It appears in the phone inbox: tap Copy'); await sleep(2600);
    await tap(ph, '#feed .msg-in:last-child .btn-primary').catch(() => {}); await sleep(2800);

    // --- PC -> phone: Send to
    fs.writeFileSync(path.join(RAW, 'Holiday-photos-list.txt'), 'Photos to print:\n1. Beach\n2. Hike\n');
    await goPC(mocks.sendto('Holiday-photos-list.txt'));
    cap('Or right-click any file on the PC...'); await sleep(500);
    await pc.evaluate(() => step('menu')); await sleep(2600);
    await pc.evaluate(() => step('sub')); cap('...choose Send to > Phone (CopyApp)'); await sleep(2600);
    const before = await ph.locator('#feed .msg-in').count();
    cp.execFileSync(EXE, [path.join(RAW, 'Holiday-photos-list.txt')], { timeout: 20000, env: { ...process.env, APPDATA: process.env.COPYAPP_APPDATA } });
    await ph.waitForFunction((n) => document.querySelectorAll('#feed .msg-in').length > n, before, { timeout: 8000 }).catch(() => console.log('WARN: file did not arrive'));
    cap('The file shows up on the phone: tap Download'); await sleep(2800);
    await tap(ph, '#feed .msg-in:last-child a.btn-primary').catch(() => {}); await sleep(3200);
  }
})().catch((e) => { console.error(e); process.exit(1); });
